package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/golang/glog"
)

const (
	FormatSignedEncrypted     byte = 1
	FormatSigned              byte = 2
	FormatX509SignedEncrypted byte = 3
	FormatX509Signed          byte = 4
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
	copy(magic[:], []byte(dk.SoCCode()))

	buf := bytes.NewBuffer(nil)

	// Align body to 0x10.
	if (len(body) % 16) != 0 {
		pad := bytes.Repeat([]byte{0}, 16-(len(body)%16))
		body = append(body, pad...)
	}

	format := FormatX509Signed
	sigLength := 0x80
	certLength := 0x300
	var version [3]byte
	if dk == devices.Nano3 {
		copy(version[:], []byte("1.0"))
		format = FormatSigned
		sigLength = 0
		certLength = 0
	} else {
		copy(version[:], []byte("2.0"))
	}

	// Start off with the header.
	hdr := &IMG1Header{
		Magic:            magic,
		Version:          version,
		Format:           format,
		Entrypoint:       entrypoint,
		BodyLength:       uint32(len(body)),
		DataLength:       uint32(len(body) + sigLength + certLength),
		FooterCertOffset: uint32(len(body) + sigLength),
		FooterCertLength: uint32(certLength),
	}
	if err := binary.Write(buf, binary.LittleEndian, hdr); err != nil {
		return nil, fmt.Errorf("could not serialize header: %w", err)
	}

	// Pad to 0x600/0x800.
	if dk == devices.Nano3 {
		buf.Write(bytes.Repeat([]byte{0}, 0x800-buf.Len()))
	} else {
		buf.Write(bytes.Repeat([]byte{0}, 0x600-buf.Len()))
	}

	// Add body.
	buf.Write(body)

	// Add unused signature.
	buf.Write(bytes.Repeat([]byte{'S'}, sigLength))

	// Add unused certificates.
	buf.Write(bytes.Repeat([]byte{'C'}, certLength))

	return buf.Bytes(), nil
}

type IMG1 struct {
	Header     IMG1Header
	DeviceKind devices.Kind
	Body       []byte
}

var (
	ErrNotImage1 = errors.New("Not an IMG1 file")
)

func Read(r io.ReadSeeker) (*IMG1, error) {
	var hdr IMG1Header
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	var kind devices.Kind
	for _, k := range []devices.Kind{devices.Nano3, devices.Nano4, devices.Nano5, devices.Nano6, devices.Nano7} {
		if bytes.Equal(hdr.Magic[:], []byte(k.SoCCode())) {
			kind = k
			break
		}
	}
	if kind.String() == "UNKNOWN" {
		return nil, ErrNotImage1
	}

	if kind == devices.Nano3 {
		if !bytes.Equal(hdr.Version[:], []byte("1.0")) {
			return nil, fmt.Errorf("unsupported image version %q", hdr.Version)
		}
	} else {
		if !bytes.Equal(hdr.Version[:], []byte("2.0")) {
			return nil, fmt.Errorf("unsupported image version %q", hdr.Version)
		}
	}

	hdrSize := int64(0x600)
	switch kind {
	case devices.Nano3:
		hdrSize = 0x800
	case devices.Nano7:
		hdrSize = 0x400
	}
	if _, err := r.Seek(hdrSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek past header")
	}

	glog.Infof("Parsed %s image.", kind)

	body := make([]byte, hdr.BodyLength)
	if _, err := r.Read(body); err != nil {
		return nil, fmt.Errorf("could not read body")
	}

	// Ignore the rest of the fields, whatever.

	return &IMG1{
		Header:     hdr,
		DeviceKind: kind,
		Body:       body,
	}, nil
}
