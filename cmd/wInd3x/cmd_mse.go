package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

			img, err := image.Read(bytes.NewReader(file.Data))
			if err != nil {
				return err
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

			path = filepath.Join(dir, file.Header.Name.String() + ".body")
			if err := os.WriteFile(path, img.Body, 0666); err != nil {
				return err
			}

			if img.Header.Format == image.FormatX509SignedEncrypted || img.Header.Format == image.FormatX509Signed {
				path = filepath.Join(dir, file.Header.Name.String() + ".sign")
				if err := os.WriteFile(path, img.BodySignature, 0666); err != nil {
					return err
				}

				path = filepath.Join(dir, file.Header.Name.String() + ".cert")
				if err := os.WriteFile(path, img.CertificateBundle, 0666); err != nil {
					return err
				}
			}
		}

		return nil
	},
}
