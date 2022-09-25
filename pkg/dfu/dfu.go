package dfu

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"time"

	"github.com/golang/glog"
	"github.com/google/gousb"
)

type Request uint8

const (
	RequestDetach    Request = 0
	RequestDnload    Request = 1
	RequestUpload    Request = 2
	RequestGetStatus Request = 3
	RequestClrStatus Request = 4
	RequestGetState  Request = 5
	RequestAbort     Request = 6
)

type Err uint8

const (
	ErrOk          Err = 0x00
	ErrTarget      Err = 0x01
	ErrFile        Err = 0x02
	ErrWrite       Err = 0x03
	ErrErase       Err = 0x04
	ErrCheckErased Err = 0x05
	ErrProg        Err = 0x06
	ErrVerify      Err = 0x07
	ErrAddress     Err = 0x08
	ErrNotDone     Err = 0x09
	ErrFirmware    Err = 0x0a
	ErrVendor      Err = 0x0b
	ErrUsbr        Err = 0x0c
	ErrPor         Err = 0x0d
	ErrUnknown     Err = 0x0e
	ErrStalledPkt  Err = 0x0f
)

type State uint8

const (
	StateAppIdle           State = 0
	StateAppDetach         State = 1
	StateIdle              State = 2
	StateDnloadSync        State = 3
	StateDnBusy            State = 4
	StateDnloadIdle        State = 5
	StateManifestSync      State = 6
	StateManifest          State = 7
	StateManifestWaitReset State = 8
	StateUploadIdle        State = 9
	StateError             State = 10
)

func (d State) String() string {
	switch d {
	case StateAppIdle:
		return "appIDLE"
	case StateAppDetach:
		return "appDETACH"
	case StateIdle:
		return "dfuIDLE"
	case StateDnBusy:
		return "dfuDNBUSY"
	case StateDnloadIdle:
		return "dfuDNLOAD-IDLE"
	case StateManifestSync:
		return "dfuMANIFEST-SYNC"
	case StateManifest:
		return "dfuMANIFEST"
	case StateManifestWaitReset:
		return "dfuMANIFEST-WAIT-RESET"
	case StateUploadIdle:
		return "dfuUPLOAD-IDLE"
	case StateError:
		return "dfuERROR"
	}
	return "UNKNOWN"
}

func GetState(usb *gousb.Device) (State, error) {
	buf := make([]byte, 1)
	res, err := usb.Control(0xa1, uint8(RequestGetState), 0, 0, buf)
	if err != nil {
		return StateError, fmt.Errorf("control: %w", err)
	}
	if res != 1 {
		return StateError, fmt.Errorf("state returned %d bytes", res)
	}
	return State(uint8(buf[0])), nil
}

type Status struct {
	Err     Err
	State   State
	Timeout time.Duration
}

func GetStatus(usb *gousb.Device) (*Status, error) {
	buf := make([]byte, 6)
	res, err := usb.Control(0xa1, uint8(RequestGetStatus), 0, 0, buf)
	if err != nil {
		return nil, fmt.Errorf("control: %w", err)
	}
	if res != 6 {
		return nil, fmt.Errorf("status returned %d bytes", res)
	}

	timeoutMsec := (uint32(buf[2]) << 16) | (uint32(buf[1]) << 8) | uint32(buf[0])
	return &Status{
		Err:     Err(uint8(buf[0])),
		State:   State(uint8(buf[4])),
		Timeout: time.Duration(timeoutMsec) * time.Millisecond,
	}, nil
}

func ClearStatus(usb *gousb.Device) error {
	_, err := usb.Control(0x21, uint8(RequestClrStatus), 0, 0, nil)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	return nil
}

func SendChunk(usb *gousb.Device, c []byte, blockno uint16) error {
	_, err := usb.Control(0x21, uint8(RequestDnload), blockno, 0, c)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	return nil
}

type ProtoVersion int

const (
	// ProtoVersion1 is implemented by Nano3G.
	ProtoVersion1 ProtoVersion = 1
	// ProtoVersion2 is implemented by Nano4G+.
	ProtoVersion2 ProtoVersion = 2
)

func SendImage(usb *gousb.Device, i []byte, version ProtoVersion) error {
	if err := Clean(usb); err != nil {
		return fmt.Errorf("clean: %w", err)
	}

	if version == ProtoVersion1 {
		crc := bytes.NewBuffer(nil)
		binary.Write(crc, binary.LittleEndian, crc32.ChecksumIEEE(i))
		for _, b := range crc.Bytes() {
			i = append(i, b^0xff)
		}
	}

	buf := bytes.NewBuffer(i)
	blockno := uint16(0)
	for {
		chunk := make([]byte, 0x400)
		_, err := buf.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read failed: %w", err)
		}
		if err := SendChunk(usb, chunk, blockno); err != nil {
			return fmt.Errorf("chunk %d failed: %w", blockno, err)
		}
		status, err := GetStatus(usb)
		if err != nil {
			return fmt.Errorf("chunk %d status failed: %w", blockno, err)
		}
		if want, got := ErrOk, status.Err; want != got {
			return fmt.Errorf("chunk %d status expected %d, got %d", blockno, want, got)
		}
		blockno += 1

	}
	blockno += 1

	// Send zero-length download, completing image.
	if err := SendChunk(usb, nil, blockno); err != nil {
		return fmt.Errorf("zero length send failed: %w", err)
	}

	for i := 0; i < 100; i++ {
		// Send status request, causing manifest.
		st, err := GetStatus(usb)
		if err != nil {
			return fmt.Errorf("status failed: %w", err)
		}
		if st.State == StateIdle {
			return fmt.Errorf("unexpected idle, err: %d", st.Err)
		}
		if st.State == StateDnBusy {
			continue
		}
		if st.State == StateManifest {
			glog.Infof("Got dfuMANIFEST, image uploaded.")
			return nil
		}
	}

	return fmt.Errorf("did not reach manifest")
}

func Clean(usb *gousb.Device) error {
	if err := ClearStatus(usb); err != nil {
		return fmt.Errorf("ClrStatus: %w", err)
	}
	state, err := GetState(usb)
	if err != nil {
		return fmt.Errorf("GetState: %w", err)
	}
	if state != StateIdle {
		return fmt.Errorf("unexpected DFU state %s", state)
	}
	return nil

}
