package usbms

import (
	"errors"
	"fmt"
	"time"
)

// The following is borrowed from
// https://github.com/monogon-dev/monogon/blob/main/metropolis/pkg/scsi/scsi.go
//
// Copyright 2023 The Monogon Project Authors.	// OperationCode contains the code of the command to be called
// SPDX-License-Identifier: Apache-2.0

type OperationCode uint8

const (
	InquiryOp        OperationCode = 0x12
	ReadDefectDataOp OperationCode = 0x37
	LogSenseOp       OperationCode = 0x4d
)

type DataTransferDirection uint8

const (
	DataTransferNone DataTransferDirection = iota
	DataTransferToDevice
	DataTransferFromDevice
	DataTransferBidirectional
)

// CommandDataBuffer represents a command
type CommandDataBuffer struct {
	OperationCode OperationCode
	// Request contains the OperationCode-specific request parameters
	Request []byte
	// ServiceAction can (for certain CDB encodings) contain an additional
	// qualification for the OperationCode.
	ServiceAction *uint8
	// Control contains common CDB metadata
	Control uint8
	// DataTransferDirection contains the direction(s) of the data transfer(s)
	// to be made.
	DataTransferDirection DataTransferDirection
	// Data contains the data to be transferred. If data needs to be received
	// from the device, a buffer needs to be provided here.
	Data []byte
	// Timeout can contain an optional timeout (0 = no timeout) for the command
	Timeout time.Duration
}

// Bytes returns the raw CDB to be sent to the device
func (c *CommandDataBuffer) Bytes() ([]byte, error) {
	// Table 24
	switch {
	case c.OperationCode < 0x20:
		// Use CDB6 as defined in Table 3
		if c.ServiceAction != nil {
			return nil, errors.New("ServiceAction field not available in CDB6")
		}
		if len(c.Request) != 4 {
			return nil, fmt.Errorf("CDB6 request size is %d bytes, needs to be 4 bytes without LengthField", len(c.Request))
		}

		outBuf := make([]byte, 6)
		outBuf[0] = uint8(c.OperationCode)

		copy(outBuf[1:5], c.Request)
		outBuf[5] = c.Control
		return outBuf, nil
	case c.OperationCode < 0x60:
		// Use CDB10 as defined in Table 5
		if len(c.Request) != 8 {
			return nil, fmt.Errorf("CDB10 request size is %d bytes, needs to be 4 bytes", len(c.Request))
		}

		outBuf := make([]byte, 10)
		outBuf[0] = uint8(c.OperationCode)
		copy(outBuf[1:9], c.Request)
		if c.ServiceAction != nil {
			outBuf[1] |= *c.ServiceAction & 0b11111
		}
		outBuf[9] = c.Control
		return outBuf, nil
	case c.OperationCode < 0x7e:
		return nil, errors.New("OperationCode is reserved")
	case c.OperationCode == 0x7e:
		// Use variable extended
		return nil, errors.New("variable extended CDBs are unimplemented")
	case c.OperationCode == 0x7f:
		// Use variable
		return nil, errors.New("variable CDBs are unimplemented")
	case c.OperationCode < 0xa0:
		// Use CDB16 as defined in Table 13
		if len(c.Request) != 14 {
			return nil, fmt.Errorf("CDB16 request size is %d bytes, needs to be 14 bytes", len(c.Request))
		}

		outBuf := make([]byte, 16)
		outBuf[0] = uint8(c.OperationCode)
		copy(outBuf[1:15], c.Request)
		if c.ServiceAction != nil {
			outBuf[1] |= *c.ServiceAction & 0b11111
		}
		outBuf[15] = c.Control
		return outBuf, nil
	case c.OperationCode < 0xc0:
		// Use CDB12 as defined in Table 7
		if len(c.Request) != 10 {
			return nil, fmt.Errorf("CDB12 request size is %d bytes, needs to be 10 bytes", len(c.Request))
		}

		outBuf := make([]byte, 12)
		outBuf[0] = uint8(c.OperationCode)
		copy(outBuf[1:11], c.Request)
		if c.ServiceAction != nil {
			outBuf[1] |= *c.ServiceAction & 0b11111
		}
		outBuf[11] = c.Control
		return outBuf, nil
	case c.OperationCode == 0xc6:
		// Special iPod operation code.
		limit := 5
		switch c.Request[0] {
		case IPodSubcommandUpdateStart, IPodSubcommandUpdateEnd, IPodSubcommandUpdateFinalize, IPodSubcommandRepartition:
			limit = 15
		case IPodSubcommandUpdateChunk:
			limit = 9
		default:
			return nil, fmt.Errorf("cannot serialize subcommand %x", c.Request[0])
		}

		if len(c.Request) > limit {
			return nil, fmt.Errorf("request too long")
		}
		res := make([]byte, limit+1)
		res[0] = byte(c.OperationCode)
		copy(res[1:], c.Request)
		return res, nil
	default:
		return nil, errors.New("unable to encode CDB for given OperationCode")
	}
}
