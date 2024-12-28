package formats

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
)

func tryCombining(i C, p Parasite) ([]io.Reader, error) {
	var result []io.Reader

	first, err := i.Infect(p)
	if err == nil {
		result = append(result, bytes.NewReader(first))
	}
	if err != nil {
		log.Printf("couldn't infect %s with %s skipping", i.Format().String(), p.Format().String())
	}

	second, err := i.Attach(p.Reader())
	if err == nil {
		result = append(result, bytes.NewReader(second))
	}
	if err != nil {
		log.Printf("couldn't attach %s to %s", p.Format().String(), i.Format().String())
	}

	return result, nil
}

func Combine(f1 FormatChecker, f2 FormatChecker) ([]io.Reader, error) {
	switch f1.Format() {
	case PDF:
		return pdfWrap(f1.(*Pdf), f2)
	case ZIP:
		return zipWrap(f1.(*Zip), f2)
	case PNG:
		return pngWrap(f1.(*Png), f2)
	case JPG:
		return jpgWrap(f1.(*Jpg), f2)
	case WASM:
		return wasmWrap(f1.(*Wasm), f2)
	case NES:
		return nesWrap(f1.(*Nes), f2)
	case GIF:
		return gifWrap(f1.(*Gif), f2)
	case LITERAL:
		return binaryWrap(f1.(*Binary), f2)
	case MP3:
		return mp3Wrap(f1.(*Mp3), f2)
	default:
		return nil, errors.New("unknown fileformat for f1")
	}
}

func mp3Wrap(m *Mp3, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(m, f2.(Parasite))
	case PNG, JPG, WASM, NES:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), m.Format().String())
	case MP3:
		return nil, errors.New("failed to merge file of the same type")
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func binaryWrap(b *Binary, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(b, f2.(Parasite))
	case PNG, JPG, WASM, NES, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), b.Format().String())
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func gifWrap(gif *Gif, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(gif, f2.(Parasite))
	case PNG, JPG, WASM, NES, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), gif.Format().String())
	case GIF:
		return nil, errors.New("failed to merge file of the same type")
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func nesWrap(nes *Nes, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(nes, f2.(Parasite))
	case PNG, JPG, WASM, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), nes.Format().String())
	case NES:
		return nil, errors.New("failed to merge file of the same type")
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func wasmWrap(wasm *Wasm, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(wasm, f2.(Parasite))
	case PNG, JPG, NES, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), wasm.Format().String())
	case WASM:
		return nil, errors.New("failed to merge file of the same type")
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func jpgWrap(jpg *Jpg, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(jpg, f2.(Parasite))
	case PNG, WASM, NES, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), jpg.Format().String())
	case JPG:
		return nil, errors.New("failed to merge two file of the same type")
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func pngWrap(png *Png, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, ZIP, LITERAL:
		return tryCombining(png, f2.(Parasite))
	case PNG:
		return nil, errors.New("failed to merge two file of the same type")
	case JPG, WASM, NES, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't inject or attach to %s", f2.Format().String(), png.Format().String())
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func pdfWrap(pdf *Pdf, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case ZIP, PDF, LITERAL:
		return tryCombining(pdf, f2.(Parasite))
	case PNG, WASM, JPG, NES, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), pdf.Format().String())
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}

func zipWrap(z *Zip, f2 FormatChecker) ([]io.Reader, error) {
	switch f2.Format() {
	case PDF, LITERAL:
		return tryCombining(z, f2.(Parasite))
	case ZIP:
		return nil, errors.New("failed to merge two files of the same type")
	case WASM, PNG, JPG, NES, GIF, MP3:
		return nil, fmt.Errorf("%s requires offset at 0 can't attach or inject into %s", f2.Format().String(), z.Format().String())
	default:
		return nil, errors.New("unknown fileformat for f2")
	}
}
