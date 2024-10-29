package bitfield

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitField_Set(t *testing.T) {
	type args struct {
		idx uint32
	}
	tests := []struct {
		name     string
		b        *BitField
		args     args
		validate func(t *testing.T, b *BitField)
	}{
		{
			name: "ok-set-3-in-byte-1",
			b:    NewBitfield(8),
			args: args{3},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.b[0]>>4)&0x1)
			},
		},
		{
			name: "ok-set-0-in-byte-1",
			b:    NewBitfield(8),
			args: args{0},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.b[0]>>7)&0x1)
			},
		},
		{
			name: "ok-set-7-in-byte-1",
			b:    NewBitfield(8),
			args: args{7},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), b.b[0]&0x1)
			},
		},
		{
			name: "ok-set-overflow",
			b:    NewBitfield(9),
			args: args{8},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.b[1]>>7)&0x1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.b.Set(tt.args.idx)
			tt.validate(t, tt.b)
		})
	}
}

func TestBitField_SetWithCheck(t *testing.T) {
	type args struct {
		idx uint32
	}
	tests := []struct {
		name     string
		bitfield *BitField
		args     args
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "err-set-overflow",
			bitfield: NewBitfield(9),
			args:     args{9},
			wantErr:  func(t assert.TestingT, err error, i ...interface{}) bool { return assert.NotNil(t, err) },
		},
		{
			name:     "ok-set-overflow",
			bitfield: NewBitfield(9),
			args:     args{8},
			wantErr:  func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.wantErr(t, tt.bitfield.SetWithCheck(tt.args.idx), fmt.Sprintf("SetWithCheck(%v)", tt.args.idx))
		})
	}
}

func TestBitField_MissingPieces(t *testing.T) {
	tests := []struct {
		name string
		b    *BitField
		want []uint32
	}{
		{
			name: "ok-overflow",
			b:    NewBitfield(1),
			want: []uint32{0},
		},
		{
			name: "ok-not-overflow",
			b:    NewBitfield(8),
			want: []uint32{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			name: "ok-overflow-2",
			b:    NewBitfield(9),
			want: []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.b.MissingPieces(), "MissingPieces()")
		})
	}
}

func Test_ExistingPieces(t *testing.T) {
	t.Parallel()

	pieces := []uint32{
		9,
		837,
		586,
		358,
		854,
		459,
		177,
		803,
		1436,
		791,
		241,
		1936,
		535,
		1781,
		556,
		842,
		172,
	}

	b := NewBitfield(2105)
	for _, p := range pieces {
		b.Set(p)
	}

	if len(b.ExistingPieces()) != len(pieces) {
		t.Errorf("mismatch existing pieces not equal to inserted amount")
	}
}
