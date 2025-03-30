package compression

type TianoCompression interface {
	// Decompress using Tiano compression algorithm from EDK2.
	Decompress(in []byte) ([]byte, error)
	// Compress using Tiano compression algorithm from EDK2.
	Compress(in []byte) ([]byte, error)
}
