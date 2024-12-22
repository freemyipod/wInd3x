package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/syscfg"
	"github.com/freemyipod/wInd3x/pkg/uasm"
	"github.com/spf13/cobra"
)

func readFrom(app *app.App, addr uint32) ([]byte, error) {
	if err := dfu.Clean(app.Usb); err != nil {
		return nil, fmt.Errorf("clean failed: %w", err)
	}

	dump := uasm.Program{
		Address: app.Ep.ExecAddr(),
		Listing: app.Ep.HandlerFooter(addr),
	}
	res, err := exploit.RCE(app.Usb, app.Ep, dump.Assemble(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute dump payload: %w", err)
	}
	return res, nil
}

func readCP15(app *app.App, register, reg2, opc2 uint8) (uint32, error) {
	insns := app.Ep.DisableICache()
	insns = append(insns,
		uasm.Ldr{Dest: uasm.R1, Src: uasm.Constant(0x22000100)},
		// Read ID code.
		uasm.Mrc{CPn: 15, Opc: 0, Dest: uasm.R0, CRn: register, CRm: reg2, Opc2: opc2},
		uasm.Str{Src: uasm.R0, Dest: uasm.Deref(uasm.R1, 0)},
	)
	insns = append(insns, app.Ep.HandlerFooter(0x22000100)...)
	program := uasm.Program{
		Address: app.Ep.ExecAddr(),
		Listing: insns,
	}
	if err := dfu.Clean(app.Usb); err != nil {
		return 0, fmt.Errorf("clean failed: %w", err)
	}
	data, err := exploit.RCE(app.Usb, app.Ep, program.Assemble(), nil)
	if err != nil {
		return 0, fmt.Errorf("Failed to read ID code: %w", err)
	}
	var idcode uint32
	binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &idcode)
	return idcode, nil
}

func readCP14(app *app.App, opc2, reg2 uint8) (uint32, error) {
	insns := app.Ep.DisableICache()
	insns = append(insns,
		uasm.Ldr{Dest: uasm.R1, Src: uasm.Constant(0x22000100)},
		// Read ID code.
		uasm.Mrc{CPn: 14, Opc: 0, Dest: uasm.R0, CRn: 0, CRm: reg2, Opc2: opc2},
		uasm.Str{Src: uasm.R0, Dest: uasm.Deref(uasm.R1, 0)},
	)
	insns = append(insns, app.Ep.HandlerFooter(0x22000100)...)
	program := uasm.Program{
		Address: app.Ep.ExecAddr(),
		Listing: insns,
	}
	if err := dfu.Clean(app.Usb); err != nil {
		return 0, fmt.Errorf("clean failed: %w", err)
	}
	data, err := exploit.RCE(app.Usb, app.Ep, program.Assemble(), nil)
	if err != nil {
		return 0, fmt.Errorf("Failed to read ID code: %w", err)
	}
	var idcode uint32
	binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &idcode)
	return idcode, nil
}

func dumpCP15(app *app.App) {
	is1176 := false
	idcode, err := readCP15(app, 0, 0, 0)
	partnum := (idcode >> 4) & 0xfff
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
		fmt.Printf("  Part number: %03x, Revision: %x\n", partnum, idcode&0xf)
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
		// Ugly hack to skip some CP15 reads on Cortex A5 (N7G)
		// TODO(q3k): clean all of this up
		if partnum == 0xc05 && el.reg1 > 3 {
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

type peripheral struct {
	name      string
	registers []register
}

type register struct {
	name    string
	address uint32
	parse   func(v uint32)
}

var n3gates = map[int]string{
	0x00: "SHA",
	0x01: "LCD",
	0x02: "USBOTG",
	0x03: "SMx",
	0x04: "SM1",
	0x0A: "AES",
	0x08: "NAND",
	0x0C: "NANDECC",
	0x19: "DMAC0",
	0x1A: "DMAC1",
	0x1E: "ROM",
	0x22: "SPI0",
	0x23: "USBPHY",
	0x2B: "SPI1",
	0x2C: "GPIO",
	0x2E: "CHIPID",
	0x2F: "SPI2",
}

func printN3Gates(v uint32) {
	var gates []string
	for i := 0; i < 32; i++ {
		if (v & (1 << i)) == 0 {
			gate := n3gates[i]
			if gate == "" {
				gate = fmt.Sprintf("%d", i)
			}
			gates = append(gates, gate)
		}
	}
	fmt.Printf("%08x, %s\n", v, strings.Join(gates, ","))
}

var peripherals = map[devices.Kind][]peripheral{
	devices.Nano5: []peripheral{
		{name: "CHIPID", registers: []register{
			// eg. 00000001
			{name: "CID_VALID", address: 0x3d10_0000},
			// eg. 19000011
			{name: "CHIPIDL", address: 0x3d10_0004},
			// eg. 8730000b
			{name: "CHIPIDH", address: 0x3d10_0008},
			{name: "DIEIDL", address: 0x3d10_000C},
			{name: "DIEIDH", address: 0x3d10_0010},
			// eg. 00000004
			{name: "ECID_VERSION", address: 0x3d10_0014},
		}},
	},
	devices.Nano3: []peripheral{
		{name: "CLKGEN", registers: []register{
			{name: "CLKCON0", address: 0x3c50_0000},
			{name: "CLKCON1", address: 0x3c50_0004},
			{name: "CLKCON2", address: 0x3c50_0008},
			{name: "CLKCON3", address: 0x3c50_000C},
			{name: "CLKCON4", address: 0x3c50_0010},
			{name: "CLKCON5", address: 0x3c50_0014},

			{name: "PLLCON0", address: 0x3c50_0020},
			{name: "PLLCON1", address: 0x3c50_0024},
			{name: "PLLCON2", address: 0x3c50_0028},

			{name: "PLLCNT0", address: 0x3c50_0030},
			{name: "PLLCNT1", address: 0x3c50_0034},
			{name: "PLLCNT2", address: 0x3c50_0038},

			{name: "PLLLOCK", address: 0x3c50_0040},
			{name: "PLLMODE", address: 0x3c50_0044},

			{name: "GATES0", address: 0x3c50_0048, parse: func(v uint32) {
				printN3Gates(v)
			}},
			{name: "GATES1", address: 0x3c50_004C, parse: func(v uint32) {
				printN3Gates(v + 0x20)
			}},
		}},
		{name: "WATCHDOG", registers: []register{
			{name: "WDTCON", address: 0x3c80_0000},
			{name: "WDTCNT", address: 0x3c80_0004},
		}},
		{name: "CHIPID", registers: []register{
			{name: "CHIPIDUNK", address: 0x3d10_0000},
			// eg. 84000019
			{name: "CHIPIDL", address: 0x3d10_0004},
			// eg. 00008702
			{name: "CHIPIDH", address: 0x3d10_0008},
			{name: "DIEIDL", address: 0x3d10_000C},
			{name: "DIEIDH", address: 0x3d10_0010},
		}},
		{name: "EDGEIC", registers: []register{
			{name: "UNK0", address: 0x38e0_2000},
			{name: "UNK4", address: 0x38e0_2004},
			{name: "UNK8", address: 0x38e0_2008},
			{name: "UNKC", address: 0x38e0_200c},
		}},
		// Named per N3G/N4G rockbox branch.
		{name: "SYSALV", registers: []register{
			{name: "ALVCON", address: 0x39a0_0000},
			{name: "ALVUNK4", address: 0x39a0_0004},
			{name: "ALVUNK100", address: 0x39a0_0100},
			{name: "ALVUNK104", address: 0x39a0_0104},

			{name: "ALVTCOM", address: 0x39a0_006c},
			{name: "ALVTEND", address: 0x39a0_0070},
			{name: "ALVTDIV", address: 0x39a0_0074},
			{name: "ALVTCNT", address: 0x39a0_0078},
			{name: "ALVTSTAT", address: 0x39a0_007c},
		}},
	},
}

var spewCmd = &cobra.Command{
	Use:   "spew",
	Short: "Display information about the connected device",
	Long:  "Displays SysCfg, GPIO, ... info from the connected device. Useful for reverse engineering and development.",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := app.New()
		if err != nil {
			return err
		}
		defer app.Close()

		fmt.Println("\nCP15")
		fmt.Println("----")

		dumpCP15(app)

		fmt.Println("\nCP14 (debug)")
		fmt.Println("----")

		fmt.Printf("  DIDR: ")
		didr, err := readCP14(app, 0, 0)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			// 0x1512_1004
			// Watchpoint pairs: 1
			// Breakpoint pairs: 5
			// Breakpoint pairs with context ID: 1
			// Debug architecture: 2 (v6.1)
			// Trustzone Features
			// Revision: 4, Variant: 0,
			fmt.Printf("0x%08x\n", didr)
		}

		fmt.Printf("  DSCR: ")
		dscr, err := readCP14(app, 0, 1)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			// 0x0000_0002
			fmt.Printf("0x%08x\n", dscr)
		}

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

		for _, p := range peripherals[app.Desc.Kind] {
			fmt.Printf("\n%s\n", p.name)
			fmt.Printf("%s\n", strings.Repeat("-", len(p.name)))
			for _, reg := range p.registers {
				data, err := readFrom(app, reg.address)
				fmt.Printf("  %s: ", reg.name)
				if err != nil {
					fmt.Printf("error: %v\n", err)
				} else {
					var u32 uint32
					binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &u32)
					if reg.parse != nil {
						reg.parse(u32)
					} else {
						fmt.Printf("%08x\n", u32)
					}
				}
			}
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
