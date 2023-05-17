package compactindex

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

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
	require.EqualError(t, dbErr, "not a radiance compactindex file")
}

func TestOpen_HeaderOnly(t *testing.T) {
	buf := [32]byte{
		// Magic
		'r', 'd', 'c', 'e', 'c', 'i', 'd', 'x',
		// FileSize
		0x37, 0x13, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// NumBuckets
		0x42, 0x00, 0x00, 0x00,
		// Padding
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	db, dbErr := Open(bytes.NewReader(buf[:]))
	require.NotNil(t, db)
	require.NoError(t, dbErr)

	assert.NotNil(t, db.Stream)
	assert.Equal(t, Header{
		FileSize:   0x1337,
		NumBuckets: 0x42,
	}, db.Header)
}
