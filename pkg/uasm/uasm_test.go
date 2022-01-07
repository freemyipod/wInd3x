package uasm

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestSample(t *testing.T) {
	p := Program{
		Address: 0x2202dc08,
		Listing: []Statement{
			// Load offset
			Ldr{Dest: R0, Src: Constant(0x2202db00)},
			Ldr{Dest: R1, Src: Deref(R0, 0)},

			// Copy 0x40 bytes from R1 to R0.
			Mov{Dest: R2, Src: Immediate(0)},

			Label("loop"),
			Ldrb{Dest: R3, Src: Deref(R1, 0)},
			Strb{Src: R3, Dest: Deref(R0, 0)},
			Add{Dest: R0, Src: R0, Compl: Immediate(1)},
			Add{Dest: R1, Src: R1, Compl: Immediate(1)},
			Add{Dest: R2, Src: R2, Compl: Immediate(1)},
			Cmp{A: R2, B: Immediate(0x40)},
			B{Cond: NE, Dest: LabelRef("loop")},

			Ldr{Dest: LR, Src: Constant(0x20004d70)},
			Bx{Dest: LR},
		},
	}

	res := p.Assemble()
	want, _ := hex.DecodeString("28009fe5001090e50020a0e30030d1e50030c0e5010080e2011081e2012082e2400052e3f8ffff1a04e09fe51eff2fe100db0222704d0020")
	if !bytes.Equal(res, want) {
		t.Fatalf("wrong assembly")
	}
}
