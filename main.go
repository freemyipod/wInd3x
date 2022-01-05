package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/haxeddfu"
	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
)

var (
	flagImage        string
	flagForceExploit bool
)

func main() {
	flag.BoolVar(&flagForceExploit, "force", false, "Force re-running haxed DFU exploit (repeated use might lock up device)")
	flag.StringVar(&flagImage, "image", "", "Path to DFU image to run.")
	flag.Parse()

	log.Printf("wInd3x - iPod Nano 4G and Nano 5G bootrom exploit")
	log.Printf("by q3k, with help from user890104, zizzy, d42")

	ctx, err := newContext()
	if err != nil {
		log.Fatalf("Failed to initialize USB: %v", err)
	}
	defer ctx.Close()

	usb, desc, err := findDevice(ctx)
	if err != nil {
		log.Fatalf("Device unavailable: %v", err)
	}
	if usb == nil {
		log.Fatalf("Device not found. Make sure it's in DFU mode.")
	}

	log.Printf("Found %s in DFU mode", desc.Kind)
	ep := exploit.ParametersForKind[desc.Kind]

	if err := dfu.Clean(usb); err != nil {
		log.Fatalf("Could not get device into clean state: %v", err)
	}

	if err := haxeddfu.Trigger(usb, ep, flagForceExploit); err != nil {
		log.Fatalf("Failed to run wInd3x exploit: %v", err)
	}

	if flagImage != "" {
		log.Printf("Uploading %s...", flagImage)
		data, err := os.ReadFile(flagImage)
		if err != nil {
			log.Fatalf("Failed to read image: %v", err)
		}
		if err := dfu.SendImage(usb, data); err != nil {
			log.Fatalf("Failed to send image: %v", err)
		}
		log.Printf("Image sent.")
		return
	}

	log.Fatalf("Device will now accept signed and unsigned DFU images.")
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

func findDevice(ctx *gousb.Context) (*gousb.Device, *devices.Description, error) {
	var errs error
	for _, deviceDesc := range devices.Descriptions {
		usb, err := ctx.OpenDeviceWithVIDPID(deviceDesc.DFUVID, deviceDesc.DFUPID)
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usb == nil {
			continue
		}

		return usb, &deviceDesc, nil
	}
	return nil, nil, errs
}
