package main

import (
	"fmt"
	"io"
	"os"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/uasm"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var norCmd = &cobra.Command{
	Use:   "nor",
	Short: "NOR Flash access (EXPERIMENTAL)",
	Long:  "Manipulate SPI NOR Flash on the device. Currently this is EXPERIMENTAL, as the SPI NOR access methods are not well reverse engineered.",
}

func readNOR(app *app.App, w io.Writer, spino, offset, size uint32) error {
	ep := app.Ep
	usb := app.Usb

	listing := ep.DisableICache()
	payload, err := ep.NORInit(spino)
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

	for i := uint32(0); i < size; i += 0x40 {
		listing, dataAddr := ep.NORRead(spino, offset+i)
		listing = append(listing, ep.HandlerFooter(dataAddr)...)
		read := uasm.Program{
			Address: ep.ExecAddr(),
			Listing: listing,
		}
		if err := dfu.Clean(app.Usb); err != nil {
			return fmt.Errorf("clean failed: %w", err)
		}

		data, err := exploit.RCE(usb, ep, read.Assemble(), nil)
		if err != nil {
			return fmt.Errorf("failed to execute read payload: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
	}
	return nil
}

var norReadCmd = &cobra.Command{
	Use:   "read [spino] [address] [count] [file]",
	Short: "Read NOR flash",
	Long:  "Read N bytes from an address from given SPI peripheral.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		spino, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid spi peripheral number")
		}
		address, err := parseNumber(args[1])
		if err != nil {
			return fmt.Errorf("invalid address")
		}
		count, err := parseNumber(args[2])
		if err != nil {
			return fmt.Errorf("invalid count")
		}

		f, err := os.Create(args[3])
		if err != nil {
			return err
		}
		glog.Infof("Reading NOR address 0x%08x... (SPI %d, %d bytes)", address, spino, count)
		err = readNOR(app, f, spino, address, count)
		if err != nil {
			return err
		}
		glog.Infof("Done")
		return nil
	},
}
