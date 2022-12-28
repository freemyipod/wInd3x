package main

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/cfw"
	"github.com/freemyipod/wInd3x/pkg/efi"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var cfwCmd = &cobra.Command{
	Use:   "cfw",
	Short: "Custom firmware generation (EXPERIMENTAL)",
	Long:  "Build custom firmware bits. Very new, very undocumented. Mostly useful for devs.",
}

var cfwN5gTestCmd = &cobra.Command{
	Use:   "n5g_test [decrypted WTF] [WTF out]",
	Short: "Build N5G CFW",
	Long:  "Build experimental N5G CFW bootchain: currently a modified WTF",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvf, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("could not open decrypted WTF: %w", err)
		}
		img1, err := image.Read(fvf)
		if err != nil {
			return fmt.Errorf("failed to read decrypted WTF: %w", err)
		}
		nr := efi.NewNestedReader(img1.Body[0x100:])
		fv, err := efi.ReadVolume(nr)
		if err != nil {
			return fmt.Errorf("failed to read firmware volume: %w", err)
		}

		origSize, err := cfw.SecoreOffset(fv)
		if err != nil {
			return fmt.Errorf("failed to calculate original secore offset: %w", err)
		}
		glog.Infof("Initial pre-padding size: %d", origSize)

		glog.Infof("Applying patches...")
		if err := cfw.VisitVolume(fv, &cfw.N5GWTF); err != nil {
			return fmt.Errorf("failed to apply patches: %w", err)
		}

		glog.Infof("Fixing up padding...")
		if err := cfw.SecoreFixup(origSize, fv); err != nil {
			return fmt.Errorf("failed to fix up size: %w", err)
		}
		glog.Infof("Done.")

		fvb, err := fv.Serialize()
		if err != nil {
			return fmt.Errorf("failed to rebuild firmware: %w", err)
		}

		fvb = append(img1.Body[:0x100], fvb...)
		imb, err := image.MakeUnsigned(img1.DeviceKind, img1.Header.Entrypoint, fvb)
		if err != nil {
			return fmt.Errorf("failed to build new image1: %w", err)
		}

		if err := os.WriteFile(args[1], imb, 0644); err != nil {
			return fmt.Errorf("failed to write resulting WTF: %v", err)
		}
		return nil
	},
}
