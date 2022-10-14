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

func readCP15(app *app, register, reg2, opc2 uint8) (uint32, error) {
	insns := app.ep.DisableICache()
	insns = append(insns,
		uasm.Ldr{Dest: uasm.R1, Src: uasm.Constant(0x22000000)},
		// Read ID code.
		uasm.Mrc{CPn: 15, Opc: 0, Dest: uasm.R0, CRn: register, CRm: reg2, Opc2: opc2},
		uasm.Str{Src: uasm.R0, Dest: uasm.Deref(uasm.R1, 0)},
	)
	insns = append(insns, app.ep.HandlerFooter(0x22000000)...)
	program := uasm.Program{
		Address: app.ep.ExecAddr(),
		Listing: insns,
	}
	if err := dfu.Clean(app.usb); err != nil {
		return 0, fmt.Errorf("clean failed: %w", err)
	}
	data, err := exploit.RCE(app.usb, app.ep, program.Assemble(), nil)
	if err != nil {
		return 0, fmt.Errorf("Failed to read ID code: %w", err)
	}
	var idcode uint32
	binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &idcode)
	return idcode, nil
}

func dumpCP15(app *app) {
	is1176 := false
	idcode, err := readCP15(app, 0, 0, 0)
	if err != nil {
		fmt.Printf("Failed to read ID Code: %v", err)
	} else {
		fmt.Printf("ID code: 0x%08x\n", idcode)
		switch (idcode >> 24) & 0xff {
		case 'A':
			fmt.Printf("  Implementer: ARM\n")
		default:
			fmt.Printf("  Implementer: unknown (0x%02x)\n", (idcode>>24)&0xff)
		}
		fmt.Printf("  Variant: 0x%x\n", (idcode>>20)&0xf)
		switch (idcode >> 16) & 0xf {
		case 6:
			fmt.Printf("  Architecture: ARMv5TEJ\n")
		case 0xf:
			fmt.Printf("  Architecture: See CPUID\n")
			is1176 = true
		default:
			fmt.Printf("  Architecture: unknown (%0x)\n", (idcode>>16)&0xf)
		}
		fmt.Printf("  Part number: %03x, Revision: %x\n", ((idcode) >> 4 & 0xfff), idcode&0xf)
	}

	fmt.Println("Extra Junk:")
	for _, el := range []struct {
		only1176 bool
		reg1     uint8
		reg2     uint8
		opc2     uint8
		desc     string
	}{
		{false, 0, 0, 0, "Main ID"},
		{false, 0, 0, 1, "Cache Type"},
		{false, 0, 0, 2, "TCM Status"},
		{false, 0, 0, 3, "TLB Type"},

		{true, 0, 1, 0, "Processor Feature 0"},
		{true, 0, 1, 1, "Processor Feature 1"},
		{true, 0, 1, 2, "Debug Feature 0"},
		{true, 0, 1, 3, "Auxiliary Feature 0"},
		{true, 0, 1, 4, "Memory Model Feature 0"},
		{true, 0, 1, 5, "Memory Model Feature 1"},
		{true, 0, 1, 6, "Memory Model Feature 2"},
		{true, 0, 1, 7, "Memory Model Feature 3"},

		{true, 0, 2, 0, "Instruction Set Feature Attribute 0"},
		{true, 0, 2, 1, "Instruction Set Feature Attribute 1"},
		{true, 0, 2, 2, "Instruction Set Feature Attribute 2"},
		{true, 0, 2, 3, "Instruction Set Feature Attribute 3"},
		{true, 0, 2, 4, "Instruction Set Feature Attribute 4"},
		{true, 0, 2, 5, "Instruction Set Feature Attribute 5"},

		{false, 1, 0, 0, "Control"},
		{true, 1, 0, 1, "Auxiliary Control"},
		{true, 1, 0, 2, "Coprocessor Access Control"},
		{true, 1, 1, 0, "Secure Configuration"},
		{true, 1, 1, 1, "Secure Debug Enable"},
		{true, 1, 1, 2, "Non-Secure Access Control"},

		{false, 2, 0, 0, "Translation Table Base 0"},
		{true, 2, 0, 1, "Translation Table Base 1"},
		{true, 2, 0, 2, "Translation Table Base Control"},

		{false, 3, 0, 0, "Domain Access Control"},
		{true, 7, 4, 0, "PCA"},
		{true, 7, 10, 6, "Cache Dirty Status"},

		{false, 9, 0, 0, "Data Cache Lockdown"},
		{false, 9, 0, 1, "Instruction Cache Lockdown"},
		{false, 9, 1, 0, "Data TCM Region"},
		{false, 9, 1, 1, "Instruction TCM Region"},
		{true, 9, 1, 2, "Data TCM Non-secure Control Access"},
		{true, 9, 1, 3, "Instruction TCM Non-secure Control Access"},
		{true, 9, 2, 0, "TCM Selection"},
		{true, 9, 8, 0, "Cache Behavior Override"},
	} {
		if el.only1176 && !is1176 {
			continue
		}
		res, err := readCP15(app, el.reg1, el.reg2, el.opc2)
		fmt.Printf("  CP15 c%d,c%d,%d (%s): ", el.reg1, el.reg2, el.opc2, el.desc)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("%08x\n", res)
		}
	}
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

		fmt.Println("\nCP15")
		fmt.Println("----")

		dumpCP15(app)

		fmt.Println("\nSysCfg")
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

		fmt.Println("\nGPIO")
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
