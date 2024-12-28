package formats

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

type Jpg struct {
	contents []byte
}

func NewJpg(contents []byte) (*Jpg, error) {
	if bytes.HasPrefix(contents, JpgHeaderStart) {
		return &Jpg{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse jpg")
}

func (j *Jpg) Format() FileFormat { return JPG }
func (j *Jpg) IsParasite() bool   { return false }
func (j *Jpg) Reader() io.Reader  { return bytes.NewReader(j.contents) }

func (j *Jpg) Infect(file Parasite) ([]byte, error) {
	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(b)+len(j.contents)+2) // 2 is the block start and end byte
	out = append(out, JpgHeaderStart...)

	out = append(out, []byte("\xFE")...)

	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(b)+2)) // 2 for the extra start end blocks

	out = append(out, length...)

	out = append(out, b...)
	out = append(out, []byte("\xFF")...)

	out = append(out, j.contents[len(JpgHeaderStart):]...)
	return out, nil
}

func (j *Jpg) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(j.contents, "\n"...), b...), nil
}
