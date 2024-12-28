package formats

import (
	"bytes"
	"errors"
	"github.com/Despire/ff-tools/algo"
	"io"
)

// customSection marker for WASM.
const customSection byte = 0x0

type Wasm struct {
	contents []byte
}

func NewWasm(contents []byte) (*Wasm, error) {
	if bytes.HasPrefix(contents, WASMHEaderStart) {
		return &Wasm{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse WASM")
}

func (w *Wasm) Format() FileFormat { return WASM }
func (w *Wasm) IsParasite() bool   { return false }
func (w *Wasm) Reader() io.Reader  { return bytes.NewReader(w.contents) }

func (w *Wasm) Infect(file Parasite) ([]byte, error) {
	b, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(b)+len(w.contents)+1+4) // marker + length
	out = append(out, WASMHEaderStart...)

	// custom section
	out = append(out, customSection)

	out = append(out, algo.ToLEB128(uint64(len(b)))...)
	out = append(out, b...)

	out = append(out, w.contents[len(WASMHEaderStart):]...)
	return out, nil
}

func (w *Wasm) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(b)+len(w.contents)+1+4) // marker + length
	out = append(out, w.contents...)
	out = append(out, customSection)
	out = append(out, algo.ToLEB128(uint64(len(b)))...)
	out = append(out, b...)

	return out, nil
}
