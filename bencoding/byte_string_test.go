package bencoding

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

func (ByteString) Generate(r *rand.Rand, size int) reflect.Value {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	var sbuilder strings.Builder
	for range r.Intn(100) {
		sbuilder.WriteByte(alphabet[r.Intn(len(alphabet))])
	}

	bs := ByteString(sbuilder.String())
	return reflect.ValueOf(bs)
}

func TestEncodeDecode(t *testing.T) {
	property := func(b ByteString) bool {
		e := new(ByteString)
		t.Log(fmt.Sprintf("testing: %s", b))
		return e.decode(b.encode()) == nil && *e == b
	}

	if err := quick.Check(property, nil); err != nil {
		t.Errorf("byte_string encode -> decode failed: %v", err)
	}
}
