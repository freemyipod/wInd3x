package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
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
			return fmt.Errorf("Failed to run wInd3x exploit: %w", err)
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
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(haxDFUCmd)
	rootCmd.AddCommand(runCmd)
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
