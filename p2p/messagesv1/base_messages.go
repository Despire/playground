package messagesv1

import (
	"encoding/binary"
	"fmt"
)

type KeepAlive struct{}

func (k KeepAlive) Serialize() []byte {
	return []byte{
		0x0,
		0x0,
		0x0,
		0x0,
	}
}

type Choke struct{}

func (c Choke) Serialize() []byte {
	var msg [5]byte

	binary.BigEndian.PutUint32(msg[:4], 1)

	msg[4] = byte(ChokeType)

	return msg[:]
}

type Unchoke struct{}

func (u Unchoke) Serialize() []byte {
	var msg [5]byte

	binary.BigEndian.PutUint32(msg[:4], 1)

	msg[4] = byte(UnChokeType)

	return msg[:]
}

type Interest struct{}

func (i Interest) Serialize() []byte {
	var msg [5]byte

	binary.BigEndian.PutUint32(msg[:4], 1)

	msg[4] = byte(InterestType)

	return msg[:]
}

type NotInterest struct{}

func (n NotInterest) Serialize() []byte {
	var msg [5]byte

	binary.BigEndian.PutUint32(msg[:4], 1)

	msg[4] = byte(NotInterestType)

	return msg[:]
}

type Have struct {
	Index uint32
}

func (h *Have) Serialize() []byte {
	var msg [4 + 1 + 4]byte // 4 for the length, 1 for id, 4 for index

	binary.BigEndian.PutUint32(msg[:4], 1+4)
	binary.BigEndian.PutUint32(msg[5:], h.Index)
	msg[4] = byte(HaveType)

	return msg[:]
}

func (h *Have) Deserialize(b []byte) error {
	if len(b) != 4 {
		return fmt.Errorf("invalid payload length")
	}

	h.Index = binary.BigEndian.Uint32(b)
	return nil
}
