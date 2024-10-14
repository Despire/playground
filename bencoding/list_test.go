package bencoding

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

func (List) Generate(r *rand.Rand, size int) reflect.Value {
	var l List

	for range r.Intn(15) {
		var v Value
		switch r.Intn(3) {
		case 0:
			n := new(Integer).Generate(r, size).Interface().(Integer)
			v = &n
		case 1:
			n := new(ByteString).Generate(r, size).Interface().(ByteString)
			v = &n
		case 2:
			q := List{}
			for range r.Intn(3) {
				var k Value
				switch r.Intn(3) {
				case 0:
					n := new(Integer).Generate(r, size).Interface().(Integer)
					k = &n
				case 1:
					n := new(ByteString).Generate(r, size).Interface().(ByteString)
					k = &n
				case 2:
					n := new(List).Generate(r, size).Interface().(List)
					k = &n
				}
				q = append(q, k)
			}
			v = &q
		}
		l = append(l, v)
	}

	return reflect.ValueOf(l)
}

func Test_ListEncodeDecode(t *testing.T) {
	property := func(l List) bool {
		n := new(List)
		e := l.Literal()
		pos, err := n.Decode([]byte(e), 0)
		return err == nil && n.equal(&l) && pos == len(e)-1
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("list encode -> decode failed: %v", err)
	}
}

func Test_ListDecodingErrors(t *testing.T) {
	pos, err := new(List).Decode([]byte("e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.List: failed to decode list, missing 'l' indicating start of list", err.Error())

	pos, err = new(List).Decode([]byte("l*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.List: unrecognized token: *", err.Error())

	pos, err = new(List).Decode([]byte("l0:*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.List: unrecognized token: *", err.Error())

	pos, err = new(List).Decode([]byte("l0*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.List: failed to decode list item of type '*bencoding.ByteString': Failed to decode bencoding.ByteString: expected separator ':' while parsing string, but did not found", err.Error())

	pos, err = new(List).Decode([]byte("l59:qKMSbDTqRpgfIzWxYVREfqBFUydVVFcBaVnkoROEkDcSUNcaIbCOUxPLOAti182ei218ei453ei352e62:qPQSEKHjusCUzhBELRTKrZrcXLFUhRnFOXPNDWcMqGRhVFMGmPbtKNpkTlRGKMi11ei207e90:DtSNpwxBDUWVkVZeJHEeEyiSDjrRZPkOadEXinjglMCyzyHptaRKuKgNQidFbQaaudiLPEMclEadFXlVvcffQremcDi321ei413ei433ee"), 0)
	assert.Nil(t, err)
	assert.Equal(t, 265, pos)

	pos, err = new(List).Decode([]byte("l60:RtlYPbXvkJeFfgGWFnAeyNvhjobhLuGViGngTxEfTbafgtJZNUWOhTZARKrFi369elee"), 0)
	assert.Nil(t, err)
	assert.Equal(t, 71, pos)
}

func Test_ListEqual(t *testing.T) {
	l1 := List{}
	l2 := List{}

	assert.True(t, l1.equal(&l2))

	l1 = List{
		ptrOf(Integer(123)),
		&List{
			&List{
				ptrOf(Integer(123)),
				ptrOf(ByteString("1:t")),
			},
		},
		&List{},
	}
	l2 = List{
		ptrOf(Integer(123)),
		&List{
			&List{
				ptrOf(Integer(123)),
				ptrOf(ByteString("1:t")),
			},
		},
		&List{},
	}
	assert.True(t, l1.equal(&l2))

}

func ptrOf[T any](t T) *T { return &t }
