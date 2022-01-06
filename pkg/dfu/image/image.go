package image

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/freemyipod/wInd3x/pkg/devices"
)

const (
	FormatSignedEncrypted byte = 3
	FormatSigned          byte = 4
)

// IMG1Headers are also known as '8900' headers. More info:
// https://freemyipod.org/wiki/IMG1
type IMG1Header struct {
	Magic            [4]byte
	Version          [3]byte
	Format           byte
	Entrypoint       uint32
	BodyLength       uint32
	DataLength       uint32
	FooterCertOffset uint32
	FooterCertLength uint32
	Salt             [32]byte
	Unknown1         uint16
	SecurityEpoch    uint16
	HeaderSignature  [16]byte
}

func MakeUnsigned(dk devices.Kind, entrypoint uint32, body []byte) ([]byte, error) {
	var magic [4]byte
	switch dk {
	case devices.Nano4:
		copy(magic[:], []byte("8720"))
	case devices.Nano5:
		copy(magic[:], []byte("8730"))
	default:
		panic("unimplemented")
	}

	buf := bytes.NewBuffer(nil)

	// Align body to 0x10.
	if (len(body) % 16) != 0 {
		pad := bytes.Repeat([]byte{0}, 16-(len(body)%16))
		body = append(body, pad...)
	}

	// Start off with the header.
	hdr := &IMG1Header{
		Magic:            magic,
		Version:          [3]byte{'2', '.', '0'},
		Format:           FormatSigned,
		Entrypoint:       entrypoint,
		BodyLength:       uint32(len(body)),
		DataLength:       uint32(len(body) + 0x80 + 0x300),
		FooterCertOffset: uint32(len(body) + 0x80),
		FooterCertLength: 0x300,
	}
	if err := binary.Write(buf, binary.LittleEndian, hdr); err != nil {
		return nil, fmt.Errorf("could not serialize header: %w", err)
	}

	// Pad to 0x600.
	buf.Write(bytes.Repeat([]byte{0}, 0x600-buf.Len()))

	// Add body.
	buf.Write(body)

	// Add unused signature.
	buf.Write(bytes.Repeat([]byte{'S'}, 0x80))

	// Add unused certificates.
	buf.Write(bytes.Repeat([]byte{'C'}, 0x300))

	return buf.Bytes(), nil
}
