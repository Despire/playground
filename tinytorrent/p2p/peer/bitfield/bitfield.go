package bitfield

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

type BitField struct {
	b         []byte
	numPieces int64
	l         sync.Mutex
}

func NewBitfield(numPieces int64) *BitField {
	blocks := float64(numPieces) / 8.0
	blocks = math.Ceil(blocks)
	return &BitField{
		b:         make([]byte, int64(blocks)),
		numPieces: numPieces,
	}
}
func (b *BitField) MissingPieces() []uint32 {
	var pieces []uint32

	b.l.Lock()
	defer b.l.Unlock()

	for i := int64(0); i < b.numPieces; i++ {
		o := b.byteOffset(uint32(i))
		_ = b.b[o] // bounds check
		piece := b.bitOffset(uint32(i))
		shift := ((1 << 3) - 1) - piece
		if (b.b[o] & (1 << shift)) == 0 {
			pieces = append(pieces, uint32(i))
		}
	}

	return pieces
}

func (b *BitField) ExistingPieces() []uint32 {
	var pieces []uint32

	b.l.Lock()
	defer b.l.Unlock()

	for i := int64(0); i < b.numPieces; i++ {
		o := b.byteOffset(uint32(i))
		_ = b.b[o] // bounds check
		piece := b.bitOffset(uint32(i))
		shift := ((1 << 3) - 1) - piece
		if (b.b[o] & (1 << shift)) != 0 {
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

	if int64(idx) >= b.numPieces {
		return errors.New("trying to set bits that do not belong to the torrent")
	}

	if field := b.byteOffset(idx); int(field) > len(b.b)-1 {
		return fmt.Errorf("%v is out of range for bitfield", field)
	}

	o := b.byteOffset(idx)
	_ = b.b[o] // bounds check
	piece := b.bitOffset(idx)
	shift := ((1 << 3) - 1) - piece
	b.b[o] |= 1 << shift
	return nil
}

func (b *BitField) Set(idx uint32) {
	b.l.Lock()
	defer b.l.Unlock()

	o := b.byteOffset(idx)
	_ = b.b[o] // bounds check
	piece := b.bitOffset(idx)
	shift := ((1 << 3) - 1) - piece
	b.b[o] |= 1 << shift
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
