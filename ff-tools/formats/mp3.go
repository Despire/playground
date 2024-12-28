package formats

import (
	"bytes"
	"errors"
	"github.com/bogem/id3v2/v2"
	"io"
)

type Mp3 struct {
	contents []byte
}

func NewMp3(contents []byte) (*Mp3, error) {
	if bytes.HasPrefix(contents, MP3HeaderStart) ||
		bytes.HasPrefix(contents, ID3V2Header) {
		return &Mp3{contents: contents}, nil
	}

	return nil, errors.New("failed to parse mp3")
}

func (m *Mp3) Format() FileFormat { return MP3 }
func (m *Mp3) IsParasite() bool   { return false }
func (m *Mp3) Reader() io.Reader  { return bytes.NewReader(m.contents) }

func (m *Mp3) Infect(file Parasite) ([]byte, error) {
	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	mp3, err := id3v2.ParseReader(bytes.NewReader(m.contents), id3v2.Options{
		Parse: true,
	})
	if err != nil {
		return nil, err
	}

	defer mp3.Close()

	commentFrame := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    "eng",
		Description: "eng",
		Text:        string(b),
	}

	mp3.AddCommentFrame(commentFrame)

	buff := new(bytes.Buffer)
	if _, err := mp3.WriteTo(buff); err != nil {
		return nil, err
	}

	return append(buff.Bytes(), m.contents...), nil
}

func (m *Mp3) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(m.contents, "\n"...), b...), nil
}
