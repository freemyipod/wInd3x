package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/cache"
	"github.com/freemyipod/wInd3x/pkg/cfw"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/efi"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var cfwCmd = &cobra.Command{
	Use:   "cfw",
	Short: "Custom firmware generation (EXPERIMENTAL)",
	Long:  "Build custom firmware bits. Very new, very undocumented. Mostly useful for devs.",
}

type findVisitor struct {
	want  []string
	found []*efi.FirmwareFile
}

func (v *findVisitor) Done() error {
	return nil
}

func (v *findVisitor) VisitFile(file *efi.FirmwareFile) error {
	for _, section := range file.Sections {
		if section.Header().Type == efi.SectionTypeUserInterface {
			name := string(bytes.ReplaceAll(section.Raw(), []byte{0}, []byte{}))
			if slices.Contains(v.want, name) {
				slog.Debug("Found", "name", name)
				v.found = append(v.found, file)
			}
		}
	}
	return nil
}

func (v *findVisitor) VisitSection(section efi.Section) error {
	return nil
}

func superdiags(app *app.App) ([]byte, error) {
	diagb, err := cache.Get(app, cache.PayloadKindDiagsDecrypted)
	if err != nil {
		return nil, fmt.Errorf("when getting diags: %w", err)
	}
	diagi, err := image.Read(bytes.NewReader(diagb))
	if err != nil {
		return nil, fmt.Errorf("when reading diags: %w", err)
	}
	diag, err := efi.ReadVolume(efi.NewNestedReader(diagi.Body))
	if err != nil {
		return nil, fmt.Errorf("when reading diags fv: %w", err)
	}
	bootb, err := cache.Get(app, cache.PayloadKindBootloaderDecrypted)
	if err != nil {
		return nil, fmt.Errorf("when getting bootloader: %w", err)
	}
	booti, err := image.Read(bytes.NewReader(bootb))
	if err != nil {
		return nil, fmt.Errorf("when reading bootloader: %w", err)
	}
	boot, err := efi.ReadVolume(efi.NewNestedReader(booti.Body))
	if err != nil {
		return nil, fmt.Errorf("when reading bootloader fv: %w", err)
	}

	fv := &findVisitor{
		want: []string{
			"DiskIoDxe",
			"Partition",
			"Image1FSReadOnly",
			"Nand",
		},
	}
	if err := cfw.VisitVolume(boot, fv); err != nil {
		return nil, fmt.Errorf("when visiting bootloader: %w", err)
	}
	if want, got := len(fv.want), len(fv.found); want != got {
		return nil, fmt.Errorf("did not find all requested modules (wanted %v, got %d)", fv.want, len(fv.found))
	}

	slog.Debug("Before append", "files", len(diag.Files))
	diag.Files = append(diag.Files, fv.found...)
	slog.Debug("After append", "files", len(diag.Files))
	diagb, err = diag.Serialize()
	if err != nil {
		return nil, fmt.Errorf("could not serialize superdiags fv: %w", err)
	}
	diagbi, err := image.MakeUnsigned(diagi.DeviceKind, diagi.Header.Entrypoint, diagb)
	if err != nil {
		return nil, fmt.Errorf("could not make superdiags image: %w", err)
	}
	return diagbi, nil
}

var cfwSuperdiagsCmd = &cobra.Command{
	Use:   "superdiags",
	Short: "Run superdiags",
	Long:  "Run superdiags (diag with extra Nand driver). If your iPod has a connected DCSD cable, you'll be able to access a console over it.",
	RunE: func(cmd *cobra.Command, args []string) error {

		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		diags, err := superdiags(&app.App)
		if err != nil {
			return err
		}

		wtf, err := cache.Get(&app.App, cache.PayloadKindWTFDefanged)
		if err != nil {
			return err
		}

		if err := haxeddfu.Trigger(app.Usb, app.Ep, false); err != nil {
			return fmt.Errorf("failed to run wInd3x exploit: %w", err)
		}
		slog.Info("Sending defanged WTF...")
		if err := dfu.SendImage(app.Usb, wtf, app.Desc.Kind.DFUVersion()); err != nil {
			return fmt.Errorf("failed to send image: %w", err)
		}

		slog.Info("Waiting 10s for device to switch to WTF mode...")
		ctx, ctxC := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer ctxC()
		if err := app.waitSwitch(ctx, devices.WTF); err != nil {
			return fmt.Errorf("device did not switch to WTF mode: %w", err)
		}
		time.Sleep(time.Second)

		slog.Info("Sending diags...")
		for i := 0; i < 10; i++ {
			err = dfu.SendImage(app.Usb, diags, app.Desc.Kind.DFUVersion())
			if err == nil {
				break
			} else {
				slog.Error("Error when sending diags", "err", err)
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			return err
		}

		slog.Info("Done.")

		return nil
	},
}

var cfwRunCmd = &cobra.Command{
	Use:   "run [firmware]",
	Short: "Run CFW",
	Long:  "Run CFW based on modified WTF and firmware (eg. modified OSOS or u-boot)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		fwb, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		wtf, err := cache.Get(&app.App, cache.PayloadKindWTFDefanged)
		if err != nil {
			return err
		}

		if err := haxeddfu.Trigger(app.Usb, app.Ep, false); err != nil {
			return fmt.Errorf("failed to run wInd3x exploit: %w", err)
		}
		slog.Info("Sending defanged WTF...")
		if err := dfu.SendImage(app.Usb, wtf, app.Desc.Kind.DFUVersion()); err != nil {
			return fmt.Errorf("failed to send image: %w", err)
		}

		_, err = image.Read(bytes.NewReader(fwb))
		switch {
		case err == nil:
		case err == image.ErrNotImage1:
			fallthrough
		case len(fwb) < 0x400:
			slog.Info("Given firmware file is not IMG1, packing into one...")
			fwb, err = image.MakeUnsigned(app.Desc.Kind, 0, fwb)
			if err != nil {
				return err
			}
		default:
			return err
		}

		slog.Info("Waiting 10s for device to switch to WTF mode...")
		ctx, ctxC := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer ctxC()
		if err := app.waitSwitch(ctx, devices.WTF); err != nil {
			return fmt.Errorf("device did not switch to WTF mode: %w", err)
		}
		time.Sleep(time.Second)

		slog.Info("Sending firmware...")
		for i := 0; i < 10; i++ {
			err = dfu.SendImage(app.Usb, fwb, app.Desc.Kind.DFUVersion())
			if err == nil {
				break
			} else {
				slog.Error("Error when sending firmware", "err", err)
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			return err
		}

		slog.Info("Done.")

		return nil
	},
}
