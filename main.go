package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
)

var (
	flagImage string
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

	dev, err := findDevice(ctx)
	if err != nil {
		log.Fatalf("Device unavailable: %v", err)
	}
	if dev == nil {
		log.Fatalf("Device not found. Make sure it's in DFU mode.")
	}

	log.Printf("Found %s in DFU mode", dev.kind)
	if err := dev.clean(); err != nil {
		log.Fatalf("Could not get device into clean state: %v", err)
	}

	if err := dev.haxDFU(); err != nil {
		log.Fatalf("Failed to run wInd3x exploit: %v", err)
	}

	if flagImage != "" {
		log.Printf("Uploading %s...", flagImage)
		data, err := os.ReadFile(flagImage)
		if err != nil {
			log.Fatalf("Failed to read image: %v", err)
		}
		if err := dev.sendImage(data); err != nil {
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

func findDevice(ctx *gousb.Context) (*device, error) {
	var errs error
	for _, deviceDesc := range deviceDescriptions {
		usbDevice, err := ctx.OpenDeviceWithVIDPID(deviceDesc.vid, deviceDesc.pid)
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usbDevice == nil {
			continue
		}

		return &device{
			kind:          deviceDesc.kind,
			exploitParams: deviceDesc.exploitParams,
			usb:           usbDevice,
		}, nil
	}
	return nil, errs
}

func (d *device) clean() error {
	if err := d.clearStatus(); err != nil {
		return fmt.Errorf("ClrStatus: %w", err)
	}
	state, err := d.getState()
	if err != nil {
		return fmt.Errorf("GetState: %w", err)
	}
	if state != dfuIdle {
		return fmt.Errorf("unexpected DFU state %s", state)
	}
	return nil
}
