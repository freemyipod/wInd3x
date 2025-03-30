//go:build !wasm
// +build !wasm

// package compression implements EFI compression/decompression routines by
// calling out into edk2 Tiano{Dec,C}ompres functions compiled into
// WebAssembly.
//
// We don't use cgo or c2go because I don't trust that code.
//
// See build.sh on how to regenerate edk2.wasm.
package compression

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

var (
	// mu guards the entirety of the compression/decompression library behind a
	// singleton mutex. That's because the Intel compression/decompression code
	// is very, very, extremely non memory safe.
	mu sync.Mutex

	//go:embed edk2.wasm
	wasm []byte
)

// edk2 is the WebAssembly module and loaded functions from edk2.wasm.
type edk2 struct {
	module api.Module

	mallocF     api.Function
	freeF       api.Function
	compressF   api.Function
	decompressF api.Function
}

func (e *edk2) malloc(ctx context.Context, size int) uint32 {
	results, err := e.mallocF.Call(ctx, uint64(size))
	if err != nil {
		panic(fmt.Sprintf("wasm malloc() failed: %v", err))
	}
	return uint32(results[0])
}

func (e *edk2) free(ctx context.Context, ptr uint32) {
	e.freeF.Call(ctx, uint64(ptr))
}

func (e *edk2) write(ctx context.Context, ptr uint32, data []byte) {
	if !e.module.Memory().Write(ctx, ptr, data) {
		panic("memory write failed")
	}
}

func (e *edk2) writeu32(ctx context.Context, ptr, data uint32) {
	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, data); err != nil {
		panic("err")
	}
	e.write(ctx, ptr, buf.Bytes())
}

func (e *edk2) read(ctx context.Context, ptr uint32, size int) []byte {
	res, ok := e.module.Memory().Read(ctx, ptr, uint32(size))
	if !ok {
		panic("memory read failed")
	}
	res2 := make([]byte, len(res))
	copy(res2, res)
	return res2
}

func (e *edk2) readu32(ctx context.Context, ptr uint32) uint32 {
	data := e.read(ctx, ptr, 4)
	var res uint32
	if err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &res); err != nil {
		panic(err)
	}
	return res
}

var (
	edk *edk2
)

func edk2Error(code int32) error {
	switch code {
	case 0:
		return nil
	case 2:
		return errors.New("invalid parameter")
	case 5:
		return errors.New("buffer too small")
	case 9:
		return errors.New("out of resources")
	default:
		return fmt.Errorf("unknown (%d)", code)
	}
}

func getedk2() *edk2 {
	// Already guarded by 'mu'.
	if edk != nil {
		return edk
	}

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		panic(err)
	}
	if _, err := emscripten.Instantiate(ctx, r); err != nil {
		panic(err)
	}

	config := wazero.NewModuleConfig().WithStdout(os.Stdout).WithStderr(os.Stderr)
	code, err := r.CompileModule(ctx, wasm, wazero.NewCompileConfig())
	if err != nil {
		panic(err)
	}
	mod, err := r.InstantiateModule(ctx, code, config)
	if err != nil {
		panic(err)
	}

	e := &edk2{
		module:      mod,
		mallocF:     mod.ExportedFunction("malloc"),
		freeF:       mod.ExportedFunction("free"),
		compressF:   mod.ExportedFunction("TianoCompress"),
		decompressF: mod.ExportedFunction("TianoDecompress"),
	}

	edk = e
	return e
}

type wazeroCompression struct{}

// Decompress using Tiano compression algorithm from EDK2.
func (w *wazeroCompression) Decompress(in []byte) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	var dstSize uint32
	if err := binary.Read(bytes.NewBuffer(in[4:8]), binary.LittleEndian, &dstSize); err != nil {
		return nil, err
	}

	ctx, ctxC := context.WithCancel(context.Background())
	defer ctxC()

	e := getedk2()

	// Prepare `in` in wasm.
	inPtr := e.malloc(ctx, len(in))
	defer e.free(ctx, inPtr)
	e.write(ctx, inPtr, in)

	// Prepare `out` in wasm.
	outPtr := e.malloc(ctx, int(dstSize))
	defer e.free(ctx, outPtr)

	// Prepare `scratch` in wasm.
	scratchPtr := e.malloc(ctx, 13393)
	defer e.free(ctx, outPtr)

	results, err := e.decompressF.Call(ctx, uint64(inPtr), uint64(len(in)), uint64(outPtr), uint64(dstSize), uint64(scratchPtr), 13393)
	if err != nil {
		return nil, fmt.Errorf("wasm TianoDecompress() failed: %w", err)
	}

	res := int32(results[0])
	if res != 0 {
		return nil, edk2Error(res)
	}

	data := e.read(ctx, outPtr, int(dstSize))
	return data, nil
}

// Compress using Tiano compression algorithm from EDK2.
func (w *wazeroCompression) Compress(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("canot compress empty file")
	}

	mu.Lock()
	defer mu.Unlock()

	ctx, ctxC := context.WithCancel(context.Background())
	defer ctxC()

	e := getedk2()

	// Prepare `in` in wasm.
	inPtr := e.malloc(ctx, len(in))
	defer e.free(ctx, inPtr)
	e.write(ctx, inPtr, in)

	// Prepare `out` in wasm.
	outPtr := e.malloc(ctx, len(in))
	defer e.free(ctx, outPtr)

	// Prepare `outSize` in wasm.
	outSizePtr := e.malloc(ctx, 4)
	defer e.free(ctx, outSizePtr)
	e.writeu32(ctx, outSizePtr, uint32(len(in)))

	results, err := e.compressF.Call(ctx, uint64(inPtr), uint64(len(in)), uint64(outPtr), uint64(outSizePtr))
	if err != nil {
		return nil, fmt.Errorf("wasm TianoCompress() failed: %w", err)
	}

	res := int32(results[0])
	if res != 0 {
		return nil, edk2Error(res)
	}

	outSizeU32 := e.readu32(ctx, outSizePtr)
	return e.read(ctx, outPtr, int(outSizeU32)), nil
}

var Compression TianoCompression = &wazeroCompression{}
