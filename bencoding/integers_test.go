package bencoding

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

func (Integer) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(Integer(r.Intn(1000) + 1))
}

func Test_IntegerEncodeDecode(t *testing.T) {
	property := func(i Integer) bool {
		e := new(Integer)
		return e.Decode(i.Encode()) == nil && *e == i
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("integer encode -> decode failed: %v", err)
	}
}

func Test_IntegerDecodeErrors(t *testing.T) {
	i := new(Integer)
	err := i.Decode([]byte("0e"))
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to decode bencoding.Integer: failed to parse integer, 'i' not found", err.Error())

	err = i.Decode([]byte("i0"))
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to decode bencoding.Integer: failed to parse integer, 'e' not found", err.Error())

	err = i.Decode([]byte("i023e"))
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to decode bencoding.Integer: invalid integer, cannot have an integer format prefixed with 0 (i0xxxxx...e)", err.Error())

	err = i.Decode([]byte("i0e"))
	assert.Nil(t, err)
	assert.Equal(t, Integer(0), *i)
}
