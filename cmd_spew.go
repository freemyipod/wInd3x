package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/syscfg"
	"github.com/freemyipod/wInd3x/pkg/uasm"
	"github.com/spf13/cobra"
)

func readFrom(app *app, addr uint32) ([]byte, error) {
	if err := dfu.Clean(app.usb); err != nil {
		return nil, fmt.Errorf("clean failed: %w", err)
	}

	dump := uasm.Program{
		Address: app.ep.ExecAddr(),
		Listing: app.ep.HandlerFooter(addr),
	}
	res, err := exploit.RCE(app.usb, app.ep, dump.Assemble(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute dump payload: %w", err)
	}
	return res, nil
}

var spewCmd = &cobra.Command{
	Use:   "spew",
	Short: "Display information about the connected device",
	Long:  "Displays SysCfg, GPIO, ... info from the connected device. Useful for reverse engineering and development.",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		fmt.Println("SysCfg")
		fmt.Println("------")

		syscfgBuf := bytes.NewBuffer(nil)
		err = readNOR(app, syscfgBuf, 0, 0, 0x100)
		if err != nil {
			fmt.Printf("Failed to read syscfg: %v\n", err)
		} else {

			scfg, err := syscfg.Parse(syscfgBuf)
			if err != nil {
				return fmt.Errorf("failed to parse syscfg: %w", err)
			}
			scfg.Debug(os.Stdout)
		}

		fmt.Println("GPIO")
		fmt.Println("----")

		fmt.Printf("                     01234567\n")
		// TODO: parametrize this per generation? Or are we lucky and the GPIO
		// periphs are always at the same addrs?
		for i := 0; i < 16; i++ {
			addr := 0x3cf0_0000 + i*0x20
			data, err := readFrom(app, uint32(addr))
			if err != nil {
				return fmt.Errorf("could not read GPIO %d: %w", i, err)
			}
			buf := bytes.NewBuffer(data)
			var con, dat uint32
			binary.Read(buf, binary.LittleEndian, &con)
			binary.Read(buf, binary.LittleEndian, &dat)

			state := ""
			dir := ""
			for j := 0; j < 8; j++ {
				if ((dat >> j) & 1) == 1 {
					state += "H"
				} else {
					state += "_"
				}

				c := (con >> (j * 4)) & 0xf
				switch c {
				case 0:
					dir += "i"
				case 1:
					dir += "O"
				case 2, 3, 4, 5, 6:
					dir += fmt.Sprintf("%d", c) // alternate function
				default:
					dir += "?"
				}
			}
			fmt.Printf("GPIO %03d-%03d: state: %s\n", i*8, i*8+7, state)
			fmt.Printf("                dir: %s\n", dir)
		}

		return nil
	},
}
