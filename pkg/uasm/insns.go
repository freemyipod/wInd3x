package uasm

type Ldr struct {
	instruction
	Dest Register
	Src  LoadSource
}

func (l Ldr) hydrate(c *ctx) []byte {
	var res uint32
	res |= l.Src.encodeLoadSource(c)
	res |= l.Dest.Encode() << 12
	res |= 0b111001011001 << 20
	return p32(res)
}

type Ldrb struct {
	instruction
	Dest Register
	Src  LoadSource
}

func (l Ldrb) hydrate(c *ctx) []byte {
	var res uint32
	res |= l.Src.encodeLoadSource(c)
	res |= l.Dest.Encode() << 12
	res |= 0b111001011101 << 20
	return p32(res)
}

type Str struct {
	instruction
	Src  Register
	Dest StoreDest
}

func (s Str) hydrate(c *ctx) []byte {
	var res uint32
	res |= s.Dest.encodeStoreDest(c)
	res |= s.Src.Encode() << 12
	res |= 0b111001011000 << 20
	return p32(res)
}

type Strb struct {
	instruction
	Src  Register
	Dest StoreDest
}

func (s Strb) hydrate(c *ctx) []byte {
	var res uint32
	res |= s.Dest.encodeStoreDest(c)
	res |= s.Src.Encode() << 12
	res |= 0b111001011100 << 20
	return p32(res)
}

type Bx struct {
	instruction
	Dest Register
}

func (b Bx) hydrate(c *ctx) []byte {
	var res uint32
	res |= b.Dest.Encode()
	res |= 0b1110000100101111111111110001 << 4
	return p32(res)
}

type Blx struct {
	instruction
	Dest Register
}

func (b Blx) hydrate(c *ctx) []byte {
	var res uint32
	res |= b.Dest.Encode()
	res |= 0b1110000100101111111111110011 << 4
	return p32(res)
}

type B struct {
	instruction
	Cond Condition
	Dest BranchTarget
}

func (b B) hydrate(c *ctx) []byte {
	addr := b.Dest.resolveBranchTarget(c)
	pcAddr := c.instrAddr + 8
	offset := (int64(addr) - int64(pcAddr)) / 4
	// math probably wrong, whatever
	if offset > (1<<15) || offset < -(1<<15) {
		panic("target too far away")
	}

	var res uint32
	res |= uint32(offset) & ((1 << 24) - 1)
	res |= 0b1010 << 24
	res |= b.Cond.Encode()
	return p32(res)
}

type Mov struct {
	instruction
	Dest Register
	Src  DataSource
}

func (m Mov) hydrate(c *ctx) []byte {
	var res uint32
	res |= m.Src.encodeDataSource(c)
	res |= m.Dest.Encode() << 12
	res |= 0b1110000110100000 << 16
	return p32(res)
}

type And struct {
	instruction
	Dest  Register
	Src   Register
	Compl DataSource
}

func (a And) hydrate(c *ctx) []byte {
	var res uint32
	res |= a.Dest.Encode() << 12
	res |= a.Src.Encode() << 16
	res |= a.Compl.encodeDataSource(c)
	res |= 0b111000000000 << 20
	return p32(res)
}

type Or struct {
	instruction
	Dest  Register
	Src   Register
	Compl DataSource
}

func (a Or) hydrate(c *ctx) []byte {
	var res uint32
	res |= a.Dest.Encode() << 12
	res |= a.Src.Encode() << 16
	res |= a.Compl.encodeDataSource(c)
	res |= 0b111000011000 << 20
	return p32(res)
}

type Add struct {
	instruction
	Dest  Register
	Src   Register
	Compl DataSource
}

func (a Add) hydrate(c *ctx) []byte {
	var res uint32
	res |= a.Dest.Encode() << 12
	res |= a.Src.Encode() << 16
	res |= a.Compl.encodeDataSource(c)
	res |= 0b111000001000 << 20
	return p32(res)
}

type Sub struct {
	instruction
	Dest  Register
	Src   Register
	Compl DataSource
}

func (a Sub) hydrate(c *ctx) []byte {
	var res uint32
	res |= a.Dest.Encode() << 12
	res |= a.Src.Encode() << 16
	res |= a.Compl.encodeDataSource(c)
	res |= 0b111000000100 << 20
	return p32(res)
}

type Cmp struct {
	instruction
	A Register
	B DataSource
}

func (m Cmp) hydrate(c *ctx) []byte {
	var res uint32
	res |= m.A.Encode() << 16
	res |= m.B.encodeDataSource(c)
	res |= 0b111000010101 << 20
	return p32(res)
}

type Mcr struct {
	instruction
	Opc  uint8
	CRn  uint8
	Src  Register
	CPn  uint8
	Opc2 uint8
	CRm  uint8
}

func (m Mcr) hydrate(c *ctx) []byte {
	var res uint32
	res |= 0b11101110 << 24
	res |= uint32(m.Opc) << 21
	res |= uint32(m.CRn) << 16
	res |= m.Src.Encode() << 12
	res |= uint32(m.CPn) << 8
	res |= uint32(m.Opc2) << 5
	res |= 1 << 4
	res |= uint32(m.CRm)
	return p32(res)
}

type Mrc struct {
	instruction
	Opc uint8
	// CRn is the coprocessor register number.
	CRn  uint8
	Dest Register
	// CPn is the coprocessor number (eg. CP15).
	CPn  uint8
	Opc2 uint8
	// CRm is the additional coprocessor register number.
	CRm uint8
}

func (m Mrc) hydrate(c *ctx) []byte {
	var res uint32
	res |= 0b11101110 << 24
	res |= uint32(m.Opc) << 21
	res |= 1 << 20
	res |= uint32(m.CRn) << 16
	res |= m.Dest.Encode() << 12
	res |= uint32(m.CPn) << 8
	res |= uint32(m.Opc2) << 5
	res |= 1 << 4
	res |= uint32(m.CRm)
	return p32(res)
}
