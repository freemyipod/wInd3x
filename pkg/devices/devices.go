package devices

import (
	"github.com/google/gousb"
)

type Kind string

const (
	Nano4 Kind = "n4g"
	Nano5 Kind = "n5g"
)

func (k Kind) String() string {
	switch k {
	case Nano4:
		return "Nano 4G"
	case Nano5:
		return "Nano 5G"
	}
	return "UNKNOWN"
}

type Description struct {
	DFUVID, DFUPID gousb.ID
	Kind           Kind
}

var Descriptions = []Description{
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
