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

type Bitfield struct {
	Bitfield []byte
}

func (b *Bitfield) Serialize() []byte {
	msg := make([]byte, 4+1+len(b.Bitfield))

	binary.BigEndian.PutUint32(msg[:4], 1+uint32(len(b.Bitfield)))
	msg[4] = byte(BitfieldType)
	copy(msg[5:], b.Bitfield)

	return msg
}

func (b *Bitfield) Deserialize(payload []byte) error {
	b.Bitfield = payload
	return nil
}

type Port struct {
	Port uint16
}

func (p *Port) Serialize() []byte {
	var msg [4 + 1 + 2]byte

	binary.BigEndian.PutUint32(msg[:4], 1+2)
	msg[4] = byte(PortType)
	binary.BigEndian.PutUint16(msg[5:7], p.Port)

	return msg[:]
}

func (p *Port) Deserialize(msg []byte) error {
	if len(msg) != 2 {
		return fmt.Errorf("invalid payload length")
	}

	p.Port = binary.BigEndian.Uint16(msg[:2])
	return nil
}
