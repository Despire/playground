package bencoding

import (
	"reflect"
)

type Type string

const (
	ByteStringType Type = "BYTE_STRING"
	IntegerType         = "INTEGER"
	ListType            = "LIST"
	DictionaryType      = "DICTIONARY"
)

type Value interface {
	Type() Type
	Literal() string
}

type Decoder interface {
	// Decode decodes the next Bencoded value from the src.
	Decode(src []byte, position int) (int, error)
}

type DecodingError struct {
	typ reflect.Type
	msg string
}

func (e *DecodingError) Error() string { return "Failed to decode " + e.typ.String() + ": " + e.msg }
