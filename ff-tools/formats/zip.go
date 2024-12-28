package formats

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
)

type Zip struct{ contents []byte }

func NewZip(contents []byte) (*Zip, error) {
	if bytes.HasPrefix(contents, ZipHeaderStart) {
		return &Zip{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse as zip")
}

func (z *Zip) Format() FileFormat { return ZIP }

func (z *Zip) IsParasite() bool { return true }

func (z *Zip) Reader() io.Reader { return bytes.NewReader(z.contents) }

func (z *Zip) Infect(file Parasite) ([]byte, error) {
	out := new(bytes.Buffer)

	writer := zip.NewWriter(out)

	header, err := writer.CreateHeader(&zip.FileHeader{
		Name:   ".",
		Method: zip.Store, // don't compress store as raw contents.
	})

	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(header, file.Reader()); err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(z.contents), int64(len(z.contents)))
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if err := writer.Copy(file); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func (z *Zip) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(z.contents, "\n"...), b...), nil
}
