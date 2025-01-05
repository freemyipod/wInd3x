package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var runCmd = &cobra.Command{
	Use:   "run [dfu image path]",
	Short: "Run a DFU image on a device",
	Long:  "Run a DFU image (signed/encrypted or unsigned) on a connected device, starting haxed dfu mode first if necessary.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		if err := haxeddfu.Trigger(app.Usb, app.Ep, false); err != nil {
			return fmt.Errorf("failed to run wInd3x exploit: %w", err)
		}

		path := args[0]
		glog.Infof("Uploading %s...", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read image: %w", err)
		}

		_, err = image.Read(bytes.NewReader(data))
		switch {
		case err == nil:
		case err == image.ErrNotImage1:
			fallthrough
		case len(data) < 0x400:
			glog.Infof("Given firmware file is not IMG1, packing into one...")
			data, err = image.MakeUnsigned(app.Desc.Kind, 0, data)
			if err != nil {
				return err
			}
		default:
			return err
		}

		if err := dfu.Clean(app.Usb); err != nil {
			return fmt.Errorf("Failed to clean: %w", err)
		}
		if err := dfu.SendImage(app.Usb, data, app.Desc.Kind.DFUVersion()); err != nil {
			return fmt.Errorf("Failed to send image: %w", err)
		}
		glog.Infof("Image sent.")

		return nil
	},
}
