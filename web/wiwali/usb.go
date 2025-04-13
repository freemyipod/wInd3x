package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"syscall/js"
	"time"
	"unicode/utf16"

	"github.com/freemyipod/wInd3x/pkg/devices"
)

// usb implements pkg/devices.USB backed in WebUSB, at least as much as
// possible.
type usb struct {
	usbDevice js.Value
}

func (u *usb) UseDefaultInterface() error {
	if _, err := await(u.usbDevice.Call("open")); err != nil {
		return fmt.Errorf("open: %w", err)
	}
	if _, err := await(u.usbDevice.Call("claimInterface", 0)); err != nil {
		return fmt.Errorf("claimInterface: %w", err)
	}
	if _, err := await(u.usbDevice.Call("selectAlternateInterface", 0, 0)); err != nil {
		return fmt.Errorf("selectAlternateInterface: %w", err)
	}
	slog.Info("Default interface set up.")
	return nil
}

func (u *usb) UseDiskInterface() (devices.UsbMsEndpoints, error) {
	return devices.UsbMsEndpoints{}, fmt.Errorf("unimplemented")
}

func (u *usb) Control(rType, request uint8, val, idx uint16, data []byte) (int, error) {
	toDevice := rType&0x80 == 0
	requestType := map[uint8]string{
		0: "standard",
		1: "class",
		2: "vendor",
		3: "reserved",
	}[(rType>>5)&0b11]
	recipient := map[uint8]string{
		0: "device",
		1: "interface",
		2: "endpoint",
		3: "other",
	}[(rType & 0b11111)]
	setup := map[string]any{
		"requestType": requestType,
		"recipient":   recipient,
		"request":     int(request),
		"value":       int(val),
		"index":       int(idx),
	}

	if toDevice {
		slog.Debug("Control OUT", "rType", rType, "request", request, "val", val, "idx", idx, "len", len(data))
		dataUint8Arr := toUint8Array(data)
		res, err := await(u.usbDevice.Call("controlTransferOut", setup, dataUint8Arr))
		if err != nil {
			return 0, err
		}
		status := res.Get("status").String()
		if status == "stall" {
			return 0, fmt.Errorf("stall")
		} else if status == "ok" {
			return res.Get("bytesWritten").Int(), nil
		} else {
			return 0, fmt.Errorf("invalid status: %q", status)
		}
	} else {
		slog.Debug("Control IN", "rType", rType, "request", request, "val", val, "idx", idx, "length", len(data))
		res, err := await(u.usbDevice.Call("controlTransferIn", setup, len(data)))
		if err != nil {
			return 0, err
		}
		status := res.Get("status").String()
		if status == "stall" {
			return 0, fmt.Errorf("stall")
		} else if status == "babble" {
			return 0, fmt.Errorf("babble")
		} else if status != "ok" {
			return 0, fmt.Errorf("invalid status: %q", status)
		}
		dataView := res.Get("data")
		bl := dataView.Get("byteLength").Int()
		for i := 0; i < bl; i++ {
			v := dataView.Call("getUint8", i).Int()
			data[i] = byte(v)
		}
		return bl, nil
	}
}

func (u *usb) SetControlTimeout(time.Duration) error {
	// TODO(q3k): don't call this for s5late-based sploits and throw an error here.
	return nil
}

func (u *usb) GetStringDescriptor(descIndex int) (string, error) {
	res, err := await(u.usbDevice.Call("controlTransferIn", map[string]any{
		"requestType": "standard",
		"recipient":   "device",
		"request":     0x06,
		"value":       0x0300 | descIndex,
		"index":       0x0000,
	}, 255))
	if err != nil {
		return "", fmt.Errorf("when getting descriptor: %w", err)
	}
	buffer := res.Get("data").Get("buffer")
	array := js.Global().Get("Uint8Array").New(buffer)
	data, err := fromUint8Array(array)
	if err != nil {
		return "", fmt.Errorf("could not load returned descriptor: %w", err)
	}
	if len(data) < 2 {
		return "", fmt.Errorf("returned descriptor is too short")
	}
	if len(data) != int(data[0]) || int(data[1]) != 3 {
		return "", fmt.Errorf("returned descriptor is corrupt")
	}
	arr := make([]uint16, len(data[2:])/2)
	binary.Decode(data[2:], binary.LittleEndian, arr)
	runes := utf16.Decode(arr)

	return string(runes), nil
}

func (u *usb) Close() error {
	return fmt.Errorf("Close unimplemented")
}
