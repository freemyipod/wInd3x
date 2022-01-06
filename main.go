package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/dfu/image"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/exploit/dumpmem"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
)

var rootCmd = &cobra.Command{
	Use:   "wInd3x",
	Short: "wInd3x is an exploit tool for the iPod Nano 4G/5G",
	Long: `Allows to decrypt firmware files, generate DFU images and run unsigned DFU
images on the Nano 4G/5G.

Copyright 2022 q3k, user890104. With help from zizzy and d42.

wInd3x comes with ABSOLUTELY NO WARRANTY. This is free software, and you are
welcome to redistribute it under certain conditions; see COPYING file
accompanying distribution for details.`,
	SilenceUsage: true,
}

var haxDFUCmd = &cobra.Command{
	Use:   "haxdfu",
	Short: "Started 'haxed dfu' mode on a device",
	Long:  "Runs the wInd3x exploit to turn off security measures in the DFU that's currently running on a connected devices, allowing unsigned/unencrypted images to run.",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		if err := haxeddfu.Trigger(app.usb, app.ep, false); err != nil {
			return fmt.Errorf("failed to run wInd3x exploit: %w", err)
		}

		return nil
	},
}

var (
	makeDFUEntrypoint string
	makeDFUDeviceKind string
)
var makeDFUCmd = &cobra.Command{
	Use:   "makedfu [input] [output]",
	Short: "Build 'haxed dfu' unsigned image from binary",
	Long:  "Wraps a flat binary (loadable at 0x2200_0000) into an unsigned and unencrypted DFU image, to use with haxdfu/run.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("could not read input: %w", err)
		}

		var kind devices.Kind
		switch strings.ToLower(makeDFUDeviceKind) {
		case "":
			return fmt.Errorf("--kind must be set (one of: n4g, n5g)")
		case "n4g":
			kind = devices.Nano4
		case "n5g":
			kind = devices.Nano5
		default:
			return fmt.Errorf("--kind must be one of: n4g, n5g")
		}

		entrypoint, err := parseNumber(makeDFUEntrypoint)
		if err != nil {
			return fmt.Errorf("invalid entrypoint")
		}
		wrapped, err := image.MakeUnsigned(kind, entrypoint, data)
		if err != nil {
			return fmt.Errorf("could not make image: %w", err)
		}

		if err := os.WriteFile(args[1], wrapped, 0600); err != nil {
			return fmt.Errorf("could not write image: %w", err)
		}

		return nil
	},
}

var dumpCmd = &cobra.Command{
	Use:   "dump [offset] [size] [file]",
	Short: "Dump memory to file",
	Long:  "Read memory from a connected device and write results to a file. Not very fast.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		offset, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid offset")
		}
		size, err := parseNumber(args[1])
		if err != nil {
			return fmt.Errorf("invalid size")
		}

		f, err := os.Create(args[2])
		if err != nil {
			return fmt.Errorf("could not open file for writing: %w", err)
		}
		defer f.Close()

		for i := uint32(0); i < size; i += 0x40 {
			o := offset + i
			log.Printf("Dumping %x...", o)
			data, err := dumpmem.Trigger(app.usb, app.ep, o)
			if err != nil {
				return fmt.Errorf("failed to run wInd3x exploit: %w", err)
			}
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("failed to write: %w", err)
			}
		}

		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run [dfu image path]",
	Short: "Run a DFU image on a device",
	Long:  "Run a DFU image (signed/encrypted or unsigned) on a connected device, starting haxed dfu mode first if necessary.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

		if err := haxeddfu.Trigger(app.usb, app.ep, false); err != nil {
			log.Fatalf("Failed to run wInd3x exploit: %w", err)
		}

		path := args[0]
		log.Printf("Uploading %s...", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read image: %w", err)
		}
		if err := dfu.SendImage(app.usb, data); err != nil {
			return fmt.Errorf("Failed to send image: %w", err)
		}
		log.Printf("Image sent.")

		return nil
	},
}

func main() {
	makeDFUCmd.Flags().StringVarP(&makeDFUEntrypoint, "entrypoint", "e", "0x0", "Entrypoint offset for image (added to load address == 0x2200_0000)")
	makeDFUCmd.Flags().StringVarP(&makeDFUDeviceKind, "kind", "k", "", "Device kind (one of 'n4g', 'n5g')")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(haxDFUCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(makeDFUCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.Execute()
}

func newContext() (*gousb.Context, error) {
	resC := make(chan *gousb.Context)
	errC := make(chan error)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errC <- fmt.Errorf("%v", r)
			}
		}()

		resC <- gousb.NewContext()
	}()

	select {
	case err := <-errC:
		return nil, err
	case res := <-resC:
		return res, nil
	}
}

type app struct {
	ctx  *gousb.Context
	usb  *gousb.Device
	desc *devices.Description
	ep   *exploit.Parameters
}

func (a *app) close() {
	a.ctx.Close()
}

func newApp() (*app, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		usb, err := ctx.OpenDeviceWithVIDPID(deviceDesc.DFUVID, deviceDesc.DFUPID)
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usb == nil {
			continue
		}

		return &app{
			ctx:  ctx,
			usb:  usb,
			desc: &deviceDesc,
			ep:   exploit.ParametersForKind[deviceDesc.Kind],
		}, nil
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func parseNumber(s string) (uint32, error) {
	var err error
	var res uint64
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		res, err = strconv.ParseUint(s[2:], 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid number")
		}
	} else {
		res, err = strconv.ParseUint(s, 10, 32)
		if err != nil {
			res, err = strconv.ParseUint(s, 16, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid number")
			}
		}
	}
	return uint32(res), nil
}
