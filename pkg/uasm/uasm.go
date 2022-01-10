// package uasm implements a boneless pseudo assembler and linker for ARMv6.
// It's used to generate payloads for wInd3x without relying on a third-party
// assembler at runtime, or precompiling payloads.
package uasm

import (
	"fmt"
)

// Program is a snippet of ARM code to be run at a given address.
type Program struct {
	Address uint32
	Listing []Statement
}

func (p *Program) Assemble() []byte {
	var size uint32
	for _, l := range p.Listing {
		size += l.size()
	}

	ctx := ctx{
		p:         p,
		instrAddr: p.Address,

		constantPoolStart: p.Address + size,
		constantPool:      make(map[uint32]uint32),
		constantPoolList:  nil,

		labels: make(map[string]uint32),
	}

	// First pass: labels must be created.
	for _, l := range p.Listing {
		isize := l.size()
		l.preprocess(&ctx)
		ctx.instrAddr += isize
	}

	// Second pass: bytes must be emitted.
	ctx.instrAddr = p.Address
	var res []byte
	for _, l := range p.Listing {
		isize := l.size()
		if isize == 0 {
			continue
		}
		data := l.hydrate(&ctx)
		ctx.instrAddr += isize
		res = append(res, data...)
	}

	for _, c := range ctx.constantPoolList {
		res = append(res, p32(c)...)
	}

	return res
}

type Register int

const (
	R0 Register = 0
	R1 Register = 1
	R2 Register = 2
	R3 Register = 3
	R4 Register = 4
	SP Register = 13
	LR Register = 14
	PC Register = 15
)

func (r Register) Encode() uint32 {
	return uint32(r)
}

type Condition string

const (
	AL Condition = ""
	NE Condition = "NE"
)

func (c Condition) Encode() uint32 {
	switch c {
	case AL:
		return 0b1110 << 28
	case NE:
		return 0b0001 << 28
	}
	panic("invalid condition")
}

// Statement is a listing line, eg. instruction or label.
type Statement interface {
	// preprocess is a first pass assemble function, giving the statements an
	// opportunity to register labels.
	preprocess(c *ctx)
	// hydrate is the second pass assemble function, in which a statement must
	// return concrete data.
	hydrate(c *ctx) []byte
	// size of the instruction in bytes.
	size() uint32
}

type ctx struct {
	p         *Program
	instrAddr uint32

	constantPoolStart uint32
	constantPool      map[uint32]uint32
	constantPoolList  []uint32

	labels map[string]uint32
}

func (h *ctx) AllocateConstant(val uint32) uint32 {
	if a, ok := h.constantPool[val]; ok {
		return a
	}
	a := h.constantPoolStart
	h.constantPoolStart += 4
	h.constantPool[val] = a
	h.constantPoolList = append(h.constantPoolList, val)

	return a
}

// instruction is an embeddable struct to be put in any 'typical' 4-byte ARM
// instruction that has no preprocess step.
type instruction struct {
}

func (i instruction) size() uint32 {
	return 4
}

func (i instruction) preprocess(c *ctx) {
}

func offsetForward(from, to uint32) uint16 {
	pcAddr := from + 8
	if to < pcAddr {
		panic("nonsense")
	}
	offset := to - pcAddr
	if offset >= (1 << 12) {
		panic("constant too far away")
	}
	return uint16(offset)
}

func p32(u uint32) []byte {
	return []byte{
		byte((u >> 0) & 0xff),
		byte((u >> 8) & 0xff),
		byte((u >> 16) & 0xff),
		byte((u >> 24) & 0xff),
	}
}

type Label string

func (l Label) size() uint32 {
	return 0
}

func (l Label) preprocess(c *ctx) {
	v := string(l)
	if _, ok := c.labels[v]; ok {
		panic(fmt.Sprintf("duplicate label %q", v))
	}
	c.labels[v] = c.instrAddr
}

func (l Label) hydrate(c *ctx) []byte {
	return nil
}

type LabelRef string

func (r LabelRef) resolveBranchTarget(c *ctx) uint32 {
	addr, ok := c.labels[string(r)]
	if !ok {
		panic(fmt.Sprintf("unknown label %q", string(r)))
	}
	return addr
}

func (r LabelRef) encodeLoadSource(c *ctx) uint32 {
	val := r.resolveBranchTarget(c)
	addr := c.AllocateConstant(val)
	md := MemoryDeref{
		Reg:    PC,
		Offset: offsetForward(c.instrAddr, addr),
	}
	return md.encodeLoadSource(c)
}

type Embed []byte

func (e Embed) size() uint32 {
	return uint32(len([]byte(e)))
}

func (e Embed) preprocess(_ *ctx) {
}

func (e Embed) hydrate(c *ctx) []byte {
	return []byte(e)
}
