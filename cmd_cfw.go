package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/cache"
	"github.com/freemyipod/wInd3x/pkg/cfw"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/efi"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var cfwCmd = &cobra.Command{
	Use:   "cfw",
	Short: "Custom firmware generation (EXPERIMENTAL)",
	Long:  "Build custom firmware bits. Very new, very undocumented. Mostly useful for devs.",
}

var cfwRunCmd = &cobra.Command{
	Use:   "run [modified WTF] [firmware]",
	Short: "Run CFW",
	Long:  "Run CFW based on modified WTF and firmware (eg. modified OSOS or u-boot)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		fwb, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		wtf, err := cache.Get(app, cache.PayloadKindWTFDefanged)
		if err != nil {
			return err
		}

		if err := haxeddfu.Trigger(app.Usb, app.Ep, false); err != nil {
			return fmt.Errorf("Failed to run wInd3x exploit: %w", err)
		}
		glog.Infof("Sending defanged WTF...")
		if err := dfu.SendImage(app.Usb, wtf, app.Desc.Kind.DFUVersion()); err != nil {
			return fmt.Errorf("Failed to send image: %w", err)
		}

		_, err = image.Read(bytes.NewReader(fwb))
		switch err {
		case nil:
		case image.ErrNotImage1:
			glog.Infof("Given firmware file is not IMG1, packing into one...")
			fwb, err = image.MakeUnsigned(app.Desc.Kind, 0, fwb)
			if err != nil {
				return err
			}
		default:
			return err
		}

		glog.Infof("Waiting 10s for device to switch to WTF mode...")
		ctx, ctxC := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer ctxC()
		if err := app.WaitWTF(ctx); err != nil {
			return fmt.Errorf("device did not switch to WTF mode: %w", err)
		}
		time.Sleep(time.Second)

		glog.Infof("Sending firmware...")
		for i := 0; i < 10; i++ {
			err = dfu.SendImage(app.Usb, fwb, app.Desc.Kind.DFUVersion())
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

		glog.Infof("Done.")

		return nil
	},
}

var cfwN5gTestCmd = &cobra.Command{
	Use:   "n5g_test [decrypted WTF] [WTF out]",
	Short: "Build N5G CFW",
	Long:  "Build experimental N5G CFW bootchain: currently a modified WTF",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvf, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("could not open decrypted WTF: %w", err)
		}
		img1, err := image.Read(fvf)
		if err != nil {
			return fmt.Errorf("failed to read decrypted WTF: %w", err)
		}
		nr := efi.NewNestedReader(img1.Body[0x100:])
		fv, err := efi.ReadVolume(nr)
		if err != nil {
			return fmt.Errorf("failed to read firmware volume: %w", err)
		}

		origSize, err := cfw.SecoreOffset(fv)
		if err != nil {
			return fmt.Errorf("failed to calculate original secore offset: %w", err)
		}
		glog.Infof("Initial pre-padding size: %d", origSize)

		glog.Infof("Applying patches...")
		if err := cfw.VisitVolume(fv, &cfw.N5GWTF); err != nil {
			return fmt.Errorf("failed to apply patches: %w", err)
		}

		glog.Infof("Fixing up padding...")
		if err := cfw.SecoreFixup(origSize, fv); err != nil {
			return fmt.Errorf("failed to fix up size: %w", err)
		}
		glog.Infof("Done.")

		fvb, err := fv.Serialize()
		if err != nil {
			return fmt.Errorf("failed to rebuild firmware: %w", err)
		}

		fvb = append(img1.Body[:0x100], fvb...)
		imb, err := image.MakeUnsigned(img1.DeviceKind, img1.Header.Entrypoint, fvb)
		if err != nil {
			return fmt.Errorf("failed to build new image1: %w", err)
		}

		if err := os.WriteFile(args[1], imb, 0644); err != nil {
			return fmt.Errorf("failed to write resulting WTF: %v", err)
		}
		return nil
	},
}
