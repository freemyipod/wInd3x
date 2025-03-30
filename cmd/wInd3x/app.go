package main

import (
	"context"
	"fmt"
	"time"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
)

type desktopApp struct {
	ctx *gousb.Context
	app.App
}

type desktopUsb struct {
	usb  *gousb.Device
	done func()
}

func (d *desktopUsb) UseDefaultInterface() error {
	_, done, err := d.usb.DefaultInterface()
	if err != nil {
		return err
	}
	d.done = done
	return nil
}

func (d *desktopUsb) UseDiskInterface() (devices.UsbMsEndpoints, error) {
	out := devices.UsbMsEndpoints{}

	if err := d.usb.SetAutoDetach(true); err != nil {
		return out, err
	}
	cfgNum, err := d.usb.ActiveConfigNum()
	if err != nil {
		return out, err
	}
	cfg, err := d.usb.Config(cfgNum)
	if err != nil {
		return out, err
	}
	i, err := cfg.Interface(0, 0)
	if err != nil {
		return out, err
	}
	eps := d.usb.Desc.Configs[cfg.Desc.Number].Interfaces[0].AltSettings[0].Endpoints
	for _, ep := range eps {
		var err error
		switch ep.Direction {
		case gousb.EndpointDirectionIn:
			out.In, err = i.InEndpoint(ep.Number)
		case gousb.EndpointDirectionOut:
			out.Out, err = i.OutEndpoint(ep.Number)
		}
		if err != nil {
			return out, err
		}
	}

	if out.In == nil || out.Out == nil {
		return out, fmt.Errorf("did not find both IN and OUT endpoint on mass storage interface")
	}

	return out, nil
}

func (d *desktopUsb) Control(rType, request uint8, val, idx uint16, data []byte) (int, error) {
	v, err := d.usb.Control(rType, request, val, idx, data)
	if err == gousb.ErrorTimeout {
		err = devices.UsbTimeoutError
	}
	return v, err
}

func (d *desktopUsb) SetControlTimeout(dur time.Duration) error {
	d.usb.ControlTimeout = dur
	return nil
}

func (d *desktopUsb) GetStringDescriptor(descIndex int) (string, error) {
	return d.usb.GetStringDescriptor(descIndex)
}

func (d *desktopUsb) Close() error {
	if d.done != nil {
		d.done()
		d.done = nil
	}
	return d.usb.Close()
}

func (d *desktopApp) Close() error {
	if err := d.Usb.Close(); err != nil {
		return fmt.Errorf("when closing USB device: %w", err)
	}
	if err := d.ctx.Close(); err != nil {
		return fmt.Errorf("when closing context: %w", err)
	}
	return nil
}

func newDFU() (*desktopApp, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		usb, err := ctx.OpenDeviceWithVIDPID(gousb.ID(deviceDesc.VID), gousb.ID(deviceDesc.PIDs[devices.DFU]))
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usb == nil {
			continue
		}

		app := &desktopApp{
			ctx,
			app.App{
				Usb:           &desktopUsb{usb: usb},
				InterfaceKind: devices.DFU,
				Desc:          &deviceDesc,
				Ep:            exploit.ParametersForKind[deviceDesc.Kind],
			},
		}
		return app, app.PrepareUSB()
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func newAny() (*desktopApp, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		for _, ik := range []devices.InterfaceKind{devices.DFU, devices.WTF, devices.Disk} {
			usb, err := ctx.OpenDeviceWithVIDPID(gousb.ID(deviceDesc.VID), gousb.ID(deviceDesc.PIDs[ik]))
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			if usb == nil {
				continue
			}

			app := &desktopApp{
				ctx,
				app.App{
					Usb:           &desktopUsb{usb: usb},
					InterfaceKind: ik,
					Desc:          &deviceDesc,
					Ep:            exploit.ParametersForKind[deviceDesc.Kind],
				},
			}
			return app, app.PrepareUSB()
		}
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func (a *desktopApp) waitSwitch(ctx context.Context, ik devices.InterfaceKind) error {
	for {
		usb, err := a.ctx.OpenDeviceWithVIDPID(gousb.ID(a.Desc.VID), gousb.ID(a.Desc.PIDs[ik]))
		if err != nil {
			return err
		}
		if usb != nil {
			a.Usb.Close()
			a.InterfaceKind = ik
			a.Usb = &desktopUsb{usb: usb}
			return a.PrepareUSB()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func newContext() (*gousb.Context, error) {
	resC := make(chan *gousb.Context)
	errC := make(chan error)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errC <- fmt.Errorf("%v", r)
			}
		}()

		resC <- gousb.NewContext()
	}()

	select {
	case err := <-errC:
		return nil, err
	case res := <-resC:
		return res, nil
	}
}
