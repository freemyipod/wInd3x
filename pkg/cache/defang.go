package cache

import (
	"bytes"
	"fmt"

	"github.com/freemyipod/wInd3x/pkg/cfw"
	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/efi"
	"github.com/freemyipod/wInd3x/pkg/image"
	"github.com/golang/glog"
)

// defanger takes a decrypted WTF and returns it with security checks disabled.
type defanger func(decrypted []byte) ([]byte, error)

var wtfDefangers = map[devices.Kind]defanger{
	devices.Nano3: func(decrypted []byte) ([]byte, error) {
		res := make([]byte, len(decrypted))
		copy(res, decrypted)

		// Change USB device string.
		source := []byte("iPod Recovery")
		target := []byte("Defanged WTF!")
		if len(source) != len(target) {
			panic("invalid source/target length")
		}

		// Janky little wide char replace.
		start := 0x800 + 0x770c
		for i, b := range target {
			addr := start + i*2
			if want, got := source[i], res[addr]; want != got {
				return nil, fmt.Errorf("source data mismatch at %x, wanted %v, got %v", addr, want, got)
			}
			res[addr] = b
		}

		// Disable sigchecking.
		if res[0x800+0x19ac] != 0x33 {
			panic("invalid patch offs")
		}
		res[0x800+0x19ac] = 0x00
		res[0x800+0x19ad] = 0x30
		res[0x800+0x19ae] = 0xa0
		res[0x800+0x19af] = 0xe3
		return res, nil
	},
	devices.Nano5: defangEFI(cfw.MultipleVisitors([]cfw.VolumeVisitor{
		// Change USB vendor string in RestoreDFU.efi.
		&cfw.VisitPE32InFile{
			FileGUID: efi.MustParseGUID("a0517d80-37fa-4d06-bd0e-941d5698846a"),
			Patch: cfw.Patches([]cfw.Patch{
				cfw.ReplaceExact{From: []byte("Apple Inc."), To: []byte("freemyipod")},
			}),
		},
		// Disable signature checking in ROMBootValidator.efi.
		&cfw.VisitPE32InFile{
			FileGUID: efi.MustParseGUID("1ba058e3-2063-4919-8002-6d2e0c947e60"),
			Patch: cfw.Patches([]cfw.Patch{
				// CheckHeaderSignatureImpl -> return 0
				cfw.PatchAt{
					Address: 0x15b8,
					To: []byte{
						0x00, 0x20,
						0x70, 0x47,
					},
				},
				// CheckDataSignature -> return 1
				cfw.PatchAt{
					Address: 0x0b4c,
					To: []byte{
						0x01, 0x20,
						0x70, 0x47,
					},
				},
			}),
		},
	})),
}

func defangEFI(visitor cfw.VolumeVisitor) defanger {
	return func(decrypted []byte) ([]byte, error) {
		img, err := image.Read(bytes.NewReader(decrypted))
		if err != nil {
			return nil, err
		}

		defanged, err := ApplyPatches(img, visitor)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patches: %w", err)
		}
		return defanged, nil
	}
}

func ApplyPatches(img *image.IMG1, patches cfw.VolumeVisitor) ([]byte, error) {
	nr := efi.NewNestedReader(img.Body[0x100:])
	fv, err := efi.ReadVolume(nr)
	if err != nil {
		return nil, fmt.Errorf("failed to read firmware volume: %w", err)
	}

	origSize, err := cfw.SecoreOffset(fv)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate original secore offset: %w", err)
	}
	glog.Infof("Initial pre-padding size: %d", origSize)

	glog.Infof("Applying patches...")
	if err := cfw.VisitVolume(fv, &cfw.N5GWTF); err != nil {
		return nil, fmt.Errorf("failed to apply patches: %w", err)
	}

	glog.Infof("Fixing up padding...")
	if err := cfw.SecoreFixup(origSize, fv); err != nil {
		return nil, fmt.Errorf("failed to fix up size: %w", err)
	}
	glog.Infof("Done.")

	fvb, err := fv.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild firmware: %w", err)
	}

	fvb = append(img.Body[:0x100], fvb...)
	imb, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, fvb)
	if err != nil {
		return nil, fmt.Errorf("failed to build new image1: %w", err)
	}

	return imb, nil
}
