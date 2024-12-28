package formats

import (
	"bytes"
	"errors"
	"io"
)

type Gif struct {
	contests []byte
}

func NewGif(contents []byte) (*Gif, error) {
	if bytes.HasPrefix(contents, GIFHeaderStart) {
		return &Gif{
			contests: contents,
		}, nil
	}

	return nil, errors.New("failed to parse gif")
}

func (g *Gif) Format() FileFormat { return GIF }
func (g *Gif) IsParasite() bool   { return false }
func (g *Gif) Reader() io.Reader  { return bytes.NewReader(g.contests) }

func (g *Gif) Infect(file Parasite) ([]byte, error) {
	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	if len(b) > 0xff {
		return nil, errors.New("can't inject file, exceeds maximum length of 0xff")
	}

	out := make([]byte, 0, len(g.contests)+len(b))

	globalColorTable := 0
	if g.contests[0x0A]&0x80 > 0 {
		globalColorTable += 3 * (2 << int(g.contests[0x0A]&0x07))
	}

	out = append(out, g.contests[:len(GIFHeaderStart)+7+globalColorTable]...) // header + logical screen descriptor + global color table

	out = append(out, 0x21, 0xFE)
	out = append(out, byte(len(b)))
	out = append(out, b...)
	out = append(out, 0x00)
	out = append(out, g.contests[len(GIFHeaderStart)+7+globalColorTable:]...)

	return out, nil
}

func (g *Gif) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(g.contests, "\n"...), b...), nil
}
