package bencoding

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
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
		k := slices.Collect(maps.Keys(d.Dict))
		slices.Sort(k)
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

		var err error
		k := new(ByteString)
		position, err = k.Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "failed to decode dictionary Key: " + err.Error(),
			}
		}

		position += 1
		v := nextValue(src[position])
		if v == nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "expected value, found unrecognized token: " + string(src[position]),
			}
		}
		position, err = v.(Decoder).Decode(src, position)
		if err != nil {
			return 0, &DecodingError{
				typ: reflect.TypeOf(*d),
				msg: "failed to decode list item of type '" + reflect.TypeOf(d).String() + "': " + err.Error(),
			}
		}
		d.Dict[string(*k)] = v
	}
}
