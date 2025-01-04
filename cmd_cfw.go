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
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var cfwCmd = &cobra.Command{
	Use:   "cfw",
	Short: "Custom firmware generation (EXPERIMENTAL)",
	Long:  "Build custom firmware bits. Very new, very undocumented. Mostly useful for devs.",
}

var cfwRunCmd = &cobra.Command{
	Use:   "run [firmware]",
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
		switch {
		case err == nil:
		case err == image.ErrNotImage1:
			fallthrough
		case len(fwb) < 0x400:
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
