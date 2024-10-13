package bencoding

import "reflect"

type Encoder interface {
	Encode() []byte
}

type Decoder interface {
	Decode([]byte) error
}

type DecodingError struct {
	typ reflect.Type
	msg string
}

func (e *DecodingError) Error() string {
	return "Failed to decode " + e.typ.String() + ": " + e.msg
}
