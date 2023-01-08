package main

import (
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/exploit/decrypt"
	"github.com/freemyipod/wInd3x/pkg/image"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var decryptRecovery string

var decryptCmd = &cobra.Command{
	Use:   "decrypt [input] [output]",
	Short: "Decrypt DFU image",
	Long:  "Uses a connected device to decrypt a DFU image into a Haxed DFU compatible plaintext DFU image.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("could not open input: %w", err)
		}

		img, err := image.Read(f)
		if err != nil {
			return fmt.Errorf("could not read image: %w", err)
		}

		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		if app.Desc.Kind != img.DeviceKind {
			return fmt.Errorf("image is for %s, but %s is connected", img.DeviceKind, app.Desc.Kind)
		}

		res, err := decrypt.Decrypt(app, img.Body, decryptRecovery)
		if err != nil {
			return err
		}

		// Write image.
		wrapped, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, res)
		if err != nil {
			return fmt.Errorf("could not make image: %w", err)
		}

		if err := os.WriteFile(args[1], wrapped, 0600); err != nil {
			return fmt.Errorf("could not write image: %w", err)
		}

		glog.Infof("Done!")

		return nil
	},
}
