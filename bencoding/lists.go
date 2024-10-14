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

func (l *List) equal(o Value) bool {
	if l.Type() != o.Type() {
		return false
	}

	other := o.(*List)
	if len(*other) != len(*l) {
		return false
	}

	for i := range *l {
		cv := (*l)[i]
		ov := (*other)[i]
		if cv.Type() != ov.Type() || cv.Literal() != ov.Literal() {
			return false
		}
	}

	return true
}

func (l *List) Decode(src []byte, position int) (int, error) {
	if src[position] != byte(listBegin) {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*l),
			msg: "failed to decode list, missing 'l' indicating start of list",
		}
	}

	for {
		position += 1
		var d Decoder

		switch src[position] {
		case byte(valueEnd):
			return position, nil
		case byte(listBegin):
			d = &List{}
		case byte(dictionaryBegin):
			d = &Dictionary{}
		case byte(integerBegin):
			d = new(Integer)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			d = new(ByteString)
		default:
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "unrecognized token: " + string(src[position]),
			}
		}

		var err error
		position, err = d.Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "failed to decode list item of type '" + reflect.TypeOf(d).String() + "': " + err.Error(),
			}
		}
		*l = append(*l, d.(Value))
	}
}
