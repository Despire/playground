package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitField_Set(t *testing.T) {
	type args struct {
		idx uint32
	}
	tests := []struct {
		name     string
		b        BitField
		args     args
		validate func(t *testing.T, b BitField)
	}{
		{
			name: "ok-set-3-in-byte-1",
			b:    []byte{0x0},
			args: args{3},
			validate: func(t *testing.T, b BitField) {
				assert.Equal(t, uint8(1), (b[0]>>4)&0x1)
			},
		},
		{
			name: "ok-set-0-in-byte-1",
			b:    []byte{0x0},
			args: args{0},
			validate: func(t *testing.T, b BitField) {
				assert.Equal(t, uint8(1), (b[0]>>7)&0x1)
			},
		},
		{
			name: "ok-set-7-in-byte-1",
			b:    []byte{0x0},
			args: args{7},
			validate: func(t *testing.T, b BitField) {
				assert.Equal(t, uint8(1), b[0]&0x1)
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
