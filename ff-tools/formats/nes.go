package formats

import (
	"bytes"
	"errors"
	"io"
	"log"
)

type Nes struct {
	contents []byte
}

func NewNes(contents []byte) (*Nes, error) {
	if bytes.HasPrefix(contents, NESHEaderStart) {
		return &Nes{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse NES")
}

func (n *Nes) Format() FileFormat { return NES }
func (n *Nes) IsParasite() bool   { return false }
func (n *Nes) Reader() io.Reader  { return bytes.NewReader(n.contents) }

func (n *Nes) Infect(file Parasite) ([]byte, error) {
	log.Printf("Injection via adding trainer this might or might not work.\n")
	log.Printf("A better way to inject files is to look for padded bytes or unused/uninitialzied memory\n")

	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	for len(b) < 512 {
		b = append(b, 0x0)
	}

	if len(b) > 512 {
		log.Printf("data to big for injection must be 512, truncating...\n")
	}

	b = b[:512]

	// set trainer bit
	n.contents[6] |= 1 << 2

	out := make([]byte, 0, len(n.contents)+512) // trainer size.
	out = append(out, n.contents[:len(NESHEaderStart)]...)

	out = append(out, b...)
	out = append(out, n.contents[len(NESHEaderStart):]...)

	return out, nil
}

func (n *Nes) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(n.contents, "\n"...), b...), nil
}
