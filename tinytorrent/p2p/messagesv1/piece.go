package messagesv1

import (
	"encoding/binary"
	"errors"
)

type Piece struct {
	Index uint32
	Begin uint32
	Block []byte
}

func (p *Piece) Serialize() []byte {
	// Length (4) | id (1) | index (4) | begin (4) | block variable.
	msg := make([]byte, 4+1+4+4+len(p.Block))
	binary.BigEndian.PutUint32(msg[:4], uint32(1+4+4+len(p.Block)))
	msg[4] = byte(PieceType)
	binary.BigEndian.PutUint32(msg[5:9], p.Index)
	binary.BigEndian.PutUint32(msg[9:13], p.Begin)
	copy(msg[13:], p.Block)
	return msg
}

func (p *Piece) Deserialize(msg []byte) error {
	if len(msg) < 9 {
		return errors.New("message too short")
	}
	p.Index = binary.BigEndian.Uint32(msg[:4])
	p.Begin = binary.BigEndian.Uint32(msg[4:8])
	p.Block = msg[8:]
	return nil
}
