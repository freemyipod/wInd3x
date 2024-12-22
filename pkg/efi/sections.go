package efi

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/freemyipod/wInd3x/pkg/efi/compression"
	"github.com/golang/glog"
	"github.com/ulikunitz/xz/lzma"
)

type SectionType uint8

const (
	SectionTypeCompression         SectionType = 1
	SectionTypeGUIDDefined         SectionType = 2
	SectionTypePE32                SectionType = 16
	SectionTypeTE                  SectionType = 18
	SectionTypeDXEDEPEX            SectionType = 19
	SectionTypeVersion             SectionType = 20
	SectionTypeUserInterface       SectionType = 21
	SectionTypeFirmwareVolumeImage SectionType = 23
	SectionTypeRaw                 SectionType = 25
)

func (s SectionType) String() string {
	switch s {
	case SectionTypeCompression:
		return "compression"
	case SectionTypeGUIDDefined:
		return "guid"
	case SectionTypePE32:
		return "pe32"
	case SectionTypeTE:
		return "te"
	case SectionTypeDXEDEPEX:
		return "depex"
	case SectionTypeVersion:
		return "version"
	case SectionTypeUserInterface:
		return "ui"
	case SectionTypeFirmwareVolumeImage:
		return "firmware volume image"
	case SectionTypeRaw:
		return "raw"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

type SectionOrFile struct {
	Section Section
	File    *FirmwareFile
}

// Section is the interface implemented by all EFI Firmware Volume File
// Sections.
type Section interface {
	// Header returns the common header of this section.
	Header() *commonSectionHeader
	// Sub returns all Sections nested within this section, if applicable.
	Sub() []SectionOrFile
	// Serialize serializes this section into a binary.
	Serialize() ([]byte, error)

	// Raw returns the inner data within this section, if this section is a
	// PE32/TE/DXE/Raw section.
	Raw() []byte
	// SetRaw overrides the inner data within this section, if this section is
	// a PE32/TE/DXE/Raw section.
	SetRaw([]byte)
}

func readSections(r *NestedReader) ([]Section, error) {
	var res []Section
	for r.Len() != 0 {
		p1 := r.TellGlobal()
		section, err := readSection(r)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", len(res), err)
		}
		p2 := r.TellGlobal()
		read := p2 - p1
		if read%4 != 0 && r.Len() != 0 {
			align := 4 - (read % 4)
			r.Advance(align)
		}
		res = append(res, section)
	}
	return res, nil
}

type commonSectionHeader struct {
	Size Uint24
	Type SectionType
}

func (c *commonSectionHeader) Header() *commonSectionHeader {
	return c
}

func (c *commonSectionHeader) Raw() []byte {
	return nil
}

func (c *commonSectionHeader) SetRaw([]byte) {
}

type compressionSection struct {
	commonSectionHeader
	extra struct {
		UncompressedLength uint32
		CompressionType    uint8
	}
	sub []Section
}

func (c *compressionSection) Sub() []SectionOrFile {
	res := make([]SectionOrFile, 0, len(c.sub))
	for _, s := range c.sub {
		res = append(res, SectionOrFile{Section: s})
	}
	return res
}

func concatSections(sub []Section) ([]byte, error) {
	var res []byte
	if len(sub) == 0 {
		return nil, fmt.Errorf("no sections")
	}
	for i, section := range sub {
		data, err := section.Serialize()
		if err != nil {
			return nil, fmt.Errorf("sub %d: %w", i, err)
		}
		if len(data)%4 != 0 && (i != len(sub)-1) {
			data = append(data, bytes.Repeat([]byte{0x00}, 4-(len(data)%4))...)
		}
		res = append(res, data...)
	}
	return res, nil
}

func (c *compressionSection) Serialize() ([]byte, error) {
	uncompressed, err := concatSections(c.sub)
	if err != nil {
		return nil, err
	}
	c.extra.UncompressedLength = uint32(len(uncompressed))
	compressed, err := compression.Compress(uncompressed)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}
	c.commonSectionHeader.Size = ToUint24(uint32(4 + 5 + len(compressed)))

	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, c.commonSectionHeader); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, c.extra); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, compressed); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type guidSection struct {
	commonSectionHeader
	extra struct {
		SectionDefinitionGUID GUID
		DataOffset            uint16
		Attributes            uint16
	}
	custom []byte
	sub    []Section
}

func (c *guidSection) Sub() []SectionOrFile {
	res := make([]SectionOrFile, 0, len(c.sub))
	for _, s := range c.sub {
		res = append(res, SectionOrFile{Section: s})
	}
	return res
}

func (c *guidSection) Serialize() ([]byte, error) {
	data, err := concatSections(c.sub)
	if err != nil {
		return nil, err
	}
	if (c.extra.Attributes & 1) != 0 {
		switch c.extra.SectionDefinitionGUID.String() {
		case "ee4e5898-3914-4259-9d6e-dc7bd79403cf":
			// LZMA compressed
			wbuf := bytes.NewBuffer(nil)
			wrc := lzma.WriterConfig{
				SizeInHeader: true,
				Size:         int64(len(data)),
				BufSize:      1 << 15,
				DictCap:      16 << 20,
				//EOSMarker:    true,
			}
			w, err := wrc.NewWriter(wbuf)
			if err != nil {
				return nil, fmt.Errorf("could not open LZMA section: %w", err)
			}
			glog.V(2).Infof("  LZMA compressing %x bytes", len(data))
			if _, err := w.Write(data); err != nil {
				return nil, fmt.Errorf("could not compress LZMA section: %w", err)
			}
			if err := w.Close(); err != nil {
				return nil, fmt.Errorf("could not close LZMA section: %w", err)
			}
			data = wbuf.Bytes()
		default:
			return nil, fmt.Errorf("need to process unknown GUID %s", c.extra.SectionDefinitionGUID.String())
		}
	}
	c.commonSectionHeader.Size = ToUint24(uint32(4 + 20 + len(c.custom) + len(data)))
	if c.extra.SectionDefinitionGUID.String() == "fc1bcdb0-7d31-49aa-936a-a4600d9dd083" {
		// Rebuild CRC32 checksum.
		h := crc32.NewIEEE()
		h.Write(data)

		buf := bytes.NewBuffer(nil)
		binary.Write(buf, binary.LittleEndian, h.Sum32())
		c.custom = buf.Bytes()
	}

	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, c.commonSectionHeader); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, c.extra); err != nil {
		return nil, err
	}
	//pad := make([]byte, c.extra.DataOffset-24)
	if err := binary.Write(buf, binary.LittleEndian, c.custom); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type leafSection struct {
	commonSectionHeader
	data []byte
}

func (c *leafSection) Sub() []SectionOrFile {
	return nil
}

func (c *leafSection) Serialize() ([]byte, error) {
	c.commonSectionHeader.Size = ToUint24(uint32(4 + len(c.data)))
	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, c.commonSectionHeader); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, c.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *leafSection) Raw() []byte {
	res := make([]byte, len(c.data))
	copy(res, c.data)
	return res
}

func (c *leafSection) SetRaw(d []byte) {
	res := make([]byte, len(d))
	copy(res, d)
	c.data = res
}

type nestedImageSection struct {
	commonSectionHeader
	vol *Volume
}

func (c *nestedImageSection) Sub() []SectionOrFile {
	var res []SectionOrFile
	for _, f := range c.vol.Files {
		res = append(res, SectionOrFile{File: f})
		for _, s := range f.Sections {
			res = append(res, SectionOrFile{Section: s})
		}
	}
	return res
}

func (c *nestedImageSection) Serialize() ([]byte, error) {
	compressed, err := c.vol.Serialize()
	if err != nil {
		return nil, err
	}
	c.commonSectionHeader.Size = ToUint24(uint32(4 + len(compressed)))
	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, c.commonSectionHeader); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, compressed); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *nestedImageSection) Raw() []byte {
	return nil
}

func (c *nestedImageSection) SetRaw(d []byte) {
	return
}

func readSection(r *NestedReader) (Section, error) {
	var header commonSectionHeader
	start := r.TellGlobal()
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	glog.V(1).Infof("Section header @%08x: %+v", start, header)
	switch header.Type {
	case SectionTypeCompression:
		var res compressionSection
		res.commonSectionHeader = header
		if err := binary.Read(r, binary.LittleEndian, &res.extra); err != nil {
			return nil, err
		}
		data := make([]byte, header.Size.Uint32()-(4+5))
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("reading compression data: %w", err)
		}
		if res.extra.CompressionType != 1 {
			return nil, fmt.Errorf("unsupported compression type %d", res.extra.CompressionType)
		}
		decompressed, err := compression.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("decompression failed: %w", err)
		}
		t, err := compression.Compress(decompressed)
		if err != nil || len(t) != len(data) {
			glog.Warningf("Loopback compression failed: %d -> %d", len(data), len(t))
		}
		decompressed = decompressed[:res.extra.UncompressedLength]
		//fmt.Println(hex.Dump(decompressed))
		sub, err := readSections(NewNestedReader(decompressed))
		if err != nil {
			return nil, fmt.Errorf("parsing compression subsections: %w", err)
		}
		res.sub = sub
		return &res, nil
	case SectionTypeGUIDDefined:
		var res guidSection
		res.commonSectionHeader = header
		if err := binary.Read(r, binary.LittleEndian, &res.extra); err != nil {
			return nil, err
		}
		glog.V(2).Infof(" guid: %s, doffs: %x attrs: %x", res.extra.SectionDefinitionGUID.String(), res.extra.DataOffset, res.extra.Attributes)
		customLength := int(res.extra.DataOffset - (4 + 20))
		glog.V(2).Infof(" custom len: %x", customLength)
		custom := make([]byte, customLength)
		r.Read(custom)
		res.custom = custom
		if customLength != 0 {
			glog.V(2).Infof(" custom: %q", hex.EncodeToString(res.custom))
		}

		dataLength := int(header.Size.Uint32()-(4+20)) - customLength
		dataSub := r.Sub(0, dataLength)
		r.Advance(dataLength)

		if (res.extra.Attributes & 1) != 0 {
			glog.V(2).Infof(" needs processing")
			switch res.extra.SectionDefinitionGUID.String() {
			case "ee4e5898-3914-4259-9d6e-dc7bd79403cf":
				// LZMA compressed
				glog.V(2).Infof("  LZMA compressed")
				data, _ := io.ReadAll(dataSub)
				l, err := lzma.NewReader(bytes.NewBuffer(data))
				if err != nil {
					return nil, fmt.Errorf("could not open LZMA section: %w", err)
				}
				un, err := io.ReadAll(l)
				if err != nil {
					return nil, fmt.Errorf("could not decompress LZMA section: %w", err)
				}
				dataSub = NewNestedReader(un)
			default:
				return nil, fmt.Errorf("need to process unknown GUID %s", res.extra.SectionDefinitionGUID.String())
			}
		}

		sub, err := readSections(dataSub)
		if err != nil {
			return nil, fmt.Errorf("parsing guid defined subsections: %w", err)
		}
		res.sub = sub
		return &res, nil
	case SectionTypePE32, SectionTypeTE, SectionTypeRaw, SectionTypeDXEDEPEX, SectionTypeUserInterface, SectionTypeVersion:
		data := make([]byte, header.Size.Uint32()-(4))
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("reading data: %w", err)
		}
		return &leafSection{
			commonSectionHeader: header,
			data:                data,
		}, nil
	case SectionTypeFirmwareVolumeImage:
		glog.V(2).Infof(" nested firmware image volume")
		sub := r.Sub(0, int(header.Size.Uint32()))
		r.Advance(sub.Len())
		vol, err := ReadVolume(sub)
		if err != nil {
			return nil, fmt.Errorf("reading nested image: %w", err)
		}
		return &nestedImageSection{
			commonSectionHeader: header,
			vol:                 vol,
		}, nil
	}
	return nil, fmt.Errorf("unimplemented section type %s", header.Type)
}
