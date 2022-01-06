package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
)

var runCmd = &cobra.Command{
	Use:   "run [dfu image path]",
	Short: "Run a DFU image on a device",
	Long:  "Run a DFU image (signed/encrypted or unsigned) on a connected device, starting haxed dfu mode first if necessary.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		if err := haxeddfu.Trigger(app.usb, app.ep, false); err != nil {
			return fmt.Errorf("Failed to run wInd3x exploit: %w", err)
		}

		path := args[0]
		log.Printf("Uploading %s...", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read image: %w", err)
		}
		if err := dfu.SendImage(app.usb, data); err != nil {
			return fmt.Errorf("Failed to send image: %w", err)
		}
		log.Printf("Image sent.")

		return nil
	},
}
