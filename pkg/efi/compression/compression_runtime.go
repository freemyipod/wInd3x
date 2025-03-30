//go:build wasm

package compression

type RuntimeDispatched struct {
	DecompressFn func(in []byte) ([]byte, error)
	CompressFn   func(in []byte) ([]byte, error)
}

func (r *RuntimeDispatched) Decompress(in []byte) ([]byte, error) {
	return r.DecompressFn(in)
}
func (r *RuntimeDispatched) Compress(in []byte) ([]byte, error) {
	return r.CompressFn(in)
}

var Compression = &RuntimeDispatched{}
