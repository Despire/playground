package formats

import (
	"io"
	"log"
)

type FileFormat int

const (
	PDF     FileFormat = 0x1
	ZIP     FileFormat = 0x2
	PNG     FileFormat = 0x3
	JPG     FileFormat = 0x4
	WASM    FileFormat = 0x5
	NES     FileFormat = 0x6
	GIF     FileFormat = 0x7
	LITERAL FileFormat = 0x8
	MP3     FileFormat = 0x9
)

func (f FileFormat) String() string {
	switch f {
	case PDF:
		return "pdf"
	case ZIP:
		return "zip"
	case PNG:
		return "png"
	case JPG:
		return "jpg"
	case WASM:
		return "wasm"
	case NES:
		return "nes"
	case GIF:
		return "gif"
	case LITERAL:
		return "binary-literal"
	case MP3:
		return "mp3"
	default:
		panic("unknown fileformat")
	}
}

type FormatChecker interface {
	Format() FileFormat
}

type Attacher interface {
	Attach(reader io.Reader) ([]byte, error)
}

type Infectable interface {
	Infect(file Parasite) ([]byte, error)
}

type C interface {
	Infectable
	Attacher
	FormatChecker
}

type Parasite interface {
	IsParasite() bool
	Reader() io.Reader
	FormatChecker
}

func Find(f []byte) (FormatChecker, error) {
	pdf, err := NewPdf(f)
	if err == nil {
		return pdf, nil
	}

	z, err := NewZip(f)
	if err == nil {
		return z, nil
	}

	p, err := NewPng(f)
	if err == nil {
		return p, nil
	}

	j, err := NewJpg(f)
	if err == nil {
		return j, nil
	}

	w, err := NewWasm(f)
	if err == nil {
		return w, nil
	}

	n, err := NewNes(f)
	if err == nil {
		return n, nil
	}

	g, err := NewGif(f)
	if err == nil {
		return g, nil
	}

	m, err := NewMp3(f)
	if err == nil {
		return m, nil
	}

	log.Printf("didn't matched any file formats, defaulting as identying the file as binary\n")
	return NewBinary(f)
}
