package bencoding

import (
	"reflect"
	"strconv"
	"unsafe"
)

// Integer represents a decoded integer from the Bencoding format.
type Integer int64

func (i *Integer) Equal(o Value) bool {
	rv := reflect.ValueOf(o)
	if rv.Type() != reflect.TypeFor[*Integer]() {
		return false
	}
	if (rv.IsNil() && i != nil) || (i == nil && !rv.IsNil()) {
		return false
	}
	if i == nil {
		return true
	}

	other := o.(*Integer)
	return *other == *i
}

func (i *Integer) Decode(src []byte, position int) (int, error) {
	if src[position] != byte(integerBegin) {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer, 'i' not found",
		}
	}

	start := position + 1
	end, err := advanceUntil(src, start, valueEnd)
	if err != nil {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer, 'e' not found",
		}
	}

	if end-start > 1 && src[start] == '0' {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "invalid integer, cannot have an integer format prefixed with 0 (i0xxxxx...e)",
		}
	}

	ii, err := strconv.ParseInt(string(src[start:end]), 10, int(unsafe.Sizeof(*i))*8)
	if err != nil {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*i),
			msg: "failed to parse integer: " + err.Error(),
		}
	}

	*i = Integer(ii)
	return end, nil
}

func (i *Integer) Encode() []byte {
	c := strconv.Itoa(int(*i))
	buffer := make([]byte, len(c)+2)
	buffer[0] = byte(integerBegin)
	copy(buffer[1:], c)
	buffer[len(buffer)-1] = byte(valueEnd)
	return buffer
}
