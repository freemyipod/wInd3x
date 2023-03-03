package mse

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/freemyipod/wInd3x/pkg/image"
)

func (m *MSE) FileByName(n string) *File {
	for _, fi := range m.Files {
		if fi.Header.Name.String() == n {
			return fi
		}
	}
	return nil
}

func (m *MSE) Hax() error {
	osos := m.FileByName("osos")
	disk := m.FileByName("disk")
	osos.Header.Name.Set("disk")
	disk.Header.Name.Set("osos")

	rsrc := m.FileByName("rsrc")
	fmt.Println(hex.Dump(rsrc.Data[:0x100]))
	img1, err := image.Read(bytes.NewReader(rsrc.Data))
	if err != nil {
		return err
	}

	rsrcNew := make([]byte, len(rsrc.Data))
	copy(rsrcNew[:], rsrc.Data)

	body := img1.Body
	os.WriteFile("/tmp/rsrc.bin", body, 0600)
	body, _ = os.ReadFile("/home/q3k/Projects/freemyipod/nano7g/rsrc.bin")

	copy(rsrcNew[0x400:0x400+len(body)], body)

	rsrc.Data = rsrcNew

	return nil
}
