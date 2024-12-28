package status

import (
	"os"
	"testing"

	"github.com/Despire/tinytorrent/p2p/messagesv1"
	"github.com/stretchr/testify/assert"
)

func TestTracker_FlushRead(t *testing.T) {
	downloadDir := ".testing"
	err := os.Mkdir(downloadDir, os.ModePerm)
	assert.Nil(t, err)

	t.Cleanup(func() {
		os.RemoveAll(downloadDir)
	})

	tr := &Tracker{DownloadDir: downloadDir}

	err = tr.Flush(0, []byte{0x0, 0x1})
	assert.Nil(t, err)

	b, err := tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  0,
		Length: 0,
	})
	assert.Nil(t, err)
	assert.Empty(t, b)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  0,
		Length: 1,
	})
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x0}, b)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  0,
		Length: 2,
	})
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x0, 0x1}, b)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  0,
		Length: 3,
	})
	assert.NotNil(t, err)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  1,
		Length: 0,
	})
	assert.Nil(t, err)
	assert.Empty(t, b)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  1,
		Length: 1,
	})
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x1}, b)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  1,
		Length: 2,
	})
	assert.NotNil(t, err)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  1,
		Length: 3,
	})
	assert.NotNil(t, err)

	b, err = tr.ReadRequest(&messagesv1.Request{
		Index:  0,
		Begin:  2,
		Length: 0,
	})
	assert.NotNil(t, err)
}
