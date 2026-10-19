package main

import (
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt [input] [output]",
	Short: "Encrypt one 16-byte block with the nano 4G Global key (zero IV)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		if len(data) != 16 {
			return fmt.Errorf("input must contain exactly 16 bytes")
		}
		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()
		result, err := exploit.EncryptBlock(app.Usb, app.Ep, data)
		if err != nil {
			return err
		}
		return os.WriteFile(args[1], result, 0600)
	},
}
