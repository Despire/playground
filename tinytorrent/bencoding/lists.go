package bencoding

import (
	"reflect"
	"strings"
)

// List is a grouping of Bencoded values.
type List []Value

func (l *List) Type() Type { return ListType }

func (l *List) Literal() string {
	b := &strings.Builder{}

	b.WriteByte(byte(listBegin))
	if l != nil {
		for _, v := range *l {
			b.WriteString(v.Literal())
		}
	}
	b.WriteByte(byte(valueEnd))

	return b.String()
}

func (l *List) Decode(src []byte, position int) (int, error) {
	if src[position] != byte(listBegin) {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*l),
			msg: "failed to decode list, missing 'l' indicating start of list",
		}
	}

	for {
		if position == len(src)-1 {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "un-proper formatted list",
			}
		}
		position += 1

		if src[position] == byte(valueEnd) {
			return position, nil
		}

		d := nextValue(src[position])
		if d == nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "unrecognized token: " + string(src[position]),
			}
		}

		var err error
		position, err = d.(Decoder).Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "failed to decode list item of type '" + reflect.TypeOf(d).String() + "': " + err.Error(),
			}
		}
		*l = append(*l, d.(Value))
	}
}
