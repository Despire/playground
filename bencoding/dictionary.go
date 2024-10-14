package bencoding

import (
	"fmt"
	"reflect"
	"strings"
)

// Dictionary is a grouping of Bencoded values indexable by keys.
type Dictionary struct {
	Dict map[string]Value
}

func (d *Dictionary) Type() Type { return DictionaryType }

func (d *Dictionary) Literal() string {
	b := &strings.Builder{}

	b.WriteByte(byte(dictionaryBegin))
	if d != nil {
		for k, v := range d.Dict {
			b.WriteString(fmt.Sprintf("%d:%s", len(k), k))
			b.WriteString(v.Literal())
		}
	}
	b.WriteByte(byte(valueEnd))

	return b.String()
}

func (d *Dictionary) equal(o Value) bool {
	if d.Type() != o.Type() {
		return false
	}

	other := o.(*Dictionary)
	if len(other.Dict) != len(d.Dict) {
		return false
	}

	for k, v := range d.Dict {
		o, exists := (other.Dict)[k]
		if !exists {
			return false
		}
		if o.Type() != v.Type() && o.Literal() != v.Literal() {
			return false
		}
	}

	return true
}

func (d *Dictionary) Decode(src []byte, position int) (int, error) {
	if d.Dict == nil {
		d.Dict = make(map[string]Value)
	}

	if src[position] != byte(dictionaryBegin) {
		return 0, &DecodingError{
			typ: reflect.TypeOf(*d),
			msg: "failed to decode dictionary, missing 'd' indicating start of dictionary",
		}
	}

	for {
		position += 1

		if src[position] == byte(valueEnd) {
			return position, nil
		}

		k := new(ByteString)
		var err error
		position, err = k.Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "failed to decode dictionary Key: " + err.Error(),
			}
		}

		position += 1

		var v Decoder
		switch src[position] {
		case byte(listBegin):
			v = &List{}
		case byte(dictionaryBegin):
			v = &Dictionary{}
		case byte(integerBegin):
			v = new(Integer)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			v = new(ByteString)
		default:
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "expected value, found unrecognized token: " + string(src[position]),
			}
		}
		position, err = v.Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "failed to decode list item of type '" + reflect.TypeOf(d).String() + "': " + err.Error(),
			}
		}
		d.Dict[string(*k)] = v.(Value)
	}
}
