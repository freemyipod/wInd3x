// Package mse implements parsing and unparsing of 'MSE' firmware bundle files,
// as seen in iPod firmware IPSWs.
//
// There is no spec for this format, and a lot of it seems to just be ad-hoc
// modified between generations. This library is designed to unparse files from
// all devices and be able to re-emit them while maintaining all the
// peculiarities of that generation's sub-format.
//
// Reference: http://www.ipodlinux.org/Firmware.html
package mse

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/freemyipod/wInd3x/pkg/devices"
)

// MSE firmware bundle, contains files. These should only be constructed from
// calling the Parse function.
type MSE struct {
	// Guard which contains copyright header.
	Guard string

	// The global volume/firmware header.
	VolumeHeader *VolumeHeader

	// Individual files.
	Files []*File

	// Guessed device kind.
	DeviceKind devices.Kind
}

// Firmware file, eg. osos or disk,.
type File struct {
	// Header, parsed directly from binary format.
	Header *FileHeader

	// PrefixHeader is set for some versions (N4G+) and some files, and is
	// effectively a light 'wrapper' around the file itself. If set during
	// parse, it will also be used during unparsing.
	PrefixHeader *PrefixHeader

	// Data kept in the file, eg. an IMG1 image or FAT16 filesystem.
	Data []byte
}

type PrefixHeader struct {
	Zero1 uint32
	Unk1  uint32
	Zero2 uint32
	Zero3 uint32
	Zero4 uint32
	Size  uint32
}

type FourCC uint32

func (f *FourCC) String() string {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(*f))
	return string(data)
}

func (f *FourCC) Set(s string) {
	data := []byte(s)
	if len(data) != 4 {
		panic(fmt.Sprintf("invalid fourcc: %q", s))
	}
	var u uint32
	binary.Read(bytes.NewReader(data), binary.BigEndian, &u)
	*f = FourCC(u)
}

type VolumeHeader struct {
	ID                   FourCC
	DirectoryOffset      uint32
	ExtendedHeaderOffset uint16
	Version              uint16
}

type FileHeader struct {
	// NAND or ATA!
	Target FourCC
	// osos or disk or ...
	Name FourCC
	// 0 for most files, 1 for an 'used' aupd.
	Used uint32
	// Offset within the MSE.
	Offset uint32
	// Length of the data contained. This well be recalculated to the file's
	// Data field length on unparse.
	Length uint32

	// All the following fields are just copied from the old iPodLinux wiki
	// article, and their exact function is not exactly tested.
	Address     uint32
	Entry       uint32
	Checksum    uint32
	Version     uint32
	LoadAddress uint32
}

func (f *FileHeader) Valid() bool {
	if f.Target.String() == "NAND" {
		return true
	}
	if f.Target.String() == "ATA!" {
		return true
	}
	return false
}

func guessGeneration(r io.ReadSeeker) (devices.Kind, error) {
	// Terrible hack! MSE files seem to have have slightly different packing
	// semantics between device versions, but don't have any metadata that would
	// tell us the device version.
	//
	// Easiest way to detect a version, pre-parse, is to count IMG1 header
	// fragments. This is obviously a terrible heuristic, but it works for now.

	headers := map[devices.Kind]string{
		devices.Nano3: devices.Nano3.SoCCode() + "1.0",
		devices.Nano4: devices.Nano4.SoCCode() + "2.0",
		devices.Nano5: devices.Nano5.SoCCode() + "2.0",
		devices.Nano6: devices.Nano6.SoCCode() + "2.0",
		devices.Nano7: devices.Nano7.SoCCode() + "2.0",
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return devices.Nano3, err
	}
	d, err := io.ReadAll(r)
	if err != nil {
		return devices.Nano3, err
	}

	best := devices.Nano3
	bestCount := 0
	for k, s := range headers {
		count := bytes.Count(d, []byte(s))
		if count > bestCount {
			bestCount = count
			best = k
		}
	}

	if bestCount == 0 {
		return devices.Nano3, fmt.Errorf("did not find any SoC codes")
	}
	return best, nil
}

func Parse(r io.ReadSeeker) (*MSE, error) {
	gen, err := guessGeneration(r)
	if err != nil {
		return nil, fmt.Errorf("could not guess device generation: %w", err)
	}
	log.Printf("Parsing MSE for (guessed) generation: %s", gen.String())

	r.Seek(0, io.SeekStart)
	guardB := make([]byte, 0x100)
	if _, err := io.ReadFull(r, guardB); err != nil {
		return nil, fmt.Errorf("could not read guard")
	}
	guard := string(guardB)
	if !strings.Contains(guard, "Copyright") || guardB[0xff] != 0 {
		return nil, fmt.Errorf("not a valid MSE file")
	}

	var vh VolumeHeader
	if err := binary.Read(r, binary.LittleEndian, &vh); err != nil {
		return nil, fmt.Errorf("failed to read volume header: %w", err)
	}
	if vh.ID.String() != "[hi]" {
		return nil, fmt.Errorf("invalid volume header id")
	}
	if vh.DirectoryOffset != 0x4000 {
		return nil, fmt.Errorf("unexpected directory offset %x", vh.DirectoryOffset)
	}
	if vh.ExtendedHeaderOffset != 0x10c {
		return nil, fmt.Errorf("unexpected extended header offset %x", vh.ExtendedHeaderOffset)
	}
	if vh.Version != 3 {
		return nil, fmt.Errorf("unexpected version %d", vh.Version)
	}

	padStart, _ := r.Seek(0, 1)
	pad := make([]byte, 0x5000-padStart)
	if _, err := io.ReadFull(r, pad); err != nil {
		return nil, fmt.Errorf("could not read padding")
	}
	if !bytes.Equal(pad, bytes.Repeat([]byte{0}, len(pad))) {
		return nil, fmt.Errorf("invalid padding")
	}

	var files []*File
	for i := 0; i < 16; i++ {
		var fh FileHeader
		if err := binary.Read(r, binary.LittleEndian, &fh); err != nil {
			return nil, fmt.Errorf("failed to read file header: %w", err)
		}
		files = append(files, &File{
			Header: &fh,
		})
	}

	for i, file := range files {
		if !file.Header.Valid() {
			continue
		}
		if _, err := r.Seek(int64(file.Header.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("could not seek to file %s at %x", file.Header.Name.String(), file.Header.Offset)
		}

		// Try to read PrefixHeader, adjust start offset if present.
		var header PrefixHeader
		if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
			return nil, fmt.Errorf("could not read optional file header: %v", err)
		}
		valid := true
		if header.Zero1 != 0 {
			valid = false
		}
		if header.Unk1 != 0 && header.Unk1 != 4 {
			valid = false
		}
		if header.Zero2 != 0 {
			valid = false
		}
		if header.Zero3 != 0 {
			valid = false
		}
		if header.Zero4 != 0 {
			valid = false
		}

		length := file.Header.Length
		if gen == devices.Nano3 {
			length += 0x1000
		}

		start := int64(file.Header.Offset)
		if valid {
			file.PrefixHeader = &header
			start += 0x1000
		}
		log.Printf("File %d: %s, offset %x, len %x, prefix: %v", i, file.Header.Name.String(), file.Header.Offset, file.Header.Length, valid)

		// Read main data.
		if _, err := r.Seek(start, io.SeekStart); err != nil {
			return nil, fmt.Errorf("could not seek to file %s at %x", file.Header.Name.String(), start)
		}
		file.Data = make([]byte, length)
		if _, err := io.ReadFull(r, file.Data); err != nil {
			return nil, fmt.Errorf("could not read file %s: %v", file.Header.Name.String(), err)
		}
	}

	m := MSE{
		Guard:        guard,
		VolumeHeader: &vh,
		Files:        files,
		DeviceKind:   gen,
	}

	return &m, nil
}

func (m *MSE) Serialize() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(m.Guard)

	// Calculate sizes and offsets for all files.
	var sectionSizes []int
	for _, fi := range m.Files {
		if !fi.Header.Valid() {
			continue
		}
		length := len(fi.Data)
		fi.Header.Length = uint32(length)
		if ph := fi.PrefixHeader; ph != nil {
			// The 'prefix header' length seems to always be the length, but
			// aligned to 16 bytes.
			ph.Size = uint32(length)
			if ph.Size%16 != 0 {
				ph.Size += 16 - (ph.Size % 16)
			}
		}

		if m.DeviceKind == devices.Nano3 {
			fi.Header.Length -= 0x1000
		}

		if ph := fi.PrefixHeader; ph != nil {
			length += 0x1000
		}
		if (length % 0x1000) != 0 {
			length += (0x1000 - (length % 0x1000))
		}
		sectionSizes = append(sectionSizes, int(length))
	}
	offs := 0x6000
	var sectionOffsets []int
	for _, size := range sectionSizes {
		sectionOffsets = append(sectionOffsets, offs)
		offs += size
	}
	sectionOffsets = append(sectionOffsets, offs)

	binary.Write(buf, binary.LittleEndian, m.VolumeHeader)
	// Pad to 0x5000
	pad := 0x5000 - buf.Len()
	buf.Write(bytes.Repeat([]byte{0}, pad))

	for i, fi := range m.Files {
		header := *fi.Header
		if header.Valid() {
			header.Offset = uint32(sectionOffsets[i])
		}
		binary.Write(buf, binary.LittleEndian, header)
	}

	for i, fi := range m.Files {
		if !fi.Header.Valid() {
			continue
		}
		// Pad to start of file.
		pad = sectionOffsets[i] - buf.Len()
		if pad < 0 {
			return nil, fmt.Errorf("file %d padding too long (%d too long)", i-1, -pad)
		}
		buf.Write(bytes.Repeat([]byte{0}, pad))
		// Write file data.
		if ph := fi.PrefixHeader; ph != nil {
			binary.Write(buf, binary.LittleEndian, ph)
			buf.Write(bytes.Repeat([]byte{0x00}, 0x200-6*4))
			buf.Write(bytes.Repeat([]byte{0xff}, 0xe00))
		}
		buf.Write(fi.Data)
		// Pad to next offset.
		paddingLen := sectionOffsets[i+1] - buf.Len()
		buf.Write(bytes.Repeat([]byte{0}, paddingLen))
	}

	return buf.Bytes(), nil
}
