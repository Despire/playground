package peer

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
			b:    NewBitfield(1, false),
			args: args{3},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.B[0]>>4)&0x1)
			},
		},
		{
			name: "ok-set-0-in-byte-1",
			b:    NewBitfield(1, false),
			args: args{0},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.B[0]>>7)&0x1)
			},
		},
		{
			name: "ok-set-7-in-byte-1",
			b:    NewBitfield(1, false),
			args: args{7},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), b.B[0]&0x1)
			},
		},
		{
			name: "ok-set-overflow",
			b:    NewBitfield(2, true),
			args: args{8},
			validate: func(t *testing.T, b *BitField) {
				assert.Equal(t, uint8(1), (b.B[1]>>7)&0x1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			bitfield: NewBitfield(2, true),
			args:     args{9},
			wantErr:  func(t assert.TestingT, err error, i ...interface{}) bool { return assert.NotNil(t, err) },
		},
		{
			name:     "ok-set-overflow",
			bitfield: NewBitfield(2, true),
			args:     args{8},
			wantErr:  func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, tt.bitfield.SetWithCheck(tt.args.idx), fmt.Sprintf("SetWithCheck(%v)", tt.args.idx))
		})
	}
}
