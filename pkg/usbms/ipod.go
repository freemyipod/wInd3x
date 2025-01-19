package usbms

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"howett.net/plist"
)

type DeviceInformation struct {
	UpdaterFanilyVersion int    `plist:"UpdaterFamilyVersion"`
	BuildID              string `plist:"BuildID"`
	SerialNumber         string `plist:"SerialNumber"`
}

func (h *Host) IPodDeviceInformation() (*DeviceInformation, error) {
	if _, err := h.InquiryVPD(0xc0, 0xfc); err != nil {
		return nil, err
	}
	var res []byte
	for i := uint8(0xc2); i <= 0xff; i++ {
		page, err := h.InquiryVPD(i, 0xfc)
		if err != nil {
			return nil, err
		}
		res = append(res, page...)
		if len(page) != 0xfc-4 {
			break
		}
	}
	var di DeviceInformation
	if _, err := plist.Unmarshal(res, &di); err != nil {
		return nil, err
	}
	return &di, nil
}

//type IPodSubcommand uint8

const (
	IPodSubcommandUpdateStart    uint8 = 0x90
	IPodSubcommandUpdateChunk    uint8 = 0x91
	IPodSubcommandUpdateEnd      uint8 = 0x92
	IPodSubcommandRepartition    uint8 = 0x94
	IPodSubcommandUpdateFinalize uint8 = 0x31
)

func (h *Host) IPodRepartition(targetSize int) error {
	if (targetSize % 4096) != 0 {
		return fmt.Errorf("target size must be 12 bit aligned")
	}
	sectors := targetSize >> 12
	partsize := uint32(sectors << 2)

	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, struct {
		Subcommand uint8
		Size       uint32
	}{IPodSubcommandRepartition, partsize})
	cbd := &CommandDataBuffer{
		OperationCode: 0xc6,
		Request:       buf.Bytes(),
	}
	if err := h.RawCommand(cbd); err != nil {
		return err
	}

	return nil
}

type IPodUpdateKind uint8

var (
	IPodUpdateBootloader IPodUpdateKind = 1
	IPodUpdateFirmware   IPodUpdateKind = 0
)

func (h *Host) IPodUpdateStart(kind IPodUpdateKind, size uint32) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, struct {
		Subcommand uint8
		Kind       uint8
		Size       uint32
	}{IPodSubcommandUpdateStart, uint8(kind), size})
	cbd := &CommandDataBuffer{
		OperationCode: 0xc6,
		Request:       buf.Bytes(),
	}
	if err := h.RawCommand(cbd); err != nil {
		return err
	}

	return nil
}

func (h *Host) IPodUpdateEnd() error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, struct {
		Subcommand uint8
	}{IPodSubcommandUpdateEnd})
	cbd := &CommandDataBuffer{
		OperationCode: 0xc6,
		Request:       buf.Bytes(),
	}
	if err := h.RawCommand(cbd); err != nil {
		return err
	}

	return nil
}

func (h *Host) IPodUpdateSendChunk(data []byte) error {
	if len(data)%4096 != 0 {
		return fmt.Errorf("data chunk size must be aligned to 12 bits")
	}
	nsectors := uint16(len(data) >> 12)
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, struct {
		Subcommand uint8
		NSectors   uint16
	}{IPodSubcommandUpdateChunk, nsectors})
	cbd := &CommandDataBuffer{
		OperationCode:         0xc6,
		Request:               buf.Bytes(),
		Data:                  data,
		DataTransferDirection: DataTransferToDevice,
	}
	if err := h.RawCommand(cbd); err != nil {
		return err
	}

	return nil
}

func (h *Host) IPodUpdateSendFull(kind IPodUpdateKind, data []byte) error {
	origlen := len(data)
	if len(data)%4096 != 0 {
		padding := bytes.Repeat([]byte{0}, 4096-(len(data)%4096))
		data = append(data, padding...)
	}
	if err := h.IPodUpdateStart(kind, uint32(origlen)); err != nil {
		return fmt.Errorf("starting failed: %w", err)
	}
	csize := 4096 * 8
	for i := 0; i < len(data); i += csize {
		pcnt := (i * 100) / len(data)
		fmt.Printf("%d%%...\r", pcnt)
		chunk := data[i:]
		if len(chunk) > csize {
			chunk = chunk[:csize]
		}
		if err := h.IPodUpdateSendChunk(chunk); err != nil {
			return fmt.Errorf("sending chunk %x failed: %w", i, err)
		}
	}
	if err := h.IPodUpdateEnd(); err != nil {
		return fmt.Errorf("ending failed: %w", err)
	}

	return nil
}

func (h *Host) IPodFinalize(reset bool) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, struct {
		Subcommand uint8
	}{IPodSubcommandUpdateFinalize})
	cbd := &CommandDataBuffer{
		OperationCode: 0xc6,
		Request:       buf.Bytes(),
	}
	if err := h.RawCommand(cbd); err != nil {
		return err
	}

	if reset {
		cbd = &CommandDataBuffer{
			OperationCode: 0x1e,
			Request: []byte{0, 0, 0, 0},
		}
		if err := h.RawCommand(cbd); err != nil {
			return err
		}

		cbd = &CommandDataBuffer{
			OperationCode: 0x1b,
			Request:       []byte{0, 0, 0, 2},
		}
		if err := h.RawCommand(cbd); err != nil {
			return err
		}
	}

	return nil
}
