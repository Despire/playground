package bencoding

import (
	"errors"
	"io"
)

func Decode(src io.Reader) (Value, error) {
	b, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, nil
	}

	v := nextValue(b[0])
	if v == nil {
		return nil, errors.New("no bencoded value in input")
	}

	if _, err := v.(Decoder).Decode(b, 0); err != nil {
		return nil, err
	}

	return v, nil
}
