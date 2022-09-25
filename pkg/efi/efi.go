// package efi implements support for parsing and reassembling EFI Firmware
// Volumes, as used in some Apple device firmware components.
//
// This library is similar in functionality and scope to UEFITool or
// uefi-firmware-parser. However, some differences remain:
// 1. This implements the small subset of EFI FV as used by Apple devices, and
//    is only tested against them. This is in contrast to UEFITool and
//    uefi-firmware-parser which attempt to parse all possible images out
//    there.
// 2. This implementation is in pure Go, with Tiano compression routines
//    implemented via WebAssembly (emscripten-compiled C from EDK2). This is in
//    contrast to uefi-firmware-parser and UEFITool which link against a binary
//    build of the functionality from EDK2.
// 3. This implementation focuses on bit-perfect reconstruction of images. A
//    back-to-back Read-to-Serialize of any image should result in exactly the
//    same data outputted.
//
package efi

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// GUID type compatible with EFI.
type GUID [16]byte

func (g GUID) String() string {
	a := []byte{g[3], g[2], g[1], g[0]}
	b := []byte{g[5], g[4]}
	c := []byte{g[7], g[6]}
	d := []byte{g[8], g[9]}
	e := []byte{g[10], g[11], g[12], g[13], g[14], g[15]}
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(a), hex.EncodeToString(b), hex.EncodeToString(c), hex.EncodeToString(d), hex.EncodeToString(e))
}

func MustParseGUID(s string) GUID {
	if len(s) != 36 {
		panic("wrong guid length")
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		panic("invalid format")
	}

	lengths := []int{8, 4, 4, 4, 12}
	vs := make([][]byte, 5)
	for i, l := range lengths {
		if len(parts[i]) != l {
			panic("invalid format")
		}
		v, err := hex.DecodeString(parts[i])
		if err != nil {
			panic("inavlid format")
		}
		vs[i] = v
	}

	a := vs[0]
	b := vs[1]
	c := vs[2]
	d := vs[3]
	e := vs[4]

	return [16]byte{
		a[3], a[2], a[1], a[0],
		b[1], b[0],
		c[1], c[0],
		d[0], d[1],
		e[0], e[1], e[2], e[3], e[4], e[5],
	}
}

// NestedReader is a io.Reader which implements carving out a subelement of
// itself into another io.Reader. It also allows keeping track of the position
// of a reader within the original backing data.
type NestedReader struct {
	parent *NestedReader
	data   []byte
	pos    int
	start  int
}

func (r *NestedReader) Read(out []byte) (int, error) {
	left := len(r.data) - r.pos
	if left <= 0 {
		return 0, io.EOF
	}
	if len(out) < left {
		left = len(out)
	}
	copy(out, r.data[r.pos:r.pos+left])
	r.pos += left
	return left, nil
}

func (r *NestedReader) Advance(count int) {
	left := len(r.data) - r.pos
	if left < count {
		count = left
	}
	r.pos += count
}

func (r *NestedReader) TellGlobal() int {
	return r.pos + r.start
}

func (r *NestedReader) Sub(start, length int) *NestedReader {
	return &NestedReader{
		parent: r,
		data:   r.data[r.pos+start : r.pos+start+length],
		pos:    0,
		start:  r.start + r.pos + start,
	}
}

func (r *NestedReader) Len() int {
	return len(r.data) - r.pos
}

func NewNestedReader(underlying []byte) *NestedReader {
	return &NestedReader{
		parent: nil,
		data:   underlying,
		pos:    0,
		start:  0,
	}
}

// Uint24 as per EFI.
type Uint24 [3]uint8

func ToUint24(s uint32) Uint24 {
	if s > 0xffffff {
		panic("too large")
	}
	return [3]uint8{uint8(s & 0xff), uint8((s >> 8) & 0xff), uint8((s >> 16) & 0xff)}
}

func (s Uint24) Uint32() uint32 {
	return (uint32(s[2]) << 16) | (uint32(s[1]) << 8) | uint32(s[0])
}

// checksum16 is the 16-bit checksum as used in some EFI headers. It calculates
// the value necessary to make the given data sum to 0 when interpreted as an
// array of 16-bit integers.
func checksum16(data []byte) uint16 {
	if len(data)%2 != 0 {
		panic("cannot checksum non-16-bit-chunked data")
	}
	checkNums := make([]uint16, len(data)/2)
	binary.Read(bytes.NewBuffer(data), binary.LittleEndian, checkNums)
	sum := uint16(0)
	for _, n := range checkNums {
		sum += n
	}
	return (sum ^ 0xffff) + 1
}

// checksum8 is the 16-bit checksum as used in some EFI headers. It calculates
// the value necessary to make the given data sum to 0 when interpreted as an
// array of 8-bit integers.
func checksum8(data []byte) uint8 {
	checkNums := make([]uint8, len(data))
	binary.Read(bytes.NewBuffer(data), binary.LittleEndian, checkNums)
	sum := uint8(0)
	for _, n := range checkNums {
		sum += n
	}
	return (sum ^ 0xff) + 1
}
