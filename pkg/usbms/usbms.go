package usbms

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/freemyipod/wInd3x/pkg/devices"
)

type CBW struct {
	Signature          [4]byte
	Tag                uint32
	DataTransferLength uint32
	Flags              uint8
	LUN                uint8
	Length             uint8
	CB                 [16]byte
}

type CBS struct {
	Signature   [4]byte
	Tag         uint32
	DataResidue uint32
	Status      uint8
}

type Host struct {
	Endpoints devices.UsbMsEndpoints
	Tag       uint32
}

func (h *Host) InquiryVPD(page uint8, allocation uint16) ([]byte, error) {
	req := bytes.NewBuffer(nil)
	binary.Write(req, binary.BigEndian, struct {
		EVPD             uint8
		PageCode         uint8
		AllocationLength uint16
	}{1, page, allocation})
	data := make([]byte, allocation)
	cbd := &CommandDataBuffer{
		OperationCode:         InquiryOp,
		Request:               req.Bytes(),
		Data:                  data,
		DataTransferDirection: DataTransferFromDevice,
	}
	if err := h.RawCommand(cbd); err != nil {
		return nil, err
	}
	res := struct {
		EVPD       uint8
		PageCode   uint8
		PageLength uint16
	}{}
	binary.Read(bytes.NewBuffer(cbd.Data[:4]), binary.BigEndian, &res)
	if res.EVPD != 0 || res.PageCode != page {
		return nil, fmt.Errorf("invalid response: %+v", res)
	}
	return cbd.Data[4 : 4+res.PageLength], nil
}

func (h *Host) RawCommand(cbd *CommandDataBuffer) error {
	rlen := len(cbd.Data)
	cbw, err := h.buildCBW(cbd, uint32(rlen))
	if err != nil {
		return fmt.Errorf("building CBW failed: %w", err)
	}
	cbwb := cbw.Bytes()
	if _, err := h.Endpoints.Out.Write(cbwb); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	switch cbd.DataTransferDirection {
	case DataTransferFromDevice:
		n, err := h.Endpoints.In.Read(cbd.Data)
		if err != nil {
			return fmt.Errorf("data read failed: %w", err)
		}
		cbd.Data = cbd.Data[:n]
	case DataTransferToDevice:
		n, err := h.Endpoints.Out.Write(cbd.Data)
		if err != nil {
			return fmt.Errorf("data write failed: %w", err)
		}
		if want, got := len(cbd.Data), n; want != got {
			return fmt.Errorf("should've written %d bytes, wrote %d", want, got)
		}
	}

	cbsb := make([]byte, 13)
	if n, err := h.Endpoints.In.Read(cbsb); err != nil && n != 13 {
		return fmt.Errorf("status read failed: %w", err)
	}
	var cbs CBS
	binary.Read(bytes.NewBuffer(cbsb), binary.LittleEndian, &cbs)

	if !bytes.Equal(cbs.Signature[:], []byte("USBS")) {
		return fmt.Errorf("cbs signature invalid")
	}
	if cbs.Tag != cbw.Tag {
		return fmt.Errorf("tag mismatch: CBS %d != CBW %d", cbs.Tag, cbw.Tag)
	}
	if cbs.DataResidue != 0 {
		rlen -= int(cbs.DataResidue)
		cbd.Data = cbd.Data[:rlen]
	}
	if cbs.Status != 0 {
		return fmt.Errorf("cbs status: %d", cbs.Status)
	}

	return nil
}

func (h *Host) buildCBW(cbd *CommandDataBuffer, dataLength uint32) (*CBW, error) {
	data, err := cbd.Bytes()
	if err != nil {
		return nil, err
	}
	if len(data) > 16 {
		return nil, fmt.Errorf("cbd data too long")
	}

	var flags uint8
	switch cbd.DataTransferDirection {
	case DataTransferFromDevice:
		flags = 1 << 7
	case DataTransferToDevice, DataTransferNone:
	default:
		return nil, fmt.Errorf("DataTransferDirection must be to or from device or none")
	}

	h.Tag += 1

	cbw := CBW{
		Signature:          [4]byte{'U', 'S', 'B', 'C'},
		Tag:                h.Tag,
		DataTransferLength: dataLength,
		Flags:              flags,
		LUN:                0,
		Length:             uint8(len(data)),
	}
	copy(cbw.CB[:len(data)], data)
	return &cbw, nil
}

func (c *CBW) Bytes() []byte {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.LittleEndian, c)
	return buf.Bytes()
}
