package cache

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/mse"
)

func TestRepackAll(t *testing.T) {
	for _, kind := range []devices.Kind{
		//devices.Nano3, nano3g has some bogus files
		devices.Nano4,
		devices.Nano5,
		devices.Nano6,
		devices.Nano7,
	} {
		log.Printf("Testing repack of %s", kind)
		a := app.App{
			Desc: &devices.Description{
				Kind: kind,
			},
		}
		fw, err := Get(&a, PayloadKindFirmwareUpstream)
		if err != nil {
			t.Errorf("%s: could not get firmware: %v", kind, err)
			continue
		}

		m, err := mse.Parse(bytes.NewReader(fw))
		if err != nil {
			t.Errorf("%s: could not parse firmware: %v", kind, err)
			continue
		}

		fw2, err := m.Serialize()
		if err != nil {
			t.Errorf("%s: could not serialize firmware: %v", kind, err)
			continue
		}
		os.WriteFile(fmt.Sprintf("/tmp/%s out.bin", kind.String()), fw2, 0600)

		if !bytes.Equal(fw, fw2) {
			t.Errorf("%s: diff in emitted file", kind)
		}
	}
}
