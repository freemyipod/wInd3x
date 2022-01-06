package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/freemyipod/wInd3x/pkg/dfu/image"
	"github.com/freemyipod/wInd3x/pkg/exploit/decrypt"
	"github.com/spf13/cobra"
)

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

		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		if app.desc.Kind != img.DeviceKind {
			return fmt.Errorf("image is for %s, but %s is connected", img.DeviceKind, app.desc.Kind)
		}

		log.Printf("Decrypting 0x%x bytes...", len(img.Body))

		w := bytes.NewBuffer(nil)

		// Since the decryption routine is janky and the first block
		// is always wrong, we have to commit a few sins here.
		log.Printf("Decrypting first block...")
		cipher := img.Body[:0x40]
		res, err := decrypt.Trigger(app.usb, app.ep, cipher)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		if _, err := w.Write(res); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
		ix := 0x40
		for {
			log.Printf("Decrypting 0x%x...", ix)
			ixe := ix + 0x30
			if ixe > len(img.Body) {
				ixe = len(img.Body)
			}
			b := img.Body[ix:ixe]
			b = append(b, bytes.Repeat([]byte{0}, 0x30-len(b))...)
			data := make([]byte, 0x40)
			copy(data[:0x10], cipher[0x30:0x40])
			copy(data[0x10:0x40], b)
			res, err = decrypt.Trigger(app.usb, app.ep, data)
			if err != nil {
				return fmt.Errorf("decryption failed: %w", err)
			}
			if _, err := w.Write(res[0x10:]); err != nil {
				return fmt.Errorf("write failed: %w", err)
			}

			if ixe != ix+0x30 {
				break
			}
			cipher = data
			ix += 0x30
		}

		// Write image.
		wrapped, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, w.Bytes())
		if err != nil {
			return fmt.Errorf("could not make image: %w", err)
		}

		if err := os.WriteFile(args[1], wrapped, 0600); err != nil {
			return fmt.Errorf("could not write image: %w", err)
		}

		log.Printf("Done!")

		return nil
	},
}
