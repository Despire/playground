package messagesv1

import "fmt"

// ProtocolV1 is used as part of a handshake with a peer to inform about the
// usage of the V1 protocol of BitTorrent.
const ProtocolV1 = "BitTorrent protocol"

const HandshakeLength = 49 + len(ProtocolV1)

// Handshake is the first message exchanged between peers.
type Handshake struct {
	// String identifier of the protocol.
	Pstr string

	//All current implementations use all zeroes. Each bit in these
	// bytes can be used to change the behavior of the protocol.
	Reserved [8]byte

	// SHA-1 hash of the info key in the metainfo file.
	InfoHash string

	// Unique ID for the client
	PeerID string
}

func (h *Handshake) Serialize() []byte {
	// handshake: <pstrlen><pstr><reserved><info_hash><peer_id>
	msg := []byte{byte(len(h.Pstr))}
	msg = append(msg, h.Pstr...)
	msg = append(msg, h.Reserved[:]...)
	msg = append(msg, h.InfoHash[:]...)
	msg = append(msg, h.PeerID[:]...)

	if len(msg) != HandshakeLength {
		panic("invalid BitTorrent V1 handshake message.")
	}

	return msg
}

func (h *Handshake) Deserialize(data []byte) error {
	if len(data) != HandshakeLength {
		return fmt.Errorf("invalid handshake message length")
	}

	h.Pstr = string(data[1 : 1+int(data[0])])

	rs := 1 + int(data[0])
	re := rs + len(h.Reserved)

	for i, v := range data[rs:re] {
		h.Reserved[i] = v
	}

	hs := re
	he := hs + 20

	h.InfoHash = string(data[hs:he])

	ps := he
	pe := ps + 20

	h.PeerID = string(data[ps:pe])

	return nil
}

func (h *Handshake) Validate() error {
	if h.Pstr != ProtocolV1 {
		return fmt.Errorf("invalid BitTorrent V1 handshake message, unsupported pstr")
	}
	if len(h.InfoHash) != 20 {
		return fmt.Errorf("invalid BitTorrent V1 handshake message, invalid info_hash length")
	}
	if len(h.PeerID) != 20 {
		return fmt.Errorf("invalid BitTorrent V1 handshake message, invalid peer_id length")
	}

	return nil
}
