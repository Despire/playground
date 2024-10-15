package torrent

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestFrom(t *testing.T) {
	type args struct {
		bencoded io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    *MetaInfoFile
		wantErr bool
	}{
		{
			name: "pass",
			args: args{
				bencoded: func() io.Reader {
					b, err := os.ReadFile("./test_data/debian.torrent")
					if err != nil {
						t.Fatal(err)
					}
					return bytes.NewBuffer(b)
				}(),
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := From(tt.args.bencoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("From() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("From() got = %v, want %v", got, tt.want)
			}
		})
	}
}
