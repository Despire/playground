package bencoding

import (
	"reflect"
	"strconv"
)

// ByteString represents a decoded byte string form the Bencoding format.
type ByteString string

func (s *ByteString) Equal(o Value) bool {
	rv := reflect.ValueOf(o)
	if rv.Type() != reflect.TypeFor[*ByteString]() {
		return false
	}
	if (rv.IsNil() && s != nil) || (s == nil && !rv.IsNil()) {
		return false
	}
	if s == nil {
		return true
	}

	other := o.(*ByteString)
	return *other == *s
}

func (s *ByteString) Decode(src []byte, position int) (int, error) {
	delim, err := advanceUntil(src, position, valueDelimiter)
	if err != nil {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*s),
			msg: "expected separator ':' while parsing string, but did not found",
		}
	}

	l, err := strconv.ParseInt(string(src[position:delim]), 10, 64)
	if err != nil {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*s),
			msg: "failed to decode length of the string: " + err.Error(),
		}
	}

	start := delim + 1
	end := start + int(l)
	*s = ByteString(src[start:end])
	return end - 1, nil
}

func (s *ByteString) Encode() []byte {
	l := strconv.Itoa(len(*s))

	buffer := make([]byte, len(l)+1+len(*s))
	copy(buffer[:len(l)], l)

	buffer[len(l)] = ':'
	copy(buffer[len(l)+1:], *s)
	return buffer
}
