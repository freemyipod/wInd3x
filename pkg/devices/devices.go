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
)

func (k Kind) String() string {
	switch k {
	case Nano3:
		return "Nano 3G"
	case Nano4:
		return "Nano 4G"
	case Nano5:
		return "Nano 5G"
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
	VID, DFUPID, WTFPID gousb.ID
	Kind                Kind
}

var Descriptions = []Description{
	{
		VID:    0x05ac,
		DFUPID: 0x1223,
		WTFPID: 0x1242,
		Kind:   Nano3,
	},
	{
		VID:    0x05ac,
		DFUPID: 0x1225,
		Kind:   Nano4,
	},
	{
		VID:    0x05ac,
		DFUPID: 0x1231,
		WTFPID: 0x1246,
		Kind:   Nano5,
	},
}
