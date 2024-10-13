package bencoding

import (
	"bytes"
	"reflect"
	"strconv"
)

// ByteString represents a decoded byte string form the Bencoding format.
type ByteString string

func (s *ByteString) Decode(b []byte) error {
	before, after, found := bytes.Cut(b, []byte{byte(valueDelimiter)})
	if !found {
		return &DecodingError{
			typ: reflect.TypeOf(*s),
			msg: "expected separator ':' while parsing string, but did not found",
		}
	}

	l, err := strconv.ParseInt(string(before), 10, 64)
	if err != nil {
		return &DecodingError{
			typ: reflect.TypeOf(*s),
			msg: "failed to decode length of the string: " + err.Error(),
		}
	}

	*s = ByteString(after[:l])
	return nil
}

func (s *ByteString) Encode() []byte {
	l := strconv.Itoa(len(*s))

	buffer := make([]byte, len(l)+1+len(*s))
	copy(buffer[:len(l)], l)

	buffer[len(l)] = ':'
	copy(buffer[len(l)+1:], *s)
	return buffer
}
