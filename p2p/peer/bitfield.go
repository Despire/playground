package peer

import (
	"errors"
	"fmt"
)

type BitField struct {
	B            []byte
	overflowBits uint32
}

func NewBitfield(pieces int64) *BitField {
	return &BitField{
		// If pieces % 8 == 0 then pieces + 7 will still be equal to the same number of bytes.
		// If pieces % 8 != 0 then pieces + 8 will result in length longer by exactly 1 byte.
		B:            make([]byte, (pieces+7)/8),
		overflowBits: uint32(pieces % 8), // 1bit -> 1piece.
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
	if b.overflowBits != 0 {
		last := uint32(len(b.B) - 1)
		pieceIdx := b.byteOffset(idx)
		offset := b.bitOffset(idx)

		if pieceIdx == last && offset >= b.overflowBits {
			return errors.New("trying to set bits that do not belong to the torrent")
		}
	}
	return nil
}

func (b *BitField) byteOffset(idx uint32) uint32 { return idx / (1 << 3) }
func (b *BitField) bitOffset(idx uint32) uint32  { return idx % (1 << 3) }
