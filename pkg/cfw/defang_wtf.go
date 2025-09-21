package cfw

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/efi"
	"github.com/freemyipod/wInd3x/pkg/image"
)

// defanger takes a decrypted WTF and returns it with security checks disabled.
type Defanger func(decrypted []byte) ([]byte, error)

var WTFDefangers = map[devices.Kind]Defanger{
	devices.Nano3: defangRaw(map[int][]byte{
		// replace bytes at addresses in the WTF binary:
		0x1990: []byte{0x00, 0x70, 0xA0, 0xE3, 0x22, 0x00, 0x00, 0xEA},                      // skip signature check
		0x770C: []byte("D\x00e\x00f\x00a\x00n\x00g\x00e\x00d\x00 \x00W\x00T\x00F\x00!\x00"), // change USB product string to show it's defanged
	}),
	devices.Nano5: defangEFI(MultipleVisitors([]VolumeVisitor{
		// Change USB vendor string in RestoreDFU.efi.
		&VisitPE32InFile{
			FileGUID: efi.MustParseGUID("a0517d80-37fa-4d06-bd0e-941d5698846a"),
			Patch: Patches([]Patch{
				ReplaceExact{From: []byte("Apple Inc."), To: []byte("freemyipod")},
			}),
		},
		// Disable signature checking in ROMBootValidator.efi.
		&VisitPE32InFile{
			FileGUID: efi.MustParseGUID("1ba058e3-2063-4919-8002-6d2e0c947e60"),
			Patch: Patches([]Patch{
				// CheckHeaderSignatureImpl -> return 0
				PatchAt{
					Address: 0x15b8,
					To: []byte{
						0x00, 0x20,
						0x70, 0x47,
					},
				},
				// CheckDataSignature -> return 1
				PatchAt{
					Address: 0x0b4c,
					To: []byte{
						0x01, 0x20,
						0x70, 0x47,
					},
				},
			}),
		},
	})),
	devices.Nano7: defangEFI(MultipleVisitors([]VolumeVisitor{
		// Change USB vendor string in ARM/AppleMobilePkg/Dfu/Dfu/DEBUG/Dfu.dll.
		&VisitPE32InFile{
			FileGUID: efi.MustParseGUID("936ffb79-62f6-4fc0-aff0-3e2a1c56f1a7"),
			Patch: Patches([]Patch{
				ReplaceExact{From: []byte("Apple Inc."), To: []byte("freemyipod")},
			}),
		},
		// Disable signature checking in ARM/SamsungPkg/Chipset/S5L8720/ROMBootValidator/ROMBootValidator/DEBUG/ROMBootValidator.dll
		&VisitPE32InFile{
			FileGUID: efi.MustParseGUID("1ba058e3-2063-4919-8002-6d2e0c947e60"),
			Patch: Patches([]Patch{
				// CheckHeaderSignatureImpl -> return 0
				PatchAt{
					Address: 0x19a4,
					To: []byte{
						0x00, 0x20, // mov r0, #0
						0x70, 0x47, // bx lr
					},
				},
				// CheckDataSignature -> return 1
				PatchAt{
					Address: 0x0d78,
					To: []byte{
						0x01, 0x20, // mov r0, #1
						0x70, 0x47, // bx lr
					},
				},
				// Call AES for both type 4 and type 3 (we generate type 4,
				// while usually AES images are type 3, we turn AES decryption
				// into a no-op in Aes.dll below).
				PatchAt{
					Address: 0x176e,
					To: []byte{
						0x04, 0x28, // cmp r0, #4
					},
				},
			}),
		},
		// Replace AES decryption with no-op memcpy
		&VisitPE32InFile{
			FileGUID: efi.MustParseGUID("c0287dba-8a73-4ff1-98f1-455b97d4d480"),
			Patch: Patches([]Patch{
				// AESProtocol::Decrypt -> memcpy
				PatchAt{
					Address: 0x488,
					To: []byte{
						// _loop:
						0x08, 0x68, // ldr r0, [r1]
						0x04, 0x31, // add r1, r1, #4
						0x10, 0x60, // str r0, [r2]
						0x04, 0x32, // add r2, r2, #4
						0x04, 0x3b, // sub r3, r3, #4
						0x03, 0xb1, // cbz r3, _done
						0xf8, 0xe7, // b _loop
						// _done:
						0x70, 0x47, // bx lr
					},
				},
			}),
		},
	})),
}

func defangEFI(visitor VolumeVisitor) Defanger {
	return func(decrypted []byte) ([]byte, error) {
		img, err := image.Read(bytes.NewReader(decrypted))
		if err != nil {
			return nil, err
		}

		defanged, err := applyPatches(img, visitor)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patches: %w", err)
		}
		return defanged, nil
	}
}

func applyPatches(img *image.IMG1, patches VolumeVisitor) ([]byte, error) {
	offs := 0x100
	switch img.DeviceKind {
	case devices.Nano7:
		offs = 0
	}
	nr := efi.NewNestedReader(img.Body[offs:])
	fv, err := efi.ReadVolume(nr)
	if err != nil {
		return nil, fmt.Errorf("failed to read firmware volume: %w", err)
	}

	origSize, err := SecoreOffset(fv)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate original secore offset: %w", err)
	}
	slog.Info("Initial pre-padding", "size", origSize)

	slog.Info("Applying patches...")
	if err := VisitVolume(fv, patches); err != nil {
		return nil, fmt.Errorf("failed to apply patches: %w", err)
	}

	slog.Info("Fixing up padding...")
	if err := SecoreFixup(origSize, fv); err != nil {
		return nil, fmt.Errorf("failed to fix up size: %w", err)
	}
	slog.Info("Done.")

	fvb, err := fv.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild firmware: %w", err)
	}

	fvb = append(img.Body[:offs], fvb...)
	imb, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, fvb)
	if err != nil {
		return nil, fmt.Errorf("failed to build new image1: %w", err)
	}

	return imb, nil
}

func defangRaw(patches map[int][]byte) Defanger {
	return func(decrypted []byte) ([]byte, error) {
		img, err := image.Read(bytes.NewReader(decrypted))
		if err != nil {
			return nil, fmt.Errorf("failed to read image: %w", err)
		}

		for offset, patch := range patches {
			if len(img.Body) < offset+len(patch) {
				return nil, fmt.Errorf("patch at offset %x is too large", offset)
			}
			copy(img.Body[offset:], patch)
		}

		defanged, err := image.MakeUnsigned(img.DeviceKind, img.Header.Entrypoint, img.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to build new image1: %w", err)
		}

		return defanged, nil
	}
}
