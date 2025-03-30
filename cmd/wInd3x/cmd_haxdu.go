package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
)

var haxDFUCmd = &cobra.Command{
	Use:   "haxdfu",
	Short: "Started 'haxed dfu' mode on a device",
	Long:  "Runs the wInd3x exploit to turn off security measures in the DFU that's currently running on a connected devices, allowing unsigned/unencrypted images to run.",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		if err := haxeddfu.Trigger(app.Usb, app.Ep, false); err != nil {
			return fmt.Errorf("failed to run wInd3x exploit: %w", err)
		}

		return nil
	},
}
