package bencoding

import (
	"reflect"
	"strconv"
	"unsafe"
)

// Integer represents a decoded integer from the Bencoding format.
type Integer int64

func (i *Integer) Decode(b []byte) error {
	if b[0] != byte(integerBegin) {
		return &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer, 'i' not found",
		}
	}
	if b[len(b)-1] != byte(valueEnd) {
		return &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer, 'e' not found",
		}
	}
	b = b[1:]
	b = b[:len(b)-1]
	if len(b) > 1 && b[0] == '0' {
		return &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "invalid integer, cannot have an integer format prefixed with 0 (i0xxxxx...e)",
		}
	}

	ii, err := strconv.ParseInt(string(b), 10, int(unsafe.Sizeof(*i))*8)
	if err != nil {
		return &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer: " + err.Error(),
		}
	}

	*i = Integer(ii)
	return nil
}

func (i *Integer) Encode() []byte {
	c := strconv.Itoa(int(*i))
	buffer := make([]byte, len(c)+2)
	buffer[0] = byte(integerBegin)
	copy(buffer[1:], c)
	buffer[len(buffer)-1] = byte(valueEnd)
	return buffer
}
