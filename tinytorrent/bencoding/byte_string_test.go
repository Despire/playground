package bencoding

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

func (s *ByteString) equal(o Value) bool {
	if o.Type() != s.Type() {
		return false
	}
	other := o.(*ByteString)
	return *other == *s
}

func (ByteString) Generate(r *rand.Rand, size int) reflect.Value {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	var sbuilder strings.Builder
	for range r.Intn(100) {
		sbuilder.WriteByte(alphabet[r.Intn(len(alphabet))])
	}

	bs := ByteString(sbuilder.String())
	return reflect.ValueOf(bs)
}

func Test_ByteStringEncodeDecode(t *testing.T) {
	property := func(b ByteString) bool {
		n := new(ByteString)
		e := b.Literal()
		pos, err := n.Decode([]byte(e), 0)
		return err == nil && n.equal(&b) && pos == len(e)-1
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("byte_string encode -> decode failed: %v", err)
	}
}

func Test_ByteStringDecodeErrors(t *testing.T) {
	i := new(ByteString)
	pos, err := i.Decode([]byte("1t"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.ByteString: expected separator ':' while parsing string, but did not found", err.Error())

	pos, err = i.Decode([]byte("t:t"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.ByteString: failed to decode length of the string: strconv.ParseInt: parsing \"t\": invalid syntax", err.Error())

	pos, err = i.Decode([]byte("0:"), 0)
	assert.Nil(t, err)
	assert.Equal(t, 1, pos)

	pos, err = i.Decode([]byte("1:t"), 0)
	assert.Nil(t, err)
	assert.Equal(t, 2, pos)
}
