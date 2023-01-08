package main

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/dfu"
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

		path := args[0]
		glog.Infof("Uploading %s...", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read image: %w", err)
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
