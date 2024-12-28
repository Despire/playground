package messagesv1

import (
	"encoding/binary"
	"errors"
)

type Cancel struct {
	Index  uint32
	Begin  uint32
	Length uint32
}

func (r *Cancel) Serialize() []byte {
	// Length (4) | id (1) | index (4) | begin (4) | length (4)
	var msg [4 + 1 + 4 + 4 + 4]byte

	binary.BigEndian.PutUint32(msg[:4], 1+4+4+4)
	msg[4] = byte(CancelType)
	binary.BigEndian.PutUint32(msg[5:9], r.Index)
	binary.BigEndian.PutUint32(msg[9:13], r.Begin)
	binary.BigEndian.PutUint32(msg[13:17], r.Length)

	return msg[:]
}

func (r *Cancel) Deserialize(data []byte) error {
	if len(data) != 12 {
		return errors.New("wrong length")
	}

	r.Index = binary.BigEndian.Uint32(data[:4])
	r.Begin = binary.BigEndian.Uint32(data[4:8])
	r.Length = binary.BigEndian.Uint32(data[8:12])

	return r.Validate()
}

func (r *Cancel) Validate() error {
	if r.Length > 16*1024 {
		return errors.New("length is not 2^14kb")
	}
	return nil
}
