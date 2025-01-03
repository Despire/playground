package messagesv1

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageType represents the known BitTorrent message types.
//
//go:generate stringer -type=MessageType
type MessageType int8

const (
	KeepAliveType MessageType = iota - 1
	ChokeType
	UnChokeType
	InterestType
	NotInterestType
	HaveType
	BitfieldType
	RequestType
	PieceType
	CancelType
	PortType
)

type Message struct {
	Type    MessageType
	Payload []byte
}

func Identify(reader io.Reader) (*Message, error) {
	var length [4]byte

	r, err := io.ReadFull(reader, length[:])
	if err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	if r != len(length) {
		return nil, fmt.Errorf("failed to fully read message length: expected %d, got %d", len(length), r)
	}

	l := binary.BigEndian.Uint32(length[:])
	if l == 0 {
		return &Message{Type: KeepAliveType}, nil
	}

	var messageID [1]byte
	r, err = io.ReadFull(reader, messageID[:])
	if err != nil {
		return nil, fmt.Errorf("failed to read message ID: %w", err)
	}
	if r != len(messageID) {
		return nil, fmt.Errorf("failed to fully read message ID: expected %d, got %d", len(messageID), r)
	}

	l -= 1

	payload := make([]byte, l)
	r, err = io.ReadFull(reader, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to read message payload: %w", err)
	}
	if r != int(l) {
		return nil, fmt.Errorf("failed to fully read message payload: expected %d, got %d", l, r)
	}

	switch typ := MessageType(messageID[0]); typ {
	case ChokeType, UnChokeType, InterestType, NotInterestType:
		return &Message{Type: typ}, nil
	case HaveType, BitfieldType, RequestType, PieceType, CancelType, PortType:
		return &Message{Type: typ, Payload: payload}, nil
	default:
		return nil, fmt.Errorf("unknown message id: %v", messageID[0])
	}
}
