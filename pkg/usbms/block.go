package usbms

import (
	"encoding/binary"
	"fmt"
)

const (
	readCapacity10Op OperationCode = 0x25
	read10Op         OperationCode = 0x28
)

// ReadCapacity issues READ CAPACITY (10) and returns the last addressable block and the block size.
func (h *Host) ReadCapacity() (lastLBA, blockSize uint32, err error) {
	cbd := &CommandDataBuffer{
		OperationCode:         readCapacity10Op,
		Request:               make([]byte, 8),
		Data:                  make([]byte, 8),
		DataTransferDirection: DataTransferFromDevice,
	}
	if err := h.RawCommand(cbd); err != nil {
		return 0, 0, err
	}
	if len(cbd.Data) < 8 {
		return 0, 0, fmt.Errorf("short response: %d bytes, want 8", len(cbd.Data))
	}
	return binary.BigEndian.Uint32(cbd.Data[0:4]), binary.BigEndian.Uint32(cbd.Data[4:8]), nil
}

// ReadBlocks issues READ (10) for count blocks starting at lba
func (h *Host) ReadBlocks(lba uint32, count uint16, blockSize uint32) ([]byte, error) {
	if count == 0 {
		return nil, fmt.Errorf("count must be non-zero")
	}
	req := make([]byte, 8)
	binary.BigEndian.PutUint32(req[1:5], lba)
	binary.BigEndian.PutUint16(req[6:8], count)

	cbd := &CommandDataBuffer{
		OperationCode:         read10Op,
		Request:               req,
		Data:                  make([]byte, int(blockSize)*int(count)),
		DataTransferDirection: DataTransferFromDevice,
	}
	if err := h.RawCommand(cbd); err != nil {
		return nil, fmt.Errorf("READ(10) lba %d count %d: %w", lba, count, err)
	}
	if want := int(blockSize) * int(count); len(cbd.Data) != want {
		return nil, fmt.Errorf("READ(10) lba %d returned %d bytes, want %d", lba, len(cbd.Data), want)
	}
	return cbd.Data, nil
}
