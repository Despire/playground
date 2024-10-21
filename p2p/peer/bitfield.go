package peer

import "fmt"

type BitField []byte

func (b BitField) SetWithCheck(idx uint32) error {
	if field := idx / (1 << 3); int(field) > len(b)-1 {
		return fmt.Errorf("%s is out of range for bitfield")
	}
	b.Set(idx)
	return nil
}

func (b BitField) Set(idx uint32) {
	field := idx / (1 << 3)
	_ = b[field] // bounds check
	piece := idx % (1 << 3)
	b[field] ^= 1 << (((1 << 3) - 1) - piece)
}
