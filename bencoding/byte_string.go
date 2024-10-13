package bencoding

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

var (
	// errStringSeparatorNotFound is returned when unmarshalling a byte string contains invalid format
	// that has no separator character ':'.
	errStringSeparatorNotFound = errors.New("expected string separator ':', but did not found")
	// errStringLengthInvalid is returned when the string length denoting the contents of the bytes string
	// failed to be parsed correctly.
	errStringLengthInvalid = errors.New("failed to decode string lenght")
)

const (
	// stringSeparator is the delimiter between the string length and the actual contents of a string.
	stringSeparator byte = ':'
)

// ByteString represents a bendecoded string.
type ByteString string

var byteStringType = reflect.TypeFor[ByteString]()

func (s *ByteString) decode(b []byte) error {
	before, after, found := bytes.Cut(b, []byte{stringSeparator})
	if !found {
		return errStringSeparatorNotFound
	}

	l, err := strconv.ParseInt(string(before), 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %w", errStringLengthInvalid, err)
	}

	*s = ByteString(after[:l])
	return nil
}

func (s *ByteString) encode() []byte {
	l := strconv.Itoa(len(*s))

	buffer := make([]byte, len(l)+1+len(*s))
	copy(buffer[:len(l)], []byte(l))

	buffer[len(l)] = ':'
	copy(buffer[len(l)+1:], *s)
	return buffer
}
