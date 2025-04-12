// wiwali is a non-WASI (web-compatible) wasm 'binary' that sends window.wiwali
// when loaded. This object carries all references to runtime object that Wiwali
// needs. Garbage collection is currently unimplemented.
//
// See web/go.ts for actual bindings.
package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"syscall/js"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/cache"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/dfu"
	"github.com/freemyipod/wInd3x/pkg/efi/compression"
	"github.com/freemyipod/wInd3x/pkg/exploit"
	"github.com/freemyipod/wInd3x/pkg/exploit/dumpmem"
	"github.com/freemyipod/wInd3x/pkg/exploit/haxeddfu"
)

// async wraps a Go function into a ES/TS function that returns a Promise.
func async(f func(this js.Value, args []js.Value) (js.Value, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, pargs []js.Value) any {
			resolve := pargs[0]
			reject := pargs[1]
			go func() {
				v, err := f(this, args)
				if err != nil {
					reject.Invoke(js.Global().Get("Error").New(err.Error()))
				} else {
					resolve.Invoke(v)
				}
			}()
			return js.Null()
		}))
	})
}

// await calls a ES/TS async function and blocks until it resolves or fails.
func await(v js.Value) (js.Value, error) {
	okC := make(chan js.Value)
	errC := make(chan error)
	v.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		okC <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		errC <- fmt.Errorf(args[0].Get("message").String())
		return nil
	}))

	select {
	case ok := <-okC:
		return ok, nil
	case err := <-errC:
		return js.ValueOf(nil), err
	}
}

// findDeviceAndKind looks for a matching device given a vid and pid.
func findDeviceAndKind(vid, pid int16) (*devices.Description, devices.InterfaceKind) {
	for _, desc := range devices.Descriptions {
		if desc.VID != vid {
			continue
		}
		for kind, dpid := range desc.PIDs {
			if dpid == pid {
				return &desc, kind
			}
		}
	}
	return nil, ""
}

func toUint8Array(b []byte) js.Value {
	arr := make([]any, len(b))
	for i, v := range b {
		arr[i] = v
	}
	return js.Global().Get("Uint8Array").New(arr)
}

func fromUint8Array(v js.Value) ([]byte, error) {
	dataView := js.Global().Get("DataView").New(v)
	bl := dataView.Get("byteLength").Int()
	res := make([]byte, bl)
	for i := 0; i < bl; i++ {
		v := dataView.Call("getUint8", i).Int()
		res[i] = byte(v)
	}
	return res, nil
}

// newApp builds a one-shot (ie. one-vid/pid) wInd3x representation of the device.
func newApp(this js.Value, args []js.Value) (js.Value, error) {
	if len(args) != 1 {
		return js.Null(), fmt.Errorf("newApp must be called with exactly one argument")
	}

	usbDevice := args[0]
	vid := int16(usbDevice.Get("vendorId").Int())
	pid := int16(usbDevice.Get("productId").Int())
	dev, kind := findDeviceAndKind(vid, pid)
	if dev == nil {
		return js.Null(), fmt.Errorf("unknown kind of device")
	}

	a := app.App{
		Usb: &usb{
			usbDevice: usbDevice,
		},
		InterfaceKind: kind,
		Desc:          dev,
		Ep:            exploit.ParametersForKind[dev.Kind],
	}
	return js.ValueOf(map[string]any{
		"GetStringDescriptors": async(func(this js.Value, args []js.Value) (js.Value, error) {
			// HACK, we should be doing this better (by using device descriptor
			// indices into strings descriptors).
			var descs []string
			for i := 0; i < 3; i++ {
				desc, err := a.Usb.GetStringDescriptor(i)
				if err != nil {
					return js.Null(), fmt.Errorf("getting descriptor %d: %w", i, err)
				}
				descs = append(descs, desc)
			}
			return js.ValueOf(map[string]any{
				"manufacturer": descs[1],
				"product":      descs[2],
			}), nil
		}),
		"GetDeviceDescription": async(func(this js.Value, args []js.Value) (js.Value, error) {
			return js.ValueOf(map[string]any{
				"vid":             vid,
				"pid":             pid,
				"updaterFamilyID": dev.UpdaterFamilyID,
				"kind":            string(dev.Kind),
				"interfaceKind":   string(kind),
			}), nil
		}),
		"PrepareUSB": async(func(this js.Value, args []js.Value) (js.Value, error) {
			err := a.PrepareUSB()
			return js.Null(), err
		}),
		"DumpMem": async(func(this js.Value, args []js.Value) (js.Value, error) {
			if len(args) != 1 {
				return js.Null(), fmt.Errorf("must be called with exactly one argument")
			}
			addrStr := js.Global().Get("BigInt").Get("prototype").Get("toString").Call("call", args[0]).String()
			addr, err := strconv.ParseUint(addrStr, 10, 32)
			if err != nil {
				return js.Null(), fmt.Errorf("given number is not a valid 32-bit BigInt")
			}
			block, err := dumpmem.Trigger(a.Usb, a.Ep, uint32(addr))
			if err != nil {
				return js.Null(), err
			}
			return toUint8Array(block), nil
		}),
		"HaxDFU": async(func(this js.Value, args []js.Value) (js.Value, error) {
			err := haxeddfu.Trigger(a.Usb, a.Ep, false)
			return js.Null(), err
		}),
		"SendPayload": async(func(this js.Value, args []js.Value) (js.Value, error) {
			kind := cache.PayloadKind(args[0].String())

			slog.Info("Getting payload...")
			pl, err := cache.Get(&a, kind)
			if err != nil {
				return js.Null(), fmt.Errorf("getting payload %q failed: %w", kind, err)
			}
			slog.Info("Sending payload...")
			if err := dfu.SendImage(a.Usb, pl, a.Desc.Kind.DFUVersion()); err != nil {
				return js.Null(), fmt.Errorf("failed to send image: %w", err)
			}
			return js.Null(), nil
		}),
	}), nil
}

// setup convinces the rest of the wInd3x codebase to work by poking a bunch of
// global variables.
func setup(this js.Value, args []js.Value) (js.Value, error) {
	compress := args[0].Get("compress")
	decompress := args[0].Get("decompress")
	compression.Compression.CompressFn = func(in []byte) ([]byte, error) {
		return fromUint8Array(compress.Invoke(toUint8Array(in)))
	}
	compression.Compression.DecompressFn = func(in []byte) ([]byte, error) {
		return fromUint8Array(decompress.Invoke(toUint8Array(in)))
	}

	store, err := newIndexedDBFS()
	if err != nil {
		return js.Null(), fmt.Errorf("setting up indexeddb: %w", err)
	}
	cache.Store = store

	return js.Null(), nil
}

func main() {
	js.Global().Set("wiwali", js.ValueOf(map[string]any{
		"newApp": async(newApp),
		"setup":  async(setup),
	}))
	slog.Info("wiwali loaded.")
	select {}
}
