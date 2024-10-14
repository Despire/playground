package bencoding

import (
	"fmt"
	"reflect"
	"strconv"
)

// ByteString represents a decoded byte string form the Bencoding format.
type ByteString string

func (s *ByteString) Type() Type      { return ByteStringType }
func (s *ByteString) Literal() string { return fmt.Sprintf("%d:%s", len(*s), *s) }

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
