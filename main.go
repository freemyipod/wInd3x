package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/exploit"
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

func main() {
	makeDFUCmd.Flags().StringVarP(&makeDFUEntrypoint, "entrypoint", "e", "0x0", "Entrypoint offset for image (added to load address == 0x2200_0000)")
	makeDFUCmd.Flags().StringVarP(&makeDFUDeviceKind, "kind", "k", "", "Device kind (one of 'n4g', 'n5g')")
	decryptCmd.Flags().StringVarP(&decryptRecovery, "recovery", "r", "", "EXPERIMENTAL: Path to temporary file used for recovery when restarting the transfer")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(haxDFUCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(makeDFUCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(decryptCmd)
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
