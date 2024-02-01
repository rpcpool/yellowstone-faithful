package compactindexsized

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failReader struct{ err error }

func (rd failReader) ReadAt([]byte, int64) (int, error) {
	return 0, rd.err
}

func TestOpen_ReadFail(t *testing.T) {
	err := errors.New("oh no!")
	db, dbErr := Open(failReader{err})
	require.Nil(t, db)
	require.Same(t, err, dbErr)
}

func TestOpen_InvalidMagic(t *testing.T) {
	var buf [32]byte
	rand.Read(buf[:])
	buf[1] = '.' // make test deterministic

	db, dbErr := Open(bytes.NewReader(buf[:]))
	require.Nil(t, db)
	require.EqualError(t, dbErr, ErrInvalidMagic.Error())
}

func TestOpen_HeaderOnly(t *testing.T) {
	buf := concatBytes(
		// Magic
		[]byte{'c', 'o', 'm', 'p', 'i', 's', 'z', 'd'},
		// header size
		i32tob(30),
		// FileSize
		[]byte{0x37, 0x13, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// NumBuckets
		[]byte{0x42, 0x00, 0x00, 0x00},
		// Version
		[]byte{0x01},

		// Meta: how many key-value pairs
		[]byte{2},

		// First key-value pair
		// Key length
		[]byte{3},
		// Key
		[]byte("foo"),
		// Value length
		[]byte{3},
		// Value
		[]byte("bar"),

		// Second key-value pair
		// Key length
		[]byte{3},
		// Key
		[]byte("foo"),
		// Value length
		[]byte{3},
		// Value
		[]byte("baz"),
	)

	db, dbErr := Open(bytes.NewReader(buf[:]))
	require.NoError(t, dbErr)
	require.NotNil(t, db)

	assert.NotNil(t, db.Stream)
	assert.Equal(t, &Header{
		ValueSize:  0x1337,
		NumBuckets: 0x42,
		Metadata: &indexmeta.Meta{
			KeyVals: []indexmeta.KV{
				{
					Key:   []byte("foo"),
					Value: []byte("bar"),
				},
				{
					Key:   []byte("foo"),
					Value: []byte("baz"),
				},
			},
		},
	}, db.Header)
}
