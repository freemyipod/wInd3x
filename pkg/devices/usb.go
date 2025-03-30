package devices

import (
	"errors"
	"time"
)

// Usb describes a common API to access an iPod (in any state - DFU, WTF,
// RetailOS, ...) over USB.
type Usb interface {
	// UseDefaultInterface requests the underlying provider to grant access to
	// control transfers to the default interface. This is most of our
	// interactions with the iPod.
	UseDefaultInterface() error

	// UseDiskInterface requests the underlying provider to grant access to the
	// USB Mass Storage API endpoints, taking them over from the default OS
	// driver.
	UseDiskInterface() (UsbMsEndpoints, error)

	// Control sends a control request to the device.
	Control(rType, request uint8, val, idx uint16, data []byte) (int, error)

	SetControlTimeout(time.Duration) error

	GetStringDescriptor(descIndex int) (string, error)

	// Close disposes of this device. No other functions may be called on the
	// interface afterwards.
	Close() error
}

type UsbMsInEndpoint interface {
	Read(buf []byte) (int, error)
}

type UsbMsOutEndpoint interface {
	Write(buf []byte) (int, error)
}

type UsbMsEndpoints struct {
	In  UsbMsInEndpoint
	Out UsbMsOutEndpoint
}

var UsbTimeoutError = errors.New("USB timeout error")
