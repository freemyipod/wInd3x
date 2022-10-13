package syscfg

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

type Header struct {
	Tag    Tag
	Size   uint32
	Unk1   uint32
	Unk2   uint32
	Unk3   uint32
	NumKVs uint32
}

type Tag [4]byte

func (t Tag) String() string {
	return string([]byte{
		t[3], t[2], t[1], t[0],
	})
}

type handler func(r io.Reader) *Values

type Values struct {
	// SrNm is the serial number.
	SrNm string
	// FwId is the firmware ID.
	FwId []byte
	// HwId is the hardware ID.
	HwId []byte
	// HwVr is the hardware version.
	HwVr []byte
	// SwVr is the software version.
	SwVr string
	// MLBN is the main logic board (serial) number.
	MLBN string
	// ModN is the model number. It should be named Mod#.
	ModN string
	// RegN is the region.
	Regn []byte
}

func (v *Values) Debug(w io.Writer) {
	fmt.Fprintf(w, "     SrNm (serial number): %s\n", v.SrNm)
	fmt.Fprintf(w, "       FwId (firmware ID): %s\n", hex.EncodeToString(v.FwId))
	fmt.Fprintf(w, "       HwId (hardware ID): %s\n", hex.EncodeToString(v.HwId))
	fmt.Fprintf(w, "  HwVr (hardware version): %s\n", hex.EncodeToString(v.HwVr))
	fmt.Fprintf(w, "  SwVr (software version): %s\n", v.SwVr)
	fmt.Fprintf(w, "MLBN (logic board number): %s\n", v.MLBN)
	fmt.Fprintf(w, "      Mod# (model number): %s\n", v.ModN)
	fmt.Fprintf(w, "            Regn (region): %s\n", hex.EncodeToString(v.Regn))
}

func Parse(r io.Reader) (*Values, error) {
	var hdr Header
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	if hdr.Tag.String() != "SCfg" {
		return nil, fmt.Errorf("not a syscfg block")
	}

	var v Values
	for i := uint32(0); i < hdr.NumKVs; i++ {
		var tagB [4]byte
		if _, err := r.Read(tagB[:]); err != nil {
			return nil, fmt.Errorf("failed to read tag %d header: %w", i, err)
		}
		tag := Tag(tagB)
		// Data is always 16 bytes long... for now?
		data := make([]byte, 16)
		if _, err := r.Read(data); err != nil {
			return nil, fmt.Errorf("failed to read tag %d data: %w", i, err)
		}
		switch tag.String() {
		case "SrNm":
			v.SrNm = string(bytes.TrimRight(data, "\x00"))
		case "FwId":
			v.FwId = data
		case "HwId":
			v.HwId = data
		case "HwVr":
			v.HwVr = data
		case "SwVr":
			v.SwVr = string(bytes.TrimRight(data, "\x00"))
		case "MLBN":
			v.MLBN = string(bytes.TrimRight(data, "\x00"))
		case "Mod#":
			v.ModN = string(bytes.TrimRight(data, "\x00"))
		case "Regn":
			v.Regn = data
		default:
			return nil, fmt.Errorf("unknown tag %s", tag.String())
		}
	}
	return &v, nil
}
