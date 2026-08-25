package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/image"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/mse"
)

var extractDir string

var mseCmd = &cobra.Command{
	Use:   "mse",
	Short: "Manipulate .mse firmware files",
}

var mseExtractCmd = &cobra.Command{
	Use:   "extract [Firmware.mse]",
	Short: "Extract an .mse firmware flie into images",
	Long:  "Split an .mse file into individual images like osos, disk, etc.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("could not read input: %w", err)
		}

		defer f.Close()
		m, err := mse.Parse(f)
		if err != nil {
			return fmt.Errorf("could not parse .mse: %w", err)
		}

		dir := extractDir
		if dir == "" {
			dir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("could not get working directory: %w", err)
			}
		}
		for _, file := range m.Files {
			if !file.Header.Valid() {
				continue
			}
			path := filepath.Join(dir, file.Header.Name.String())
			slog.Info("Extracting ...", "path", path)
			if err := os.WriteFile(path, file.Data, 0666); err != nil {
				return err
			}

			if m.DeviceKind == devices.Nano3 && file.Header.Name.String() == "hash" {
				// all 0xFF
				continue
			}

			bodyPath := filepath.Join(dir, file.Header.Name.String() + ".body")
			isRsrc := file.Header.Name.String() == "rsrc"
			var body []byte
			var img *image.IMG1

			// Nano3 rsrc is not an IMG1 (header/padding is 0x1000 of 0xFF then multiple 0x100 blocks of 0x00)
			// Nano4 rsrc is also not an IMG1 (header/padding is multiple 0x100 blocks of 0x00)
			if (m.DeviceKind == devices.Nano3 || m.DeviceKind == devices.Nano4) && isRsrc {
				var offset int

				if m.DeviceKind == devices.Nano3 {
					offset = 0x1000
				} else {
					offset = 0
				}

				blockSize := 0x100
				zeroBlock := make([]byte, blockSize)

				for {
					if offset+blockSize > len(file.Data) {
						break
					}

					if bytes.Equal(file.Data[offset:offset+blockSize], zeroBlock) {
						offset += blockSize
					} else {
						break
					}
				}

				body = file.Data[offset:]
			} else {
				img, err = image.Read(bytes.NewReader(file.Data))
				if err != nil {
					return err
				}

				body = img.Body
			}

			if isRsrc {
				if !bytes.Equal(body[0x1FE:0x200], []byte{0x55, 0xAA}) {
					slog.Warn("rsrc file MBR magic bytes [0x55, 0xAA] not found at offset 0x1FE")
					continue
				}

				slog.Info("Verified rsrc", "size", len(body))
			}

			if err := os.WriteFile(bodyPath, body, 0666); err != nil {
				return err
			}

			if img == nil {
				continue
			}

			calculatedDataLength := int(img.Header.BodyLength) + image.IMG1BodySignatureLength + int(img.Header.FooterCertLength)
			extraFileSize := len(file.Data) - image.IMG1BodyOffset[img.DeviceKind] - calculatedDataLength
			extraDataLength := int(img.Header.DataLength) - calculatedDataLength

			slog.Info(file.Header.Name.String(),
				"magic", string(img.Header.Magic[:]),
				"version", string(img.Header.Version[:]),
				"format", fmt.Sprintf("%s (%d)", image.IMG1Format[img.Header.Format], img.Header.Format),
				"entrypoint", fmt.Sprintf("0x%08x", img.Header.Entrypoint),
				"bodyLength", img.Header.BodyLength,
				"dataLength", img.Header.DataLength,
				"footerCertOffset", fmt.Sprintf("0x%08x", img.Header.FooterCertOffset),
				"footerCertLength", img.Header.FooterCertLength,
			)

			if extraFileSize > 0 || extraDataLength > 0 {
				slog.Warn("Length check(s) failed.",
					"extraFileSize", extraFileSize,
					"extraDataLength", extraDataLength,
				)
			}

			if img.Header.Format == image.FormatX509SignedEncrypted || img.Header.Format == image.FormatX509Signed {
				bodySignaturePath := filepath.Join(dir, file.Header.Name.String() + ".sign")
				if err := os.WriteFile(bodySignaturePath, img.BodySignature, 0666); err != nil {
					return err
				}

				certificateBundlePath := filepath.Join(dir, file.Header.Name.String() + ".cert")
				if err := os.WriteFile(certificateBundlePath, img.CertificateBundle, 0666); err != nil {
					return err
				}
			}
		}

		return nil
	},
}
