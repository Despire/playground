package client

import (
	"io"
	"strings"
	"testing"
)

// TODO: fixup tests.
func TestDecodeTrackerResponse(t *testing.T) {
	type args struct {
		src io.Reader
		out *TrackerResponse
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "complete-response-example (Dictionary Model)",
			args: args{
				src: strings.NewReader(`
d13:intervali1800e14:min intervali900e12:tracker id20:exampletrackerid1234567e8:completei42e10:incompletei100e5:peersl
  d2:peer id20:abcdefghij1234567890abcd2:ip13:192.168.1.10porti6881ee
  d2:peer id20:abcdefghij0987654321abcd2:ip13:192.168.1.11porti6882ee
ee
`),
				out: new(TrackerResponse),
			},
			wantErr: false,
		},
		{
			name: "Failure Response",
			args: args{
				src: strings.NewReader(`
d13:failure reason34:Invalid info_hash providede
`),
				out: new(TrackerResponse),
			},
			wantErr: false,
		},
		{
			name: "Warning Message",
			args: args{
				src: strings.NewReader(`
d13:intervali1800e15:warning message32:This is a warning messagee5:peersld2:peer id20:abcdefghij1234567890abcd2:ip13:192.168.1.10porti6881eeed
`),
				out: new(TrackerResponse),
			},
			wantErr: false,
		},
		{
			name: "complete-response-example (Binary Model)",
			args: args{
				src: strings.NewReader(`
d13:intervali1800e12:tracker id20:exampletrackerid1234567e8:completei42e10:incompletei100e5:peers
\xC0\xA8\x01\x0A\x1A\xB0
\xC0\xA8\x01\x0B\x1A\xB1
`),
				out: new(TrackerResponse),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DecodeTrackerResponse(tt.args.src, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("DecodeTrackerResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
