package efi

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/golang/glog"
)

// FirmwareVolumeHeader as per EFI spec.
type FirmwareVolumeHeader struct {
	Reserved [16]byte
	GUID     GUID
	// Length is recalculated when Serialize is called.
	Length        uint64
	Signature     [4]byte
	AttributeMask uint32
	HeaderLength  uint16
	// Checksum is recalculated when Serialize is called.
	Checksum        uint16
	ExtHeaderOffset uint16
	Reserved2       uint8
	Revision        uint8
}

func (h *FirmwareVolumeHeader) check() error {
	ffs1 := "7a9354d9-0468-444a-81ce-0bf617d890df"
	ffs2 := "8c8ce578-8a3d-4f1c-9935-896185c32dd3"
	if h.GUID.String() != ffs1 && h.GUID.String() != ffs2 {
		return fmt.Errorf("unknown GUID (%s)", h.GUID.String())
	}
	if !bytes.Equal(h.Signature[:], []byte("_FVH")) {
		return fmt.Errorf("invalid signature")
	}
	if h.HeaderLength < (0x38 + 8) {
		return fmt.Errorf("header length too small")
	}
	return nil
}

// Volume is an EFI Firmware Volume. It contains an array of Files, all of
// which contain recursively nested Sections.
type Volume struct {
	FirmwareVolumeHeader
	Files []*FirmwareFile
	// Custom is trailing data at the end of the Volume.
	Custom  []byte
	MinSize int
}

type blockmap struct {
	BlockCount uint32
	BlockSize  uint32
}

// Parse an EFI Firmware Volume from a NestedReader. After parsing, all files
// and sections within them will be available. These can then be arbitrarily
// modified, and Serialize can be called on the resulting Volume to rebuild a
// binary.
func ReadVolume(r *NestedReader) (*Volume, error) {
	var header FirmwareVolumeHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("reading volume header failed: %w", err)
	}

	if err := header.check(); err != nil {
		return nil, fmt.Errorf("volume header invalid: %w", err)
	}

	blockmapSize := header.HeaderLength - 0x38
	if blockmapSize%8 != 0 {
		return nil, fmt.Errorf("blockmap size not a multiple of 8")
	}
	bmapCount := blockmapSize / 8
	var bmap []blockmap
	for i := 0; i < int(bmapCount); i++ {
		var entry blockmap
		if err := binary.Read(r, binary.LittleEndian, &entry); err != nil {
			glog.Exit(err)
		}
		bmap = append(bmap, entry)
	}
	last := bmap[len(bmap)-1]
	if last.BlockCount != 0 || last.BlockSize != 0 {
		return nil, fmt.Errorf("blockmap does not end in (0, 0)")
	}

	if len(bmap) != 2 {
		return nil, fmt.Errorf("unsupported count of blockmaps (%d, wanted 2)", len(bmap))
	}

	glog.V(1).Infof("Blockmap: %+v", bmap)

	dataSize := bmap[0].BlockCount * bmap[0].BlockSize
	// This doesn't make sense, but otherwise that section is just too large. I
	// think it's an implementation bug in the iPod firmware.
	//dataSize -= 0x28 + 0x20
	//dataSize = 144928 - 0x50

	glog.V(1).Infof(" reader size: %x", r.Len()+r.pos)
	glog.V(1).Infof(" block count size: %x", dataSize)
	restSize := (r.Len() + r.pos) - int(dataSize)
	glog.V(1).Infof(" rest size: %x", restSize)

	//dataSub := r

	glog.V(1).Infof("Data size: %d bytes", dataSize)

	// Currently always 928 bytes of trailing data. That's the signature / cert
	// chain. We should also be able to recover this size from the IMG1 header.
	//if len(rest) != 928 {
	//	return nil, fmt.Errorf("trailing data of %d bytes", len(rest))
	//}

	var files []*FirmwareFile
	for r.Len() != 0 {
		file, err := readFile(r)
		if err != nil {
			return nil, fmt.Errorf("reading file %d failed: %v", len(files), err)
		}
		if file == nil {
			break
		}
		files = append(files, file)
	}
	glog.V(1).Infof("%d files", len(files))

	paddingLen := r.Len() - restSize
	glog.Infof("padding len: 0x%x", paddingLen)
	padding := make([]byte, paddingLen)
	r.Read(padding)
	if !bytes.Equal(padding, bytes.Repeat([]byte{0xff}, paddingLen)) {
		return nil, fmt.Errorf("padding is not all 0xFF")
	}

	rest, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading rest failed: %v", err)
	}
	glog.Infof("rest len: 0x%x", len(rest))
	glog.Infof("rest:\n%s", hex.Dump(rest))

	return &Volume{
		FirmwareVolumeHeader: header,
		Files:                files,
		Custom:               rest,
		MinSize:              int(dataSize),
	}, nil
}

func (v *Volume) Serialize() ([]byte, error) {
	// First, serialize all files apart from used padding file so that we know
	// how much data we're dealing with here.
	filesSize := 0
	fileData := make(map[int][]byte)
	for i, f := range v.Files {
		data, err := f.Serialize()
		if err != nil {
			return nil, fmt.Errorf("file %d: %w", i, err)
		}
		// Align all files to 8 bytes. I think generally we should align the
		// content to start at 16 bytes, with the header being an odd multiple
		// of 8, but this works for now?
		if len(data)%8 != 0 {
			pad := 8 - (len(data) % 8)
			data = append(data, bytes.Repeat([]byte{0xff}, pad)...)
		}
		fileData[i] = data
		filesSize += len(data)
	}
	// Now that we have a size, make a blockmap.
	totalSize := filesSize + 0x38 + 0x10
	if totalSize < v.MinSize {
		totalSize = v.MinSize
	}
	nblocks := uint32(totalSize / 256)
	bmap := []blockmap{
		{BlockCount: nblocks, BlockSize: 256},
		{BlockCount: 0, BlockSize: 0},
	}
	if totalSize%256 != 0 {
		totalSize += 256 - (totalSize % 256)
	}

	// Do final serialization pass into buffer.
	buf := bytes.NewBuffer(nil)
	// Header size.
	v.Length = 0
	// Blockmap size.
	v.HeaderLength = uint16(0x38 + 8*len(bmap))
	// Data size.
	v.Length += uint64(totalSize)
	v.ExtHeaderOffset = 0
	// TODO Reserved2/Revision?

	v.Checksum = 0
	checkBuf := bytes.NewBuffer(nil)
	binary.Write(checkBuf, binary.LittleEndian, v.FirmwareVolumeHeader)
	binary.Write(checkBuf, binary.LittleEndian, bmap)
	v.Checksum = checksum16(checkBuf.Bytes())

	if err := binary.Write(buf, binary.LittleEndian, v.FirmwareVolumeHeader); err != nil {
		// Shouldn't happen.
		panic(err)
	}
	if err := binary.Write(buf, binary.LittleEndian, bmap); err != nil {
		// Shouldn't happen.
		panic(err)
	}
	for i, f := range v.Files {
		if data, ok := fileData[i]; ok {
			if _, err := buf.Write(data); err != nil {
				// Shouldn't happen.
				panic(err)
			}
		} else {
			// Padding file.
			data, err := f.Serialize()
			if err != nil {
				// Shouldn't happen.
				panic(err)
			}
			if _, err := buf.Write(data); err != nil {
				// Shouldn't happen.
				panic(err)
			}
		}
	}

	buf.Write(bytes.Repeat([]byte{0xff}, totalSize-buf.Len()))
	buf.Write(v.Custom)
	return buf.Bytes(), nil
}
