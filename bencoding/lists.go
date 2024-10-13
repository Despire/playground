package bencoding

import (
	"reflect"
)

// List is a grouping of Bencoded values.
type List []Value

func (l *List) Equal(o Value) bool {
	rv := reflect.ValueOf(o)
	if rv.Type() != reflect.TypeFor[*List]() {
		return false
	}
	if (rv.IsNil() && l != nil) || (l == nil && !rv.IsNil()) {
		return false
	}
	if l == nil {
		return true
	}

	other := o.(*List)
	if len(*other) != len(*l) {
		return false
	}

	for i := range *l {
		if !(*l)[i].Equal((*other)[i]) {
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

	var err error
	for {
		position += 1

		switch src[position] {
		case byte(valueEnd):
			return position, nil
		case byte(listBegin):
			n := List{}
			position, err = n.Decode(src, position)
			if err != nil {
				return 0, &DecodingError{
					typ: reflect.TypeOf(*l),
					msg: "failed to decode list element of type 'List': " + err.Error(),
				}
			}
			*l = append(*l, &n)
		case byte(dictionaryBegin):
			// TODO: fix me
			panic("fix me")
		case byte(integerBegin):
			n := new(Integer)
			position, err = n.Decode(src, position)
			if err != nil {
				return 0, &DecodingError{
					typ: reflect.TypeOf(*l),
					msg: "failed to decode list element of type 'Integer': " + err.Error(),
				}
			}

			*l = append(*l, n)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			n := new(ByteString)
			position, err = n.Decode(src, position)
			if err != nil {
				return 0, &DecodingError{
					typ: reflect.TypeOf(*l),
					msg: "failed to decode list element of type 'ByteString': " + err.Error(),
				}
			}

			*l = append(*l, n)
		default:
			return 0, &DecodingError{
				typ: reflect.TypeOf(*l),
				msg: "unrecognized token: " + string(src[position]),
			}
		}
	}
}

func (l *List) Encode() []byte {
	buffer := make([]byte, 0, 2)
	buffer = append(buffer, byte(listBegin))
	for _, v := range *l {
		buffer = append(buffer, v.Encode()...)
	}
	buffer = append(buffer, byte(valueEnd))
	return buffer
}
