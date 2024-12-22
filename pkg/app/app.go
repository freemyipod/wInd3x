package app

import (
	"context"
	"fmt"
	"time"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/google/gousb"
	"github.com/hashicorp/go-multierror"
)

type App struct {
	ctx           *gousb.Context
	Usb           *gousb.Device
	InterfaceKind devices.InterfaceKind
	done          func()
	Desc          *devices.Description
	Ep            exploit.Parameters

	MSEndpoints struct {
		In  *gousb.InEndpoint
		Out *gousb.OutEndpoint
	}
}

func (a *App) Close() {
	if a.done != nil {
		a.done()
	}
	a.ctx.Close()
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

// TODO: rename to NewDFU.
func New() (*App, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		usb, err := ctx.OpenDeviceWithVIDPID(deviceDesc.VID, deviceDesc.PIDs[devices.DFU])
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usb == nil {
			continue
		}

		app := &App{
			ctx:           ctx,
			Usb:           usb,
			InterfaceKind: devices.DFU,
			Desc:          &deviceDesc,
			Ep:            exploit.ParametersForKind[deviceDesc.Kind],
		}
		return app, app.prepareUSB()
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func NewAny() (*App, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		for _, ik := range []devices.InterfaceKind{devices.DFU, devices.WTF, devices.Disk} {
			usb, err := ctx.OpenDeviceWithVIDPID(deviceDesc.VID, deviceDesc.PIDs[ik])
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			if usb == nil {
				continue
			}

			app := &App{
				ctx:           ctx,
				Usb:           usb,
				InterfaceKind: ik,
				Desc:          &deviceDesc,
				Ep:            exploit.ParametersForKind[deviceDesc.Kind],
			}
			return app, app.prepareUSB()
		}
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func (a *App) prepareUSB() error {
	if a.done != nil {
		a.done()
	}
	a.done = nil
	switch a.InterfaceKind {
	case devices.DFU, devices.WTF:
		_, done, err := a.Usb.DefaultInterface()
		if err != nil {
			return err
		}
		a.done = done
	case devices.Disk:
		if err := a.Usb.SetAutoDetach(true); err != nil {
			return err
		}
		cfgNum, err := a.Usb.ActiveConfigNum()
		if err != nil {
			return err
		}
		cfg, err := a.Usb.Config(cfgNum)
		if err != nil {
			return err
		}
		i, err := cfg.Interface(0, 0)
		if err != nil {
			return err
		}
		eps := a.Usb.Desc.Configs[cfg.Desc.Number].Interfaces[0].AltSettings[0].Endpoints
		for _, ep := range eps {
			var err error
			switch ep.Direction {
			case gousb.EndpointDirectionIn:
				a.MSEndpoints.In, err = i.InEndpoint(ep.Number)
			case gousb.EndpointDirectionOut:
				a.MSEndpoints.Out, err = i.OutEndpoint(ep.Number)
			}
			if err != nil {
				return err
			}
		}
	}

	if a.InterfaceKind == devices.DFU {
		if err := a.Ep.Prepare(a.Usb); err != nil {
			return fmt.Errorf("failed to prepare exploit: %w", err)
		}
	}
	return nil
}

func (a *App) WaitSwitch(ctx context.Context, ik devices.InterfaceKind) error {
	for {
		usb, err := a.ctx.OpenDeviceWithVIDPID(a.Desc.VID, a.Desc.PIDs[ik])
		if err != nil {
			return err
		}
		if usb != nil {
			a.Usb.Close()
			a.InterfaceKind = ik
			a.Usb = usb
			return a.prepareUSB()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// TODO: remove
func (a *App) WaitWTF(ctx context.Context) error {
	return a.WaitSwitch(ctx, devices.WTF)
}
