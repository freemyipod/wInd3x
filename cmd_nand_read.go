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

func readPageOffset(a *app, bank, page, offset uint32) ([]byte, error) {
	ep := a.ep
	usb := a.usb

	listing := ep.NANDReadPage(bank, page, offset)
	listing = append(listing, ep.HandlerFooter(0x20000000)...)
	read := uasm.Program{
		Address: ep.ExecAddr,
		Listing: listing,
	}

	if err := dfu.Clean(usb); err != nil {
		return nil, fmt.Errorf("clean failed: %w", err)
	}

	if _, err := exploit.RCE(usb, ep, read.Assemble(), nil); err != nil {
		return nil, fmt.Errorf("failed to execute read payload: %w", err)
	}

	resBuf := make([]byte, 0x40)
	n, err := usb.Control(0xa1, uint8(dfu.RequestUpload), 0, 0, resBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	if n != 0x40 {
		return nil, fmt.Errorf("only got %x bytes", n)
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

		if ep.NANDInit == nil {
			return fmt.Errorf("currently only implemented for N5G")
		}

		f, err := os.Create(args[1])
		if err != nil {
			return err
		}

		listing := ep.DisableICache
		listing = append(listing, ep.NANDInit...)
		listing = append(listing, ep.HandlerFooter(0x20000000)...)
		init := uasm.Program{
			Address: ep.ExecAddr,
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
				data, err := readPageOffset(app, bank, p, offs)
				if err != nil {
					return err
				}
				f.Write(data)
			}
		}
		return nil
	},
}
