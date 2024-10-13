package bencoding

import (
	"reflect"
)

type Value interface {
	Equal(o Value) bool
	Encoder
	Decoder
}

type Encoder interface {
	// Encode encodes the value to its bencoded representation.
	Encode() []byte
}

type Decoder interface {
	// Decode decodes the next Bencoded value from the src.
	Decode(src []byte, position int) (int, error)
}

type DecodingError struct {
	typ reflect.Type
	msg string
}

func (e *DecodingError) Error() string {
	return "Failed to decode " + e.typ.String() + ": " + e.msg
}
