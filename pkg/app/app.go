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
	ctx  *gousb.Context
	Usb  *gousb.Device
	Desc *devices.Description
	Ep   exploit.Parameters
}

func (a *App) Close() {
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

func New() (*App, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize USB: %w", err)
	}

	var errs error
	for _, deviceDesc := range devices.Descriptions {
		usb, err := ctx.OpenDeviceWithVIDPID(deviceDesc.VID, deviceDesc.DFUPID)
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		if usb == nil {
			continue
		}

		return &App{
			ctx:  ctx,
			Usb:  usb,
			Desc: &deviceDesc,
			Ep:   exploit.ParametersForKind[deviceDesc.Kind],
		}, nil
	}
	if errs == nil {
		return nil, fmt.Errorf("no device found")
	}
	return nil, errs
}

func (a *App) WaitWTF(ctx context.Context) error {
	for {
		usb, err := a.ctx.OpenDeviceWithVIDPID(a.Desc.VID, a.Desc.WTFPID)
		if err != nil {
			return err
		}
		if usb != nil {
			a.Usb.Close()
			a.Usb = usb
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
