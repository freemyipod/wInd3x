package compression

import (
	"bytes"
	"testing"
)

func TestLoopback(t *testing.T) {
	input := []byte("According to all known laws of aviation, there is no way an EFI implementation should be able to fly. It's wings are too small to get its fat little body off the ground. The implementation, of course, flies anyway, because computers don't care what humans think is impossible.")
	compressed, err := Compress(input)
	if err != nil {
		t.Fatalf("Compress() failed: %v", err)
	}

	uncompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress() failed: %v", err)
	}

	if !bytes.Equal(input, uncompressed) {
		t.Fatalf("did not decompress to same data: %q", string(uncompressed))
	}
}
