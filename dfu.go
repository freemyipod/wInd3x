package main

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

type dfuRequest uint8

const (
	dfuDetach    dfuRequest = 0
	dfuDnload    dfuRequest = 1
	dfuUpload    dfuRequest = 2
	dfuGetStatus dfuRequest = 3
	dfuClrStatus dfuRequest = 4
	dfuGetState  dfuRequest = 5
	dfuAbort     dfuRequest = 6
)

type dfuErr uint8

const (
	dfuOk             dfuErr = 0x00
	dfuErrTarget      dfuErr = 0x01
	dfuErrFile        dfuErr = 0x02
	dfuErrWrite       dfuErr = 0x03
	dfuErrErase       dfuErr = 0x04
	dfuErrCheckErased dfuErr = 0x05
	dfuErrProg        dfuErr = 0x06
	dfuErrVerify      dfuErr = 0x07
	dfuErrAddress     dfuErr = 0x08
	dfuErrNotDone     dfuErr = 0x09
	dfuErrFirmware    dfuErr = 0x0a
	dfuErrVendor      dfuErr = 0x0b
	dfuErrUsbr        dfuErr = 0x0c
	dfuErrPor         dfuErr = 0x0d
	dfuErrUnknown     dfuErr = 0x0e
	dfuErrStalledPkt  dfuErr = 0x0f
)

type dfuState uint8

const (
	appIdle              dfuState = 0
	appDetach            dfuState = 1
	dfuIdle              dfuState = 2
	dfuDnloadSync        dfuState = 3
	dfuDnBusy            dfuState = 4
	dfuDnloadIdle        dfuState = 5
	dfuManifestSync      dfuState = 6
	dfuManifest          dfuState = 7
	dfuManifestWaitReset dfuState = 8
	dfuUploadIdle        dfuState = 9
	dfuError             dfuState = 10
)

func (d dfuState) String() string {
	switch d {
	case appIdle:
		return "appIDLE"
	case appDetach:
		return "appDETACH"
	case dfuIdle:
		return "dfuIDLE"
	case dfuDnBusy:
		return "dfuDNBUSY"
	case dfuDnloadIdle:
		return "dfuDNLOAD-IDLE"
	case dfuManifestSync:
		return "dfuMANIFEST-SYNC"
	case dfuManifest:
		return "dfuMANIFEST"
	case dfuManifestWaitReset:
		return "dfuMANIFEST-WAIT-RESET"
	case dfuUploadIdle:
		return "dfuUPLOAD-IDLE"
	case dfuError:
		return "dfuERROR"
	}
	return "UNKNOWN"
}

func (d *device) getState() (dfuState, error) {
	buf := make([]byte, 1)
	res, err := d.usb.Control(0xa1, uint8(dfuGetState), 0, 0, buf)
	if err != nil {
		return dfuError, fmt.Errorf("control: %w", err)
	}
	if res != 1 {
		return dfuError, fmt.Errorf("state returned %d bytes", res)
	}
	return dfuState(uint8(buf[0])), nil
}

type dfuStatus struct {
	err     dfuErr
	timeout time.Duration
}

func (d *device) getStatus() (*dfuStatus, error) {
	buf := make([]byte, 6)
	res, err := d.usb.Control(0xa1, uint8(dfuGetStatus), 0, 0, buf)
	if err != nil {
		return nil, fmt.Errorf("control: %w", err)
	}
	if res != 6 {
		return nil, fmt.Errorf("status returned %d bytes", res)
	}

	timeoutMsec := (uint32(buf[2]) << 16) | (uint32(buf[1]) << 8) | uint32(buf[0])
	return &dfuStatus{
		err:     dfuErr(uint8(buf[0])),
		timeout: time.Duration(timeoutMsec) * time.Millisecond,
	}, nil
}

func (d *device) clearStatus() error {
	_, err := d.usb.Control(0x21, uint8(dfuClrStatus), 0, 0, nil)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	return nil
}

func (d *device) sendChunk(c []byte, blockno uint16) error {
	_, err := d.usb.Control(0x21, uint8(dfuDnload), blockno, 0, c)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	return nil
}

func (d *device) sendImage(i []byte) error {
	if err := d.clean(); err != nil {
		return fmt.Errorf("clean: %w", err)
	}

	buf := bytes.NewBuffer(i)
	blockno := uint16(0)
	for {
		chunk := make([]byte, 0x800)
		_, err := buf.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read failed: %w", err)
		}
		if err := d.sendChunk(chunk, blockno); err != nil {
			return fmt.Errorf("chunk %d failed: %w", blockno, err)
		}
		status, err := d.getStatus()
		if err != nil {
			return fmt.Errorf("chunk %d status failed: %w", blockno, err)
		}
		if want, got := dfuOk, status.err; want != got {
			return fmt.Errorf("chunk %d status expected %d, got %d", blockno, want, got)
		}
		//time.Sleep(status.timeout)
		blockno += 1
	}

	// Send zero-length download, completing image.
	if err := d.sendChunk(nil, blockno); err != nil {
		return fmt.Errorf("zero length send failed: %w", err)
	}
	// Send status request, causing manifest.
	st, err := d.getStatus()
	if err != nil {
		return fmt.Errorf("status failed: %w", err)
	}
	if st.err != dfuOk {
		return fmt.Errorf("status reported unexpected %d", st.err)
	}

	return nil
}
