package cfw

import (
	"bytes"
	"fmt"

	"github.com/freemyipod/wInd3x/pkg/efi"
)

// VisitVolume is the main call used to patch an EFI volume. The patching to be
// performed is defined by the VolumeVisitor given.
func VisitVolume(v *efi.Volume, vi VolumeVisitor) error {
	for _, file := range v.Files {
		if err := vi.VisitFile(file); err != nil {
			return err
		}
		for _, section := range file.Sections {
			if err := visitSection(section, vi); err != nil {
				return err
			}
		}
	}
	return vi.Done()
}

func visitSection(s efi.Section, vi VolumeVisitor) error {
	if err := vi.VisitSection(s); err != nil {
		return err
	}
	for _, sub := range s.Sub() {
		if f := sub.File; f != nil {
			if err := vi.VisitFile(f); err != nil {
				return err
			}
		}
		if s := sub.Section; s != nil {
			if err := visitSection(s, vi); err != nil {
				return err
			}
		}
	}
	return nil
}

// VolumeVisitor is a visitor which can modify an EFI firmware volume. Its
// methods will be called by VisitVolume as it recursively traverses files and
// sections contained in files.
type VolumeVisitor interface {
	// VisitFile will be called when the traversal encounters a file.
	VisitFile(file *efi.FirmwareFile) error
	// VisitSection will be called when the traversal encounters a section
	// withing a file.
	VisitSection(section efi.Section) error
	// Done will be called when the traversal is done with all files/sections.
	Done() error
}

// MultipleVisitors implements VolumeVisitor by calling subordinate
// VolumeVisitors in parallel, allowing multiple files within a single firmware
// volume to be patched.
type MultipleVisitors []VolumeVisitor

func (m MultipleVisitors) VisitFile(file *efi.FirmwareFile) error {
	for _, vi := range m {
		if err := vi.VisitFile(file); err != nil {
			return err
		}
	}
	return nil
}

func (m MultipleVisitors) VisitSection(section efi.Section) error {
	for _, vi := range m {
		if err := vi.VisitSection(section); err != nil {
			return err
		}
	}
	return nil
}

func (m MultipleVisitors) Done() error {
	for _, vi := range m {
		if err := vi.Done(); err != nil {
			return err
		}
	}
	return nil
}

// VisitPE32InFile is a VolumeVisitor which applies a Patch on a single PE32
// section within a file. The section doesn't have to be top-level.
type VisitPE32InFile struct {
	// FileGUID is the file on whose PE32 section Patch will be applied.
	FileGUID efi.GUID
	Patch    Patch

	inFile  bool
	applied bool
}

func (v *VisitPE32InFile) VisitFile(file *efi.FirmwareFile) error {
	v.inFile = file.GUID.String() == v.FileGUID.String()
	return nil
}

func (v *VisitPE32InFile) VisitSection(section efi.Section) error {
	if !v.inFile {
		return nil
	}
	if section.Header().Type == efi.SectionTypePE32 {
		if v.applied {
			return fmt.Errorf("more than one PE32 section found")
		}
		out, err := v.Patch.Apply(section.Raw())
		if err != nil {
			return fmt.Errorf("patching failed: %w", err)
		}
		section.SetRaw(out)
		v.applied = true
	}
	return nil
}

func (v *VisitPE32InFile) Done() error {
	if !v.applied {
		return fmt.Errorf("guid %s not found", v.FileGUID.String())
	}
	return nil
}

// Patch defined an operation performed on some binary blob.
type Patch interface {
	Apply(in []byte) (out []byte, err error)
}

// Patches implements Patch by calling a series of Patches in sequnce. This
// allows applying multiple Patches to a single section.
type Patches []Patch

func (p Patches) Apply(in []byte) ([]byte, error) {
	cur := in
	for i, s := range p {
		next, err := s.Apply(cur)
		if err != nil {
			return nil, fmt.Errorf("patch %d: %w", i, err)
		}
		cur = next
	}
	return cur, nil
}

// ReplaceAt replaces all occurences of a given pattern with another sequence.
// The sequences must be equal length.
type ReplaceExact struct {
	From []byte
	To   []byte
}

func (p ReplaceExact) Apply(in []byte) ([]byte, error) {
	if len(p.From) != len(p.To) {
		return nil, fmt.Errorf("from/to is different length")
	}
	if bytes.Equal(p.From, p.To) {
		return nil, fmt.Errorf("pattern is a no-op")
	}
	out := bytes.ReplaceAll(in, p.From, p.To)
	if bytes.Equal(in, out) {
		return nil, fmt.Errorf("pattern not found")
	}
	return out, nil
}

// PatchAt writes a sequence of bytes at a given offset.
type PatchAt struct {
	Address int
	To      []byte
}

func (p PatchAt) Apply(in []byte) ([]byte, error) {
	if len(in) < p.Address+len(p.To) {
		return nil, fmt.Errorf("input too small")
	}

	data := in[:p.Address]
	data = append(data, p.To...)
	data = append(data, in[p.Address+len(p.To):]...)
	return data, nil
}
