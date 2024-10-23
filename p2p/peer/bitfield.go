package peer

import (
	"errors"
	"fmt"
)

type BitField struct {
	B       []byte
	barrier *uint32
}

func NewBitfield(blocks int64, overflow bool) *BitField {
	return &BitField{
		B: make([]byte, blocks),
		barrier: func() *uint32 {
			if overflow {
				i := uint32(1)
				return &i
			}
			return nil
		}(),
	}
}

func (b *BitField) SetWithCheck(idx uint32) error {
	if err := b.validate(idx); err != nil {
		return err
	}
	b.Set(idx)
	return nil
}

func (b *BitField) Set(idx uint32) {
	o := b.byteOffset(idx)
	_ = b.B[o] // bounds check
	piece := b.bitOffset(idx)
	b.B[o] ^= 1 << (((1 << 3) - 1) - piece)
}

func (b *BitField) validate(idx uint32) error {
	if field := b.byteOffset(idx); int(field) > len(b.B)-1 {
		return fmt.Errorf("%v is out of range for bitfield", field)
	}
	if b.barrier != nil {
		last := uint32(len(b.B) - 1)
		pieceIdx := b.byteOffset(idx)
		offset := b.bitOffset(idx)

		if pieceIdx == last && offset >= *b.barrier {
			return errors.New("trying to set bits that do not belong to the torrent")
		}
	}
	return nil
}

func (b *BitField) byteOffset(idx uint32) uint32 { return idx / (1 << 3) }
func (b *BitField) bitOffset(idx uint32) uint32  { return idx % (1 << 3) }
