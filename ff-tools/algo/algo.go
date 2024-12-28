package algo

import "bytes"

func ToLEB128(x uint64) []byte {
	out := new(bytes.Buffer)
	for {
		b := byte(x & 0x7f) // take 7 lower order bits.
		x >>= 7             // shift 7 bits.

		if x == 0 {
			out.WriteByte(b)
			break
		}
		out.WriteByte(b | 0x80)
	}

	return out.Bytes()
}
