package main

import (
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/uasm"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var nandCmd = &cobra.Command{
	Use:   "nand",
	Short: "NAND Flash access (EXPERIMENTAL)",
	Long:  "Manipulate NAND Flash on the device. Currently this is EXPERIMENTAL, as the NAND access methods are not well reverse engineered.",
}

func nandReadPageOffset(a *app, bank, page, offset uint32) ([]byte, error) {
	ep := a.ep
	usb := a.usb

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

var nandReadCmd = &cobra.Command{
	Use:   "read [bank] [file]",
	Short: "Read NAND bank",
	Long:  "Read a 0x60000 'bank' (maybe?) of NAND. Slowly. Bank 0 contains the bootloader.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		bank, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid bank")
		}
		ep := app.ep
		usb := app.usb

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

		if err := dfu.Clean(app.usb); err != nil {
			return fmt.Errorf("clean failed: %w", err)
		}

		if _, err := exploit.RCE(usb, ep, init.Assemble(), nil); err != nil {
			return fmt.Errorf("failed to execute init payload: %w", err)
		}

		for p := uint32(0); p < 0x100; p += 1 {
			glog.Infof("%.2f%%...", float32(p)*100/0x100)
			for offs := uint32(0); offs < 0x600; offs += 0x40 {
				data, err := nandReadPageOffset(app, bank, p, offs)
				if err != nil {
					return err
				}
				f.Write(data)
			}
		}
		return nil
	},
}
