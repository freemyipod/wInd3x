package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/cache"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
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

		app, err := newAny()
		if err != nil {
			return err
		}
		defer app.Close()

		hasBootloader := true
		switch app.Desc.Kind {
		case devices.Nano3:
			hasBootloader = false
		}

		switch restoreVersion {
		case "list":
			versions := cache.GetFirmwareVersions(app.Desc.Kind)
			slog.Info("Available versions:", "kind", app.Desc.Kind)
			for _, version := range versions {
				slog.Info(fmt.Sprintf("- %s", version))
			}
			return nil
		case "", "current":
		default:
			cache.FirmwareVersionOverrides = map[devices.Kind]string{
				app.Desc.Kind: restoreVersion,
			}
		}

		firmware, err := cache.Get(&app.App, cache.PayloadKindFirmwareUpstream)
		if err != nil {
			return fmt.Errorf("could not get firmware: %w", err)
		}

		var bootloader []byte
		if hasBootloader {
			bootloader, err = cache.Get(&app.App, cache.PayloadKindBootloaderUpstream)
			if err != nil {
				return fmt.Errorf("could not get bootloader: %s", err)
			}
		}

		for {
			slog.Info("Found device", "device", app.Desc.Kind, "interface", app.InterfaceKind)
			switch app.InterfaceKind {
			case devices.DFU:
				wtf, err := cache.Get(&app.App, cache.PayloadKindWTFUpstream)
				if err != nil {
					return fmt.Errorf("could not get wtf payload: %s", err)
				}
				slog.Info("Sending WTF...")
				if err := dfu.SendImage(app.Usb, wtf, app.Desc.Kind.DFUVersion()); err != nil {
					return fmt.Errorf("Failed to send image: %w", err)
				}
				slog.Info("Waiting 10s for device to switch to WTF mode...")
				ctx, _ := context.WithTimeout(cmd.Context(), 10*time.Second)
				if err := app.waitSwitch(ctx, devices.WTF); err != nil {
					return fmt.Errorf("device did not switch to WTF mode: %w", err)
				}
				time.Sleep(time.Second)
			case devices.WTF:
				recovery, err := cache.Get(&app.App, cache.PayloadKindRecoveryUpstream)
				if err != nil {
					return fmt.Errorf("could not get recovery payload: %s", err)
				}
				slog.Info("Sending recovery firmware...")
				for i := 0; i < 10; i++ {
					err = dfu.SendImage(app.Usb, recovery, app.Desc.Kind.DFUVersion())
					if err == nil {
						break
					} else {
						slog.Error("Sending recovery failed", "err", err)
						time.Sleep(time.Second)
					}
				}
				if err != nil {
					return err
				}
				slog.Info("Waiting 30s for device to switch to Recovery mode...")
				ctx, _ := context.WithTimeout(cmd.Context(), 30*time.Second)
				if err := app.waitSwitch(ctx, devices.Disk); err != nil {
					return fmt.Errorf("device did not switch to Recovery mode: %w", err)
				}
				time.Sleep(time.Second)
			case devices.Disk:
				h := usbms.Host{
					Endpoints: app.MSEndpoints,
				}
				di, err := h.IPodDeviceInformation()
				if err != nil {
					slog.Error("Could not get device information", "err", err)
				} else {
					fmt.Printf("SerialNumber: %s\n", di.SerialNumber)
					fmt.Printf("     BuildID: %s\n", di.BuildID)
				}

				if restoreFull {
					partsize := len(firmware)
					slog.Info("Reformatting system partition...", "mib", partsize>>20)
					if err := h.IPodRepartition(partsize); err != nil {
						return fmt.Errorf("repartitioning failed: %w", err)
					}
					if hasBootloader {
						slog.Info("Writing bootloader...")
						if err := h.IPodUpdateSendFull(usbms.IPodUpdateBootloader, bootloader); err != nil {
							return fmt.Errorf("writing bootloader failed: %w", err)
						}
					}
				}
				slog.Info("Writing firmware...")
				if err := h.IPodUpdateSendFull(usbms.IPodUpdateFirmware, firmware); err != nil {
					return fmt.Errorf("writing firmware failed: %w", err)
				}

				if restoreFull {
					slog.Info("Finalizing...")
					if err := h.IPodFinalize(false); err != nil {
						return fmt.Errorf("rebooting failed: %w", err)
					}
					slog.Info("Please reformat the main partition of the device as FAT32, otherwise it will refuse to boot.")
				} else {
					slog.Info("Resetting...")
					if err := h.IPodFinalize(true); err != nil {
						return fmt.Errorf("rebooting failed: %w", err)
					}
				}

				return nil
			}
		}
	},
}
