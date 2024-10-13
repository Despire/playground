package bencoding

import "reflect"

// An InvalidMarshalError describes an invalid argument passed to [Marshall].
// (The argument to [Marshal] must be an valid value.)
type MarshalError struct {
	Expected reflect.Type
	Passed   reflect.Type
}
//
func (e *MarshalError) Error() string {
	if e.Passed == nil {
		return "bencoding: Marshall(nil)"
	}
	return "bencoding: Marshall( " + e.Passed.String() + " != " + e.Expected.String() + ")"
}

// An UnmarshalError describes an invalid argument passed to [Unmarshal].
// (The argument to [Unmarshal] must be a non-nil pointer.)
type UnmarshallError struct {
	Type reflect.Type
}

func (e *UnmarshallError) Error() string {
	if e.Type == nil {
		return "bencoding: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "bencoding: Unmarshal(non-pointer " + e.Type.String() + ")"
	}

	return "bencoding: Unmarshal(nil " + e.Type.String() + ")"
}
