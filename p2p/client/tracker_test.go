package client

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTrackerResponse(t *testing.T) {
	type args struct {
		src io.Reader
		out *TrackerResponse
	}
	tests := []struct {
		name     string
		args     args
		wantErr  bool
		validate func(t *testing.T, resp *TrackerResponse)
	}{
		{
			name: "complete-response-example (Dictionary Model)",
			args: args{
				src: strings.NewReader(`
		d8:intervali1800e12:min intervali900e10:tracker id2:ab8:completei42e10:incompletei100e5:peersld7:peer id2:q12:ip12:192.168.1.104:porti6881eed7:peer id2:q22:ip12:192.168.1.114:porti6882eeee
		`),
				out: new(TrackerResponse),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *TrackerResponse) {
				assert.NotNil(t, resp.Interval)
				assert.Equal(t, int64(1800), *resp.Interval)

				assert.NotNil(t, resp.MinInterval)
				assert.Equal(t, int64(900), *resp.MinInterval)

				assert.NotNil(t, resp.TrackerID)
				assert.Equal(t, "ab", *resp.TrackerID)

				assert.NotNil(t, resp.Complete)
				assert.Equal(t, int64(42), *resp.Complete)

				assert.NotNil(t, resp.Incomplete)
				assert.Equal(t, int64(100), *resp.Incomplete)

				assert.Equal(t, 2, len(resp.Peers))
				assert.Equal(t, "q1", resp.Peers[0].PeerID)
				assert.Equal(t, "q2", resp.Peers[1].PeerID)
				assert.Equal(t, "192.168.1.10", resp.Peers[0].IP)
				assert.Equal(t, "192.168.1.11", resp.Peers[1].IP)
				assert.Equal(t, int64(6881), resp.Peers[0].Port)
				assert.Equal(t, int64(6882), resp.Peers[1].Port)
			},
		},
		{
			name: "Failure Response",
			args: args{
				src: strings.NewReader(`
		d14:failure reason26:Invalid info_hash providede
		`),
				out: new(TrackerResponse),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *TrackerResponse) {
				assert.Equal(t, "Invalid info_hash provided", *resp.FailureReason)
			},
		},
		{
			name: "Warning Message",
			args: args{
				src: strings.NewReader(`d8:completei15e15:warning message3:abc8:intervali900e5:peersld2:ip12:146.71.73.514:porti51230eeee`),
				out: new(TrackerResponse),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *TrackerResponse) {
				assert.NotNil(t, resp.Complete)
				assert.Equal(t, int64(15), *resp.Complete)

				assert.NotNil(t, resp.WarningMessage)
				assert.Equal(t, "abc", *resp.WarningMessage)

				assert.NotNil(t, resp.Interval)
				assert.Equal(t, int64(900), *resp.Interval)

				assert.Equal(t, 1, len(resp.Peers))
				assert.Equal(t, int64(51230), resp.Peers[0].Port)
				assert.Equal(t, "146.71.73.51", resp.Peers[0].IP)
			},
		},
		{
			name: "complete-response-example (Binary Model)",
			args: args{
				src: bytes.NewReader([]byte{ // d8:intervali900e5:peers6:�GI3�6:peers60:e
					0x64, 0x38, 0x3a, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x69,
					0x39, 0x30, 0x30, 0x65, 0x35, 0x3a, 0x70, 0x65, 0x65, 0x72, 0x73, 0x36,
					0x3a, 0x92, 0x47, 0x49, 0x33, 0xc8, 0x1e, 0x36, 0x3a, 0x70, 0x65, 0x65,
					0x72, 0x73, 0x36, 0x30, 0x3a, 0x65}),
				out: new(TrackerResponse),
			},
			wantErr: false,
			validate: func(t *testing.T, resp *TrackerResponse) {
				assert.NotNil(t, resp.Interval)
				assert.Equal(t, int64(900), *resp.Interval)

				assert.Equal(t, 1, len(resp.Peers))
				assert.Equal(t, int64(51230), resp.Peers[0].Port)
				assert.Equal(t, "146.71.73.51", resp.Peers[0].IP)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := DecodeTrackerResponse(tt.args.src, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("DecodeTrackerResponse() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.validate(t, tt.args.out)
		})
	}
}
