package formats

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"unicode"
)

const template = `%%PDF-1.3
1 0 obj
<</Length 2 0 R>>
stream
%v
endstream
endobj
2 0 obj
%v
endobj
3 0 obj
<<
  /Payload 1 0 R
  /Pages 4 0 R
  /Type /Catalog
>>
endobj
4 0 obj
<<
  /Count %v
  /Kids[%v]
  /Type /Pages
>>
endobj`

type Pdf struct{ contents []byte }

func NewPdf(contents []byte) (*Pdf, error) {
	if bytes.HasPrefix(contents, PdfHeaderStart) {
		return &Pdf{
			contents: contents,
		}, nil
	}

	return nil, errors.New("failed to parse as pdf")
}

func (p *Pdf) Format() FileFormat { return PDF }
func (p *Pdf) IsParasite() bool   { return true }
func (p *Pdf) Reader() io.Reader  { return bytes.NewReader(p.contents) }

func (p *Pdf) unwrap(b []byte, beg, end []byte) []byte {
	i := bytes.Index(b, beg)
	if i < 0 {
		return nil
	}

	s := i + len(beg)
	e := bytes.Index(b[s:], end)
	if e < 0 {
		return nil
	}

	return b[s : s+e]
}

func (p *Pdf) unwrapObject(b []byte, start []byte) []byte {
	objectNumber := bytes.Trim(p.unwrap(b, start, []byte("0 R")), " \t\r\n")
	return []byte(fmt.Sprintf("%s %s 0 R", string(start), string(objectNumber)))
}

func (p *Pdf) fixXref(b []byte) []byte {
	i := bytes.Index(b, []byte("\nxref\n0 "))
	if i < 0 {
		panic("invalid pdf")
	}

	ll := len("\nxref\n0 ")
	eol := bytes.Index(b[i+ll:], []byte("\n"))

	// including the 0th object.
	objCount, err := strconv.Atoi(string(b[i+ll : i+ll+eol]))
	if err != nil {
		panic("invalid object count inside xref table")
	}

	offsets := make([]int, 0, objCount-1)

	// we need to find each object and fix up the offset in the xref table.
	// skip object 0
	for i := 1; i < objCount; i++ {
		offset := bytes.Index(b, []byte(fmt.Sprintf("%d 0 obj", i)))
		if offset < 0 {
			continue
			//panic("couldn't find object defined inside xref table")
		}

		offsets = append(offsets, offset)
	}

	// get the beginning of the first object (skipping the 0th)
	step := len("0000000000 65535 f \n")
	start := i + ll + eol + 1 + step

	for i := 0; i < len(offsets); i++ {
		offset := fmt.Sprintf("%d", offsets[i])
		for len(offset) < 10 {
			offset = string(append([]byte("0"), offset...))
		}

		offset += " 00000 n"

		// overwrite contents
		copy(b[start:], offset)
		start += step
	}

	// fix the offset of the startxref
	xref := bytes.Index(b, []byte("\nxref\n0 ")) + len("\nxref\n")

	startXref := bytes.Index(b, []byte("\nstartxref\n")) + len("\nstartxref\n")
	eol = bytes.Index(b[startXref:], []byte("\n"))

	buff := make([]byte, len(b[startXref+eol+1:]))
	copy(buff, b[startXref+eol+1:])

	b = b[:startXref]
	b = append(b, strconv.Itoa(xref)...)
	b = append(b, "\n"...)

	for _, line := range bytes.Split(buff, []byte("\n")) {
		b = append(b, line...)
		b = append(b, "\n"...)
	}

	return b[:len(b)-1] // ignore last new line.
}

func (p *Pdf) sanitize(b []byte) []byte {
	sanitized := make([]byte, 0, len(b))

	stream := false
	for _, line := range bytes.Split(b, []byte("\n")) {
		if bytes.Contains(line, []byte("stream")) {
			stream = true
		}

		if bytes.Contains(line, []byte("endstream")) {
			stream = false
		}

		if stream {
			sanitized = append(sanitized, line...)
			sanitized = append(sanitized, "\n"...)
			continue
		}

		var i int
		tmp := make([]byte, 0, len(line))

		for {
			i = bytes.Index(line, []byte("<<"))
			if i < 0 {
				break
			}
			tmp = append(tmp, line[:i]...)
			tmp = append(tmp, "\n"...)
			tmp = append(tmp, "<<\n"...)
			line = line[i+len("<<"):]
		}

		for {
			i = bytes.Index(line, []byte(">>"))
			if i < 0 {
				tmp = append(tmp, line...)
				tmp = append(tmp, "\n"...)
				break
			}
			tmp = append(tmp, line[:i]...)
			tmp = append(tmp, "\n"...)
			tmp = append(tmp, ">>\n"...)
			line = line[i+len(">>"):]
		}

		sanitized = append(sanitized, tmp...)
	}

	out := make([]byte, 0, len(sanitized))
	for _, line := range bytes.Split(sanitized, []byte("\n")) {
		if len(bytes.Trim(line, " \t\n")) > 0 {
			out = append(out, line...)
			out = append(out, "\n"...)
		}
	}

	return out
}

func (p *Pdf) enclosingObject(b []byte, index int) int {
	i := bytes.LastIndex(b[:index], []byte("<<"))
	if i < 0 {
		panic("invalid pdf format")
	}

	return i
}

func (p *Pdf) getCount(b []byte) (int, error) {
	i := p.unwrap(b, []byte("/Count "), []byte("/"))

	eol := 0
	for unicode.IsNumber(rune(i[eol])) {
		eol++
	}

	return strconv.Atoi(string(i[:eol]))
}

func (p *Pdf) getKids(b []byte) ([]byte, error) {
	i := bytes.Index(b, []byte("/Kids"))
	if i < 0 {
		return nil, errors.New("failed to find /Kids inside the /Type /Pages object of the contents of the pdf")
	}

	for b[i] != '[' {
		i++
	}

	return p.unwrap(b[i:], []byte("["), []byte("]")), nil
}

func (p *Pdf) mergeTemplate(file Parasite, objCount int) ([]byte, error) {
	parasiteBytes, err := io.ReadAll(file.Reader())
	if err != nil {
		return nil, err
	}

	i := bytes.Index(p.contents, []byte("/Type /Pages"))
	if i < 0 {
		return nil, errors.New("failed to find /Type /Pages")
	}

	i = p.enclosingObject(p.contents, i)
	count, err := p.getCount(p.contents[i:])
	if err != nil {
		return nil, err
	}

	kids, err := p.getKids(p.contents[i:])
	if err != nil {
		return nil, err
	}

	kids = bytes.Trim(kids, " \t\n")
	kids = bytes.Join(bytes.Split(kids, []byte("\n")), []byte(" "))

	var newKids []byte
	// since we added 2 objects we need to increase the count.
	for _, line := range bytes.Split(kids, []byte("0 R")) {
		if len(line) < 1 {
			continue
		}

		obj, err := strconv.Atoi(string(bytes.Trim(line, " \t")))
		if err != nil {
			return nil, err
		}

		obj += objCount
		newKids = append(newKids, []byte(fmt.Sprintf(" %d 0 R ", obj))...)
	}

	// TODO: check if string(parasiteBytes) work!!!
	return []byte(fmt.Sprintf(template, string(parasiteBytes), len(parasiteBytes), count, string(newKids))), nil
}

func (p *Pdf) removeObjects(objs ...string) error {
	for _, obj := range objs {
		i := bytes.Index(p.contents, []byte(obj))
		if i < 0 {
			return fmt.Errorf("failed to find object %s", obj)
		}

		enclosingObject := p.enclosingObject(p.contents, i)
		start := bytes.LastIndex(p.contents[:enclosingObject-1], []byte("\n")) + 1
		end := enclosingObject + bytes.Index(p.contents[enclosingObject:], []byte("endobj")) + len("endobj")

		tmp := make([]byte, len(p.contents))
		copy(tmp[:start], p.contents[:start])
		copy(tmp[start:], p.contents[end:])

		p.contents = tmp[:len(tmp)-(end-start)]
	}

	return nil
}

func (p *Pdf) removeOldHeader() error {
	i := bytes.Index(p.contents, []byte("obj"))
	if i < 0 {
		return errors.New("failed to find obj malformed pdf")
	}

	start := bytes.LastIndex(p.contents[:i], []byte("\n")) + 1
	p.contents = p.contents[start:]
	return nil
}

func (p *Pdf) increaseObjCount(count int) error {
	tmp := make([]byte, 0, len(p.contents))

	var reg = regexp.MustCompile("([0-9]*) 0 obj")
	for _, line := range bytes.Split(p.contents, []byte("\n")) {
		if bytes.Contains(line, []byte(" obj")) {
			num, err := strconv.Atoi(string(reg.FindSubmatch(line)[1]))
			if err != nil {
				return err
			}

			num += count
			tmp = append(tmp, fmt.Sprintf("%d 0 obj\n", num)...)
			continue
		}

		tmp = append(tmp, line...)
		tmp = append(tmp, "\n"...)
	}

	p.contents = tmp
	return nil
}

func (p *Pdf) increaseObjectReferences(count int) error {
	tmp := make([]byte, 0, len(p.contents))

	for _, line := range bytes.Split(p.contents, []byte("\n")) {
		if bytes.Contains(line, []byte(" 0 R")) {
			parts := bytes.Split(line, []byte(" 0 R"))
			for i := 0; i < len(parts)-1; i++ {
				elem := bytes.Split(parts[i], []byte(" "))
				num, err := strconv.Atoi(string(elem[len(elem)-1]))
				if err != nil {
					return err
				}

				num += count

				index := 1
				for len(parts[i])-index >= 0 && unicode.IsNumber(rune(parts[i][len(parts[i])-index])) {
					index++
				}
				index -= 1

				parts[i] = append(parts[i][:len(parts[i])-index], fmt.Sprintf("%d", num)...)
			}

			tmp = append(tmp, bytes.Join(parts, []byte(" 0 R"))...)
			continue
		}

		tmp = append(tmp, line...)
		tmp = append(tmp, "\n"...)
	}

	p.contents = tmp
	return nil
}

func (p *Pdf) addObjectCount(count int) error {
	t := bytes.Index(p.contents, []byte("trailer\n"))
	trailer := make([]byte, len(p.contents)-t)
	copy(trailer, p.contents[t:])

	i := bytes.Index(p.contents, []byte("\nxref\n0 ")) + len("\nxref\n0 ")
	eol := bytes.Index(p.contents[i:], []byte("\n"))

	tmp := make([]byte, i)
	copy(tmp, p.contents[:i])

	objCount, err := strconv.Atoi(string(p.contents[i : i+eol]))
	if err != nil {
		return err
	}

	objCount += count
	tmp = append(tmp, fmt.Sprintf("%d\n", objCount)...)
	tmp = append(tmp, fmt.Sprintf("0000000000 65535 f \n")...)
	for i := 1; i < objCount; i++ {
		tmp = append(tmp, fmt.Sprintf("0000000000 00000 n \n")...)
	}

	trailer = trailer[:len("trailer\n")]
	trailer = append(trailer, "<<\n"...)
	trailer = append(trailer, fmt.Sprintf(" /Root 3 0 R /Size %d\n", objCount)...)
	trailer = append(trailer, ">>\n"...)
	trailer = append(trailer, "startxref\n"...)
	trailer = append(trailer, "0\n"...)
	trailer = append(trailer, "%%EOF"...)

	tmp = append(tmp, trailer...)
	p.contents = tmp

	return nil
}

func (p *Pdf) Infect(file Parasite) ([]byte, error) {
	template, err := p.mergeTemplate(file, 4)
	if err != nil {
		return nil, err
	}

	if err := p.removeObjects("/Type /Pages", "/Type /Catalog"); err != nil {
		return nil, err
	}

	if err := p.removeOldHeader(); err != nil {
		return nil, err
	}

	if err := p.increaseObjCount(4); err != nil {
		return nil, err
	}

	if err := p.increaseObjectReferences(4); err != nil {
		return nil, err
	}

	if err := p.addObjectCount(2); err != nil {
		return nil, err
	}

	template = append(template, "\n"...)
	p.contents = append(template, p.contents...)
	// todo: fix xref
	p.contents = p.fixXref(p.contents)

	return p.contents, nil
}

func (p *Pdf) Attach(reader io.Reader) ([]byte, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return append(append(p.contents, "\n"...), b...), nil
}
