package usbms

import (
	"encoding/binary"
	"fmt"
)

// MBRPartition is one entry of the four in an MBR partition table, with LBAs in
// whatever unit the device reports.
type MBRPartition struct {
	Index int
	Type  byte
	Start uint32
	Size  uint32
}

// Empty reports whether the slot is unused.
func (p MBRPartition) Empty() bool { return p.Type == 0 && p.Size == 0 }

// End is the first LBA past the partition.
func (p MBRPartition) End() uint32 { return p.Start + p.Size }

func (p MBRPartition) String() string {
	return fmt.Sprintf("part%d type 0x%02x LBA %d-%d (%d blocks)", p.Index, p.Type, p.Start, p.End()-1, p.Size)
}

// ParseMBR reads the four partition entries out of LBA 0
func ParseMBR(block []byte) ([]MBRPartition, error) {
	if len(block) < 512 {
		return nil, fmt.Errorf("block is %d bytes, need at least 512", len(block))
	}
	if block[0x1fe] != 0x55 || block[0x1ff] != 0xaa {
		return nil, fmt.Errorf("no MBR signature at +0x1fe (got %02x%02x)", block[0x1fe], block[0x1ff])
	}
	parts := make([]MBRPartition, 4)
	for i := range parts {
		e := block[0x1be+i*16 : 0x1be+(i+1)*16]
		parts[i] = MBRPartition{
			Index: i,
			Type:  e[4],
			Start: binary.LittleEndian.Uint32(e[8:12]),
			Size:  binary.LittleEndian.Uint32(e[12:16]),
		}
	}
	return parts, nil
}
