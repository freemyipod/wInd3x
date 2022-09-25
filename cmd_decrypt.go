package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

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

		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		if app.desc.Kind != img.DeviceKind {
			return fmt.Errorf("image is for %s, but %s is connected", img.DeviceKind, app.desc.Kind)
		}

		glog.Infof("Decrypting 0x%x bytes...", len(img.Body))

		w := bytes.NewBuffer(nil)

		// Create a temporary file that we can use to continue decryption from
		// after restarting the program.
		var recovery io.WriteCloser
		if decryptRecovery != "" {
			st, err := os.Stat(decryptRecovery)
			if err == nil {
				glog.Infof("Using recovery buffer at %s...", decryptRecovery)
				sz := st.Size()
				if (sz % 0x30) != 0 {
					return fmt.Errorf("recovery buffer invalid size (%x)", sz)
				}
				f, err := os.Open(decryptRecovery)
				if err != nil {
					return fmt.Errorf("could not open recovery buffer: %w", err)
				}
				if _, err := io.Copy(w, f); err != nil {
					return fmt.Errorf("could not read recovery buffer: %w", err)
				}
				f.Close()
			} else if os.IsNotExist(err) {
				glog.Infof("Creating recovery buffer at %s...", decryptRecovery)
			} else {
				return fmt.Errorf("could not access recoveyr buffer: %w", err)
			}
			recovery, err = os.OpenFile(decryptRecovery, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("could not open recovery buffer for append: %w", err)
			}
		}

		ix := w.Len()
		for {
			glog.Infof("Decrypting 0x%x (%.3f%%)...", ix, float64(ix*100)/float64(len(img.Body)))

			// Get plaintext block, pad to 0x30.
			ixe := ix + 0x30
			if ixe > len(img.Body) {
				ixe = len(img.Body)
			}
			b := img.Body[ix:ixe]
			b = append(b, bytes.Repeat([]byte{0}, 0x30-len(b))...)

			tries := 10
			var res []byte
			for {
				data := make([]byte, 0x40)
				// We need to feed the previous 0x10 bytes of ciphertext for...
				// some reason. Unless we're the first block.
				if ix == 0 {
					copy(data[:0x30], b)
				} else {
					copy(data[:0x10], img.Body[ix-0x10:ix])
					copy(data[0x10:0x40], b)
				}

				res, err = decrypt.Trigger(app.usb, app.ep, data)
				if err == nil {
					break
				}
				if tries < 1 {
					return fmt.Errorf("decryption failed, and out of retries: %w", err)
				} else {
					glog.Infof("Decryption failed (%v), retrying...", err)
					time.Sleep(100 * time.Millisecond)
					tries -= 1
				}
			}

			plaintext := res[0x10:0x40]
			if ix == 0 {
				plaintext = res[0x00:0x30]
			}
			if recovery != nil {
				if _, err := recovery.Write(plaintext); err != nil {
					return fmt.Errorf("write to recovery failed: %w", err)
				}
			}
			if _, err := w.Write(plaintext); err != nil {
				return fmt.Errorf("write failed: %w", err)
			}

			ix += 0x30
			if ix >= len(img.Body) {
				break
			}
		}

		// Write image.
		wrapped, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, w.Bytes())
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
