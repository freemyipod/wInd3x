package main

import (
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/exploit/encryptsha1"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var encryptSHA1Cmd = &cobra.Command{
	Use:   "encrypt [input] [output]",
	Short: "Encrypt truncated binary sha1 hash file",
	Long:  "Uses a connected device to encrypt a sha1 hash in order to allow for creating valid IMG1 headers (v1/3/4) or IMG1 (v2) files.\nThis is used to manually assemble valid IMG1 headers (for easy exploitation of pwnage 2.0 or to create valid IMG1 v2 files for the iPod Nano 3g. Simply generate a binary sha1 file of the header (0x40 bytes) using a command like (openssl dgst -binary -sha1 header_file_0x40_bytes) then use a hex editor or a command like head to delete the last four bytes of the sha1 hash and produce a 0x10 byte long file that will be encrypted with the AES engine.\nThis encrypted hash can then used to assemble a valid IMG1 header or should be able to produce a full IMG1 on devices that support IMG1v2(iPod Nano 3g), assuming you encrypt the body sha1 in the appropriate place as well.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("Failed to read sha1 file: %w", err)
		}

		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		res, err := encryptsha1.Encryptsha1(app, f)
		if err != nil {
			return err
		}

		// Write sha1file.
		if err := os.WriteFile(args[1], res, 0600); err != nil {
			return fmt.Errorf("could not write image: %w", err)
		}

		glog.Infof("Done!")

		return nil
	},
}
