package cfw

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/freemyipod/wInd3x/pkg/efi"
)

// secoreOffset returns the offset of the security core TE within the firmware
// volume. The security core is located in the last file in the firmware, and
// must not be moved around when the firmware is rebuilt.
func SecoreOffset(fv *efi.Volume) (int, error) {
	if len(fv.Files) < 3 {
		return 0, fmt.Errorf("firmwware volume must have at least two files")
	}

	ipadding := len(fv.Files) - 2
	ite := len(fv.Files) - 1
	if fv.Files[ipadding].FileType != efi.FileTypePadding {
		return 0, fmt.Errorf("second to last file must be padding")
	}
	if fv.Files[ite].FileType != efi.FileTypeSecurityCore {
		return 0, fmt.Errorf("last file must be security core")
	}

	return fv.Files[ite].ReadOffset, nil
}

// secoreFixup attempts to mangle the given firmware volume to place the
// security core at origPos. This is currently done by modifying the padding
// file, which is the second-to-last file within the firmware volume.
func SecoreFixup(origPos int, fv *efi.Volume) error {
	// Serialize and deserialize to get updated ReadOffsets and thus correct
	// SecoreOffset.
	data, err := fv.Serialize()
	if err != nil {
		return fmt.Errorf("when roundtrip-serializing: %w", err)
	}
	fv2, err := efi.ReadVolume(efi.NewNestedReader(data))
	if err != nil {
		return fmt.Errorf("when roundtrip-deserializing: %w", err)
	}

	startPos, err := SecoreOffset(fv2)
	if err != nil {
		return err
	}
	needed := origPos - startPos
	glog.Infof("Pre-padding after patches: %d (need %d fixup)", startPos, needed)

	if needed == 0 {
		return nil
	}

	ipadding := len(fv.Files) - 2
	padding := fv.Files[ipadding]
	psize := int(padding.Size.Uint32() - 0x18)
	if needed < 0 {
		reduce := -needed
		if psize < reduce {
			return fmt.Errorf("Padding too small: need %d, got %d bytes", reduce, psize)
		}
		psize -= reduce
	} else {
		psize += needed
	}
	padding.Size = efi.ToUint24(uint32(psize) + 0x18)

	// One more roundtrip to check. This code isn't great.
	data, err = fv.Serialize()
	if err != nil {
		return fmt.Errorf("when check-serializing: %w", err)
	}
	fv3, err := efi.ReadVolume(efi.NewNestedReader(data))
	if err != nil {
		return fmt.Errorf("when check-deserializing: %w", err)
	}

	endPos, err := SecoreOffset(fv3)
	if err != nil {
		return err
	}
	if endPos != origPos {
		return fmt.Errorf("Failed to adjust padding (%d -> %d)", origPos, endPos)
	}
	return nil
}
