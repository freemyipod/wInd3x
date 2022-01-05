package main

import "github.com/google/gousb"

type deviceKind string

const (
	deviceNano4 deviceKind = "n4g"
	deviceNano5 deviceKind = "n5g"
)

type deviceDescription struct {
	vid, pid      gousb.ID
	kind          deviceKind
	exploitParams *exploitParameters
}

type device struct {
	kind          deviceKind
	exploitParams *exploitParameters
	usb           *gousb.Device
}

var deviceDescriptions = []deviceDescription{
	{
		vid:  0x05ac,
		pid:  0x1225,
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
	},
	{
		vid:  0x05ac,
		pid:  0x1231,
		kind: deviceNano5,
		exploitParams: &exploitParameters{
			dfuBufAddr:     0x2202db00,
			execAddr:       0x2202dc08,
			usbBufAddr:     0x2202e300,
			returnAddr:     0x20004d70,
			trampolineAddr: 0x37c,
			// b 0x2202dc08
			setupPacket: []byte{0x40, 0xfe, 0xff, 0xea, 0x03, 0x00, 0x00, 0x00},
		},
	},
}

func (d deviceKind) String() string {
	switch d {
	case deviceNano4:
		return "Nano 4G"
	case deviceNano5:
		return "Nano 5G"
	}
	return "UNKNOWN"
}
