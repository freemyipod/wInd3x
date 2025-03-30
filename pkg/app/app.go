package app

import (
	"fmt"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/exploit"
)

type App struct {
	Usb           devices.Usb
	InterfaceKind devices.InterfaceKind
	Desc          *devices.Description
	Ep            exploit.Parameters

	MSEndpoints devices.UsbMsEndpoints
}

func (a *App) Close() error {
	if err := a.Usb.Close(); err != nil {
		return fmt.Errorf("when closing usb: %w", err)
	}
	return nil
}

// PrepareUSB sets up the correct interface based on the selected interface kind
// and runs any exploit preparation code (for eg. s5late compat).
func (a *App) PrepareUSB() error {
	switch a.InterfaceKind {
	case devices.DFU, devices.WTF:
		if err := a.Usb.UseDefaultInterface(); err != nil {
			return fmt.Errorf("UseDefaultInterface: %w", err)
		}
	case devices.Disk:
		ep, err := a.Usb.UseDiskInterface()
		if err != nil {
			return fmt.Errorf("UseDiskInterface: %w", err)
		}
		a.MSEndpoints = ep
	}

	if a.InterfaceKind == devices.DFU {
		if err := a.Ep.Prepare(a.Usb); err != nil {
			return fmt.Errorf("failed to prepare exploit: %w", err)
		}
	}
	return nil
}
