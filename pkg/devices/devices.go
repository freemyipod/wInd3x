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
	if k == Nano3 {
		return dfu.ProtoVersion1
	}
	return dfu.ProtoVersion2
}

type Description struct {
	DFUVID, DFUPID gousb.ID
	Kind           Kind
}

var Descriptions = []Description{
	{
		DFUVID: 0x05ac,
		DFUPID: 0x1223,
		Kind:   Nano3,
	},
	{
		DFUVID: 0x05ac,
		DFUPID: 0x1225,
		Kind:   Nano4,
	},
	{
		DFUVID: 0x05ac,
		DFUPID: 0x1231,
		Kind:   Nano5,
	},
}
