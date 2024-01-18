package uasm

// DataSource is an operand which can be a source of data to a non-memory
// operation.
type DataSource interface {
	encodeDataSource(c *ctx) uint32
}

// LoadSource is an operand which can be a source of data to a memory
// operation.
type LoadSource interface {
	encodeLoadSource(c *ctx) uint32
}

// StoreDest is an operand which can be a destination for a memory operation.
type StoreDest interface {
	encodeStoreDest(c *ctx) uint32
}

// Branch target is an operand that can be interpreted as a program address.
type BranchTarget interface {
	resolveBranchTarget(c *ctx) uint32
}

// Constant is a 32-bit number that will end up in a constant pool.
type Constant uint32

func (t Constant) encodeLoadSource(c *ctx) uint32 {
	val := uint32(t)
	addr := c.AllocateConstant(val)
	md := MemoryDeref{
		Reg:    PC,
		Offset: offsetForward(c.instrAddr, addr),
	}
	return md.encodeLoadSource(c)
}

type MemoryDeref struct {
	Reg    Register
	Offset uint16
}

func (m MemoryDeref) encodeLoadSource(c *ctx) uint32 {
	if m.Offset >= (1 << 12) {
		panic("offset too large")
	}

	var res uint32
	res |= uint32(m.Offset)
	res |= m.Reg.Encode() << 16
	return res
}

func (m MemoryDeref) encodeStoreDest(c *ctx) uint32 {
	if m.Offset >= (1 << 12) {
		panic("offset too large")
	}

	var res uint32
	res |= uint32(m.Offset)
	res |= m.Reg.Encode() << 16
	return res
}

func Deref(r Register, offset uint16) MemoryDeref {
	return MemoryDeref{
		Reg:    r,
		Offset: offset,
	}
}

// Immediate is a data source (for operations like mov, add, etc).
type Immediate uint32

func (i Immediate) encodeDataSource(c *ctx) uint32 {
	val := uint32(i)
	encodable := false
	if val >= (1 << 11) {
		for i := 0; i < 16; i++ {
			m := ((val << uint32(i*2)) | (val >> (32 - uint32(i*2)))) & 0xffffffff
			if m < 256 {
				val = (uint32(i) << 8) | m
				encodable = true
				break
			}
		}
		if !encodable {
			panic("unencodable immediate")
		}
	}
	var res uint32
	res |= 1 << 25
	res |= val
	return res
}

func (r Register) encodeDataSource(c *ctx) uint32 {
	var res uint32
	res |= r.Encode()
	return res
}
