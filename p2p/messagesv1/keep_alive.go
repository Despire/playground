package messagesv1

import (
	"encoding/binary"
	"fmt"
)

const KeepAliveLength = 4

type KeepAlive struct{}

func (k *KeepAlive) Serialize() []byte {
	return []byte{
		0x0,
		0x0,
		0x0,
		0x0,
	}
}

func (k *KeepAlive) Deserialize(data []byte) error {
	if len(data) != KeepAliveLength {
		return fmt.Errorf("invalid keep alive data length, expected %d, got %d", KeepAliveLength, data)
	}
	if binary.BigEndian.Uint32(data) == 0 {
		return nil
	}
	return fmt.Errorf("invalid keep alive data: %v", data)
}
