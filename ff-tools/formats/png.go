package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

type Png struct {
	contents []byte
}

func NewPng(contents []byte) (*Png, error) {
	if bytes.HasPrefix(contents, PngHeaderStart) {
		return &Png{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse png")
}

func (p *Png) Format() FileFormat { return PNG }
func (p *Png) IsParasite() bool   { return false }
func (p *Png) Reader() io.Reader  { return bytes.NewReader(p.contents) }

func (p *Png) Infect(file Parasite) ([]byte, error) {
	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	i := bytes.Index(p.contents, []byte("IHDR"))
	if i < 0 {
		return nil, errors.New("missing IHDR block invalid PNG file")
	}

	i += len("IHDR") + 13 + 4 // len of IHDR is 13 bytes + 4 bytes of crc32.

	out := make([]byte, 0, len(b)+len(p.contents))
	out = append(out, p.contents[:i]...)

	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(b)))

	out = append(out, length...)
	out = append(out, []byte("fILE")...)
	out = append(out, b...)

	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.ChecksumIEEE(append([]byte("fILE"), b...)))

	out = append(out, checksum...)
	out = append(out, p.contents[i:]...)

	return out, nil
}

func (p *Png) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(p.contents, "\n"...), b...), nil
}
