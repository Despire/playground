package tracker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Despire/tinytorrent/bencoding"
)

type Response struct {
	// Indicating what went wrong. If present no other keys may be present.
	FailureReason *string
	// Similar to FailureReason but response is valid.
	WarningMessage *string
	// Interval in seconds that the client should wait between sending
	// regular requests to the tracker.
	Interval *int64
	// Minimum announce interval. If present clients must not reannounce more
	// frequently than this.
	MinInterval *int64
	// ID that the client should send back on its next announcements
	// to the tracker. If the value is absent and it was received
	// by a previous response from the tracker that same value should
	// be re-used and not discarded.
	TrackerID *string
	// Number of peers with entire file (seeders).
	Complete *int64
	// Number of peers participating in the file (leechers).
	Incomplete *int64
	// Peers for the file.
	Peers []struct {
		PeerID string
		IP     string
		Port   int64
	}
}

func DecodeResponse(src io.Reader, out *Response) error {
	if out == nil {
		panic("no response to fill, pased <nil>")
	}

	resp, err := bencoding.Decode(src)
	if err != nil {
		return fmt.Errorf("failed to decode body: %w", err)
	}
	if typ := resp.Type(); typ != bencoding.DictionaryType {
		return fmt.Errorf("expected response to be of type dictionary but got %v", typ)
	}

	dict := resp.(*bencoding.Dictionary).Dict

	if fr := dict["failure reason"]; fr != nil {
		l, ok := fr.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected failure_reason to be of type Bytestring but was %T: ", fr)
		}
		out.FailureReason = (*string)(l)
		return nil // no other fields will be set.
	}

	if wm := dict["warning message"]; wm != nil {
		l, ok := wm.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected warning_message to be of type Bytestring but was %T: ", wm)
		}
		out.WarningMessage = (*string)(l)
	}

	if i := dict["interval"]; i != nil {
		l, ok := i.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected interval to be of type Integer but was %T: ", i)
		}
		out.Interval = (*int64)(l)
	}

	if mi := dict["min interval"]; mi != nil {
		l, ok := mi.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected min_interval to be of type Integer but was %T: ", mi)
		}
		out.MinInterval = (*int64)(l)
	}

	if ti := dict["tracker id"]; ti != nil {
		l, ok := ti.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected tracker_id to be of type Bytestring but was %T: ", ti)
		}
		out.TrackerID = (*string)(l)
	}

	if c := dict["complete"]; c != nil {
		l, ok := c.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected complete to be of type Integer but was %T: ", c)
		}
		out.Complete = (*int64)(l)
	}
	if inc := dict["incomplete"]; inc != nil {
		l, ok := inc.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected incomplete to be of type Integer but was %T: ", inc)
		}
		out.Incomplete = (*int64)(l)
	}

	if peers := dict["peers"]; peers != nil {
		switch peers.Type() {
		case bencoding.ListType: // non-compact
			wide := peers.(*bencoding.List)
			for _, peer := range *wide {
				peer, ok := peer.(*bencoding.Dictionary)
				if !ok {
					return fmt.Errorf("expected peer to be of type Dictionary but was %T: ", peer)
				}

				var peerData struct {
					PeerID string
					IP     string
					Port   int64
				}

				if id := peer.Dict["peer id"]; id != nil {
					l, ok := id.(*bencoding.ByteString)
					if !ok {
						return fmt.Errorf("expected peer_id to be of type Bytestring but was %T: ", id)
					}
					peerData.PeerID = string(*l)
				}

				ip := peer.Dict["ip"]
				if ip == nil {
					return errors.New("no ip listed for peer, inside peers list")
				}
				l, ok := ip.(*bencoding.ByteString)
				if !ok {
					return fmt.Errorf("expected peer_ip to be of type Bytestring but was %T: ", ip)
				}

				port := peer.Dict["port"]
				if port == nil {
					return fmt.Errorf("no port listed for peer, inside peers list")
				}
				p, ok := port.(*bencoding.Integer)
				if !ok {
					return fmt.Errorf("expected peer_port to be of type Integer but was %T: ", port)
				}

				peerData.IP = string(*l)
				peerData.Port = int64(*p)

				out.Peers = append(out.Peers, peerData)
			}

		case bencoding.ByteStringType: // compact
			compact := []byte(*(*string)(peers.(*bencoding.ByteString)))
			if len(compact)%6 != 0 {
				return fmt.Errorf("expected length of compact to be a multiple of 6 but got %v", len(compact))
			}
			for i := 0; i < len(compact); i += 6 {
				peer := compact[i : i+6]

				var peerData struct {
					PeerID string
					IP     string
					Port   int64
				}

				peerData.IP = net.IP(peer[:4]).String()
				peerData.Port = int64(binary.BigEndian.Uint16(peer[4:]))

				out.Peers = append(out.Peers, peerData)

			}
		default:
			return fmt.Errorf("peers were nor dictionary or bytestring type, got %T", peers)
		}
	}

	return nil
}
