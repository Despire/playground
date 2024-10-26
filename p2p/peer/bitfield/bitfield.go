package bitfield

import (
	"errors"
	"fmt"
	"sync"
)

type BitField struct {
	b       []byte
	barrier *uint32
	l       sync.Mutex
}

func NewBitfield(blocks int64, overflow bool) *BitField {
	return &BitField{
		b: make([]byte, blocks),
		barrier: func() *uint32 {
			if overflow {
				i := uint32(1)
				return &i
			}
			return nil
		}(),
	}
}
func (b *BitField) MissingPieces() []uint32 {
	var pieces []uint32

	b.l.Lock()
	defer b.l.Unlock()

	pcs := len(b.b) * 8
	if b.barrier != nil {
		pcs -= 7
	}

	for i := 0; i < pcs; i++ {
		o := b.byteOffset(uint32(i))
		_ = b.b[o] // bounds check
		piece := b.bitOffset(uint32(i))
		if (b.b[o] & 1 << (((1 << 3) - 1) - piece)) == 0 {
			pieces = append(pieces, uint32(i))
		}
	}

	return pieces
}

func (b *BitField) Clone() []byte {
	b.l.Lock()
	defer b.l.Unlock()

	res := make([]byte, len(b.b))
	copy(res, b.b)
	return res
}

func (b *BitField) Overwrite(other []byte) {
	b.l.Lock()
	defer b.l.Unlock()

	if len(b.b) != len(other) {
		panic("invalid bitfield overwrite")
	}

	b.b = other
}

func (b *BitField) Len() int {
	b.l.Lock()
	defer b.l.Unlock()
	return len(b.b)
}

func (b *BitField) SetWithCheck(idx uint32) error {
	b.l.Lock()
	defer b.l.Unlock()

	if field := b.byteOffset(idx); int(field) > len(b.b)-1 {
		return fmt.Errorf("%v is out of range for bitfield", field)
	}
	if b.barrier != nil {
		last := uint32(len(b.b) - 1)
		pieceIdx := b.byteOffset(idx)
		offset := b.bitOffset(idx)

		if pieceIdx == last && offset >= *b.barrier {
			return errors.New("trying to set bits that do not belong to the torrent")
		}
	}

	o := b.byteOffset(idx)
	_ = b.b[o] // bounds check
	piece := b.bitOffset(idx)
	b.b[o] ^= 1 << (((1 << 3) - 1) - piece)
	return nil
}

func (b *BitField) Set(idx uint32) {
	b.l.Lock()
	defer b.l.Unlock()

	o := b.byteOffset(idx)
	_ = b.b[o] // bounds check
	piece := b.bitOffset(idx)
	b.b[o] ^= 1 << (((1 << 3) - 1) - piece)
}

func (b *BitField) Check(idx uint32) bool {
	b.l.Lock()
	defer b.l.Unlock()

	o := b.byteOffset(idx)
	_ = b.b[o] // bounds check
	piece := b.bitOffset(idx)
	shift := ((1 << 3) - 1) - piece
	return (b.b[o] & (1 << shift)) != 0
}

func (b *BitField) byteOffset(idx uint32) uint32 { return idx / (1 << 3) }
func (b *BitField) bitOffset(idx uint32) uint32  { return idx % (1 << 3) }
