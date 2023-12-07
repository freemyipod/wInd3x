package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/cache"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/mse"
	"github.com/freemyipod/wInd3x/pkg/usbms"
)

var restoreFull bool
var restoreVersion string

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore iPod to stock firmware",
	Long:  "Restores an iPod to stock/factory firmware from DFU mode, downloading everything necessary along the way. You _will_ lose all data stored on the device.",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {

		app, err := app.NewAny()
		if err != nil {
			return err
		}
		defer app.Close()

		hasBootloader := true
		shouldParseMSE := true
		switch app.Desc.Kind {
		case devices.Nano3:
			hasBootloader = false
			shouldParseMSE = false
		}

		switch restoreVersion {
		case "list":
			versions := cache.GetFirmwareVersions(app.Desc.Kind)
			glog.Infof("Available versions for %s:", app.Desc.Kind)
			for _, version := range versions {
				glog.Infof("- %s", version)
			}
			return nil
		case "", "current":
		default:
			cache.FirmwareVersionOverrides = map[devices.Kind]string{
				app.Desc.Kind: restoreVersion,
			}
		}

		firmware, err := cache.Get(app, cache.PayloadKindFirmwareUpstream)
		if err != nil {
			return fmt.Errorf("could not get firmware: %w", err)
		}

		if shouldParseMSE {
			m, err := mse.Parse(bytes.NewReader(firmware))
			if err != nil {
				return fmt.Errorf("could not parse firmware: %w", err)
			}
			firmware, err = m.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize modified firmware: %w", err)
			}
		}

		var bootloader []byte
		if hasBootloader {
			bootloader, err = cache.Get(app, cache.PayloadKindBootloaderUpstream)
			if err != nil {
				return fmt.Errorf("could not get bootloader: %s", err)
			}
		}

		for {
			glog.Infof("Found %s in %s", app.Desc.Kind, app.InterfaceKind)
			switch app.InterfaceKind {
			case devices.DFU:
				wtf, err := cache.Get(app, cache.PayloadKindWTFUpstream)
				if err != nil {
					return fmt.Errorf("could not get wtf payload: %s", err)
				}
				glog.Infof("Sending WTF...")
				if err := dfu.SendImage(app.Usb, wtf, app.Desc.Kind.DFUVersion()); err != nil {
					return fmt.Errorf("Failed to send image: %w", err)
				}
				glog.Infof("Waiting 10s for device to switch to WTF mode...")
				ctx, _ := context.WithTimeout(cmd.Context(), 10*time.Second)
				if err := app.WaitSwitch(ctx, devices.WTF); err != nil {
					return fmt.Errorf("device did not switch to WTF mode: %w", err)
				}
				time.Sleep(time.Second)
			case devices.WTF:
				recovery, err := cache.Get(app, cache.PayloadKindRecoveryUpstream)
				if err != nil {
					return fmt.Errorf("could not get recovery payload: %s", err)
				}
				glog.Infof("Sending recovery firmware...")
				for i := 0; i < 10; i++ {
					err = dfu.SendImage(app.Usb, recovery, app.Desc.Kind.DFUVersion())
					if err == nil {
						break
					} else {
						glog.Errorf("%v", err)
						time.Sleep(time.Second)
					}
				}
				if err != nil {
					return err
				}
				glog.Infof("Waiting 30s for device to switch to Recovery mode...")
				ctx, _ := context.WithTimeout(cmd.Context(), 30*time.Second)
				if err := app.WaitSwitch(ctx, devices.Disk); err != nil {
					return fmt.Errorf("device did not switch to Recovery mode: %w", err)
				}
				time.Sleep(time.Second)
			case devices.Disk:
				h := usbms.Host{
					InEndpoint:  app.MSEndpoints.In,
					OutEndpoint: app.MSEndpoints.Out,
				}
				di, err := h.IPodDeviceInformation()
				if err != nil {
					glog.Errorf("Could not get device information: %v", err)
				} else {
					fmt.Printf("SerialNumber: %s\n", di.SerialNumber)
					fmt.Printf("     BuildID: %s\n", di.BuildID)
				}

				if restoreFull {
					partsize := len(firmware)
					glog.Infof("Reformatting to %d MiB system partition...", partsize>>20)
					if err := h.IPodRepartition(partsize); err != nil {
						return fmt.Errorf("repartitioning failed: %w", err)
					}
					if hasBootloader {
						glog.Infof("Writing bootloader...")
						if err := h.IPodUpdateSendFull(usbms.IPodUpdateBootloader, bootloader); err != nil {
							return fmt.Errorf("writing bootloader failed: %w", err)
						}
					}
				}
				glog.Infof("Writing firmware...")
				if err := h.IPodUpdateSendFull(usbms.IPodUpdateFirmware, firmware); err != nil {
					return fmt.Errorf("writing firmware failed: %w", err)
				}

				if restoreFull {
					glog.Infof("Finalizing...")
					if err := h.IPodFinalize(false); err != nil {
						return fmt.Errorf("rebooting failed: %w", err)
					}
					glog.Infof("Please reformat the main partition of the device as FAT32, otherwise it will refuse to boot.")
				} else {
					glog.Infof("Resetting...")
					if err := h.IPodFinalize(true); err != nil {
						return fmt.Errorf("rebooting failed: %w", err)
					}
				}

				return nil
			}
		}
	},
}
