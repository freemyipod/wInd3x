package efi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
)

// FirmwareFileHeader as per EFI standard.
type FirmwareFileHeader struct {
	GUID GUID
	// ChecksumHeader is recalculated when Serialize is called.
	ChecksumHeader uint8
	// ChecksumData is recalculated when Serialize is called.
	ChecksumData uint8
	FileType     FileType
	Attributes   uint8
	// Size is recalculated when Serialize is called.
	Size  Uint24
	State uint8
}

type FileType uint8

const (
	FileTypeSecurityCore FileType = 3
	FileTypePEICore      FileType = 4
	FileTypeDXECore      FileType = 5
	FileTypeDriver       FileType = 7
	FileTypeApplication  FileType = 9
	FileTypePadding      FileType = 240
)

func (f FileType) String() string {
	switch f {
	case FileTypeSecurityCore:
		return "security core"
	case FileTypePEICore:
		return "pei core"
	case FileTypeDXECore:
		return "dxe core"
	case FileTypeDriver:
		return "driver"
	case FileTypeApplication:
		return "application"
	case FileTypePadding:
		return "padding"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", f)
	}
}

// FirmwareFile represents an EFI Firmware File within a Firmware Volume.
type FirmwareFile struct {
	FirmwareFileHeader
	Sections []Section
	// ReadOffset is the offset within the volume at which the file has been
	// encountered.
	ReadOffset int
}

func (f *FirmwareFile) Serialize() ([]byte, error) {
	var data []byte
	var err error
	if f.FileType == FileTypePadding {
		data = bytes.Repeat([]byte{0xff}, int(f.Size.Uint32()-0x18))
	} else {
		data, err = concatSections(f.Sections)
		if err != nil {
			return nil, fmt.Errorf("could not serialize sections: %w", err)
		}
	}

	f.Size = ToUint24(uint32(len(data)) + 0x18)

	f.ChecksumHeader = 0
	f.ChecksumData = 0
	state := f.State
	f.State = 0

	checkBuf := bytes.NewBuffer(nil)
	binary.Write(checkBuf, binary.LittleEndian, f.FirmwareFileHeader)

	f.ChecksumHeader = checksum8(checkBuf.Bytes())
	if (f.Attributes & 0x40) != 0 {
		f.ChecksumData = checksum8(data)
	} else {
		f.ChecksumData = 0xaa
	}
	f.State = state

	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, f.FirmwareFileHeader); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data); err != nil {
		panic(err)
	}

	return buf.Bytes(), nil
}

func readFile(r *NestedReader) (*FirmwareFile, error) {
	start := r.TellGlobal()
	var header FirmwareFileHeader
	peek := r.pos
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("reading header failed: %w", err)
	}

	if header.GUID.String() == "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		r.pos = peek
		return nil, nil
	}

	slog.Debug("File header", "start", start, "header", header)
	size := header.Size.Uint32()
	dataSub := r.Sub(0, int(size-0x18))
	r.Advance(int(size - 0x18))

	alignment := (size - 0x18) % 8
	if alignment != 0 {
		r.Advance(int(8 - alignment))
	}
	var sections []Section
	if header.FileType != FileTypePadding {
		var err error
		sections, err = readSections(dataSub)
		if err != nil {
			return nil, err
		}
	}
	// TODO: checksum
	return &FirmwareFile{
		FirmwareFileHeader: header,
		Sections:           sections,
		ReadOffset:         start,
	}, nil
}
