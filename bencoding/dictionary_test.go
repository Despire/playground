package bencoding

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

func (Dictionary) Generate(r *rand.Rand, size int) reflect.Value {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	d := Dictionary{
		Dict: make(map[string]Value),
	}

	for range r.Intn(15) {
		var sbuilder strings.Builder
		for range r.Intn(100) {
			sbuilder.WriteByte(alphabet[r.Intn(len(alphabet))])
		}
		k := sbuilder.String()
		var v Value
		switch r.Intn(4) {
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
		case 3:
			q := Dictionary{
				Dict: make(map[string]Value),
			}
			for range r.Intn(3) {
				var sbuilder strings.Builder
				for range r.Intn(100) {
					sbuilder.WriteByte(alphabet[r.Intn(len(alphabet))])
				}
				lk := sbuilder.String()
				var lv Value
				switch r.Intn(4) {
				case 0:
					n := new(Integer).Generate(r, size).Interface().(Integer)
					lv = &n
				case 1:
					n := new(ByteString).Generate(r, size).Interface().(ByteString)
					lv = &n
				case 2:
					n := new(List).Generate(r, size).Interface().(List)
					lv = &n
				case 3:
					n := new(Dictionary).Generate(r, size).Interface().(Dictionary)
					lv = &n
				}
				q.Dict[lk] = lv
			}
			v = &q
		}
		d.Dict[k] = v
	}

	return reflect.ValueOf(d)
}

func Test_DictionaryEncodeDecode(t *testing.T) {
	property := func(d Dictionary) bool {
		n := new(Dictionary)
		e := d.Literal()
		pos, err := n.Decode([]byte(e), 0)
		return err == nil && n.equal(&d) && pos == len(e)-1
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("dictionary encode -> decode failed: %v", err)
	}
}

func Test_DictionaryDecodeErrors(t *testing.T) {
	pos, err := new(Dictionary).Decode([]byte("e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Dictionary: failed to decode dictionary, missing 'd' indicating start of dictionary", err.Error())

	pos, err = new(Dictionary).Decode([]byte("d*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Dictionary: failed to decode dictionary Key: Failed to decode bencoding.ByteString: expected separator ':' while parsing string, but did not found", err.Error())

	pos, err = new(Dictionary).Decode([]byte("d1:ee"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Dictionary: expected value, found unrecognized token: e", err.Error())

	pos, err = new(Dictionary).Decode([]byte("d0:*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Dictionary: expected value, found unrecognized token: *", err.Error())

	pos, err = new(Dictionary).Decode([]byte("d0*e"), 0)
	assert.NotNil(t, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "Failed to decode bencoding.Dictionary: failed to decode dictionary Key: Failed to decode bencoding.ByteString: expected separator ':' while parsing string, but did not found", err.Error())

	pos, err = new(Dictionary).Decode([]byte("d1:ti2ee"), 0)
	assert.Nil(t, err)
	assert.Equal(t, 7, pos)
}
