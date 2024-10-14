package bencoding

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

func (i *Integer) equal(o Value) bool {
	if i.Type() != o.Type() {
		return false
	}
	other := o.(*Integer)
	return *other == *i
}

func (Integer) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(Integer(r.Intn(1000) + 1))
}

func Test_IntegerEncodeDecode(t *testing.T) {
	property := func(i Integer) bool {
		n := new(Integer)
		e := i.Literal()
		pos, err := n.Decode([]byte(e), 0)
		return err == nil && n.equal(&i) && pos == len(e)-1
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("integer encode -> decode failed: %v", err)
	}
}

func Test_IntegerDecodeErrors(t *testing.T) {
	i := new(Integer)
	pos, err := i.Decode([]byte("0e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Integer: failed to parse integer, 'i' not found", err.Error())

	pos, err = i.Decode([]byte("i0"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Integer: failed to parse integer, 'e' not found", err.Error())

	pos, err = i.Decode([]byte("i023e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Integer: invalid integer, cannot have an integer format prefixed with 0 (i0xxxxx...e)", err.Error())

	pos, err = i.Decode([]byte("i0e"), 0)
	assert.Nil(t, err)
	assert.Equal(t, Integer(0), *i)

	pos, err = i.Decode([]byte("i23e"), 0)
	assert.Nil(t, err)
	assert.Equal(t, Integer(23), *i)
}
