package cache

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/mse"
)

func TestRepackAll(t *testing.T) {
	for _, kind := range []devices.Kind{
		devices.Nano3,
		// Don't diff test N4G as padding in firmware file contains repeat data
		// from previous blocks - someone forgot a memset :).
		//devices.Nano4,
		devices.Nano5,
		devices.Nano6,
		devices.Nano7,
	} {
		t.Run(fmt.Sprintf("%s", kind.String()), func(t *testing.T) {
			a := app.App{
				Desc: &devices.Description{
					Kind: kind,
				},
			}
			fw, err := Get(&a, PayloadKindFirmwareUpstream)
			if err != nil {
				t.Fatalf("%s: could not get firmware: %v", kind, err)
			}

			m, err := mse.Parse(bytes.NewReader(fw))
			if err != nil {
				t.Fatalf("%s: could not parse firmware: %v", kind, err)
			}
			for _, file := range m.Files {
				if !file.Header.Valid() {
					continue
				}
			}

			fw2, err := m.Serialize()
			if err != nil {
				t.Fatalf("%s: could not serialize firmware: %v", kind, err)
			}
			os.WriteFile(fmt.Sprintf("/tmp/%s out.bin", kind.String()), fw2, 0600)

			if !bytes.Equal(fw, fw2) {
				t.Fatalf("%s: diff in emitted file", kind)
			}
		})
	}
}
