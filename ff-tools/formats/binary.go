package formats

import (
	"bytes"
	"errors"
	"io"
)

type Binary struct {
	contents []byte
}

func NewBinary(contents []byte) (*Binary, error) {
	return &Binary{contents: contents}, nil
}

func (b *Binary) Format() FileFormat { return LITERAL }
func (b *Binary) IsParasite() bool   { return true }
func (b *Binary) Reader() io.Reader  { return bytes.NewReader(b.contents) }

func (b *Binary) Infect(file Parasite) ([]byte, error) {
	return nil, errors.New("can't inject into binary")
}

func (b *Binary) Attach(reader io.Reader) ([]byte, error) {
	c, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(b.contents, "\n"...), c...), nil
}
