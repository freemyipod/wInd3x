package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/uasm"
	"github.com/spf13/cobra"
)

var nandCmd = &cobra.Command{
	Use:   "nand",
	Short: "NAND Flash access (EXPERIMENTAL)",
	Long:  "Manipulate NAND Flash on the device. Currently this is EXPERIMENTAL, as the NAND access methods are not well reverse engineered.",
}

func nandReadPageOffset(a *app.App, bank, page, offset uint32) ([]byte, error) {
	ep := a.Ep
	usb := a.Usb

	listing, dataAddr := ep.NANDReadPage(bank, page, offset)
	listing = append(listing, ep.HandlerFooter(dataAddr)...)
	read := uasm.Program{
		Address: ep.ExecAddr(),
		Listing: listing,
	}

	if err := dfu.Clean(usb); err != nil {
		return nil, fmt.Errorf("clean failed: %w", err)
	}

	resBuf, err := exploit.RCE(usb, ep, read.Assemble(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute read payload: %w", err)
	}
	return resBuf, nil
}

func nandIdentify(a *app.App) ([]byte, error) {
	ep := a.Ep
	usb := a.Usb

	listing, dataAddr := ep.NANDIdentify()
	listing = append(listing, ep.HandlerFooter(dataAddr)...)
	read := uasm.Program{
		Address: ep.ExecAddr(),
		Listing: listing,
	}

	if err := dfu.Clean(usb); err != nil {
		return nil, fmt.Errorf("clean failed: %w", err)
	}

	resBuf, err := exploit.RCE(usb, ep, read.Assemble(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute read payload: %w", err)
	}
	return resBuf, nil
}

var nandIdentifyCmd = &cobra.Command{
	Use:   "identify [bank]",
	Short: "Read NAND identifier for bank",
	Long:  "Read NAND identifer for bank",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		bank, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid bank")
		}
		ep := app.Ep
		usb := app.Usb

		listing := ep.DisableICache()
		payload, err := ep.NANDInit(bank)
		if err != nil {
			return err
		}
		listing = append(listing, payload...)
		listing = append(listing, ep.HandlerFooter(0x20000000)...)
		init := uasm.Program{
			Address: ep.ExecAddr(),
			Listing: listing,
		}

		if err := dfu.Clean(app.Usb); err != nil {
			return fmt.Errorf("clean failed: %w", err)
		}

		if _, err := exploit.RCE(usb, ep, init.Assemble(), nil); err != nil {
			return fmt.Errorf("failed to execute init payload: %w", err)
		}

		data, err := nandIdentify(&app.App)
		if err != nil {
			return err
		}

		fmt.Printf("JEDEC manufacturer ID: 0x%02X\n", data[0])
		fmt.Printf("JEDEC device ID: 0x%02X 0x%02X 0x%02X\n", data[1], data[2], data[3])

		return nil
	},
}

var nandReadCmd = &cobra.Command{
	Use:   "read [bank] [file]",
	Short: "Read NAND bank",
	Long:  "Read a 0x60000 'bank' (maybe?) of NAND. Slowly. Bank 0 contains the bootloader.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		bank, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid bank")
		}
		ep := app.Ep
		usb := app.Usb

		f, err := os.Create(args[1])
		if err != nil {
			return err
		}

		listing := ep.DisableICache()
		payload, err := ep.NANDInit(bank)
		if err != nil {
			return err
		}
		listing = append(listing, payload...)
		listing = append(listing, ep.HandlerFooter(0x20000000)...)
		init := uasm.Program{
			Address: ep.ExecAddr(),
			Listing: listing,
		}

		if err := dfu.Clean(app.Usb); err != nil {
			return fmt.Errorf("clean failed: %w", err)
		}

		if _, err := exploit.RCE(usb, ep, init.Assemble(), nil); err != nil {
			return fmt.Errorf("failed to execute init payload: %w", err)
		}

		for p := uint32(0); p < 0x100; p += 1 {
			slog.Info("Progress...", "percent", float32(p)*100/0x100)
			for offs := uint32(0); offs < 0x600; offs += 0x40 {
				data, err := nandReadPageOffset(&app.App, bank, p, offs)
				if err != nil {
					return err
				}
				f.Write(data)
			}
		}
		return nil
	},
}
