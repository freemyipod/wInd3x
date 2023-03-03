package devices

import (
	"github.com/google/gousb"

	"github.com/freemyipod/wInd3x/pkg/dfu"
)

type Kind string

const (
	Nano3 Kind = "n3g"
	Nano4 Kind = "n4g"
	Nano5 Kind = "n5g"
	Nano6 Kind = "n6g"
	Nano7 Kind = "n7g"
)

type InterfaceKind string

const (
	DFU  InterfaceKind = "dfu"
	WTF  InterfaceKind = "wtf"
	Disk InterfaceKind = "diskmode"
)

func (k Kind) String() string {
	switch k {
	case Nano3:
		return "Nano 3G"
	case Nano4:
		return "Nano 4G"
	case Nano5:
		return "Nano 5G"
	case Nano6:
		return "Nano 6G"
	case Nano7:
		return "Nano 7G"
	}
	return "UNKNOWN"
}

func (k Kind) SoCCode() string {
	switch k {
	case Nano3:
		return "8702"
	case Nano4:
		return "8720"
	case Nano5:
		return "8730"
	case Nano6:
		return "8723"
	case Nano7:
		return "8740"
	}
	return "INVL"
}

func (k Kind) DFUVersion() dfu.ProtoVersion {
	switch k {
	case Nano3:
		return dfu.ProtoVersion1
	default:
		return dfu.ProtoVersion2
	}
}

func (k Kind) Description() Description {
	for _, d := range Descriptions {
		if d.Kind == k {
			return d
		}
	}
	panic("unreachable")
}

type Description struct {
	VID             gousb.ID
	PIDs            map[InterfaceKind]gousb.ID
	UpdaterFamilyID int
	Kind            Kind
}

var Descriptions = []Description{
	{
		VID: 0x05ac,
		PIDs: map[InterfaceKind]gousb.ID{
			DFU:  0x1223,
			WTF:  0x1242,
			Disk: 0x1262,
		},
		UpdaterFamilyID: 26,
		Kind:            Nano3,
	},
	{
		VID: 0x05ac,
		PIDs: map[InterfaceKind]gousb.ID{
			DFU:  0x1225,
			WTF:  0x1243,
			Disk: 0x1263,
		},
		UpdaterFamilyID: 31,
		Kind:            Nano4,
	},
	{
		VID: 0x05ac,
		PIDs: map[InterfaceKind]gousb.ID{
			DFU:  0x1231,
			WTF:  0x1246,
			Disk: 0x1265,
		},
		UpdaterFamilyID: 34,
		Kind:            Nano5,
	},
	{
		VID: 0x05ac,
		PIDs: map[InterfaceKind]gousb.ID{
			DFU:  0x1232,
			WTF:  0x1248,
			Disk: 0x1266,
		},
		UpdaterFamilyID: 36,
		Kind:            Nano6,
	},
	{
		VID: 0x05ac,
		PIDs: map[InterfaceKind]gousb.ID{
			DFU:  0x1234,
			WTF:  0x1249,
			Disk: 0x1267,
		},
		UpdaterFamilyID: 37,
		Kind:            Nano7,
	},
}
