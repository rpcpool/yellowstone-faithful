package compactindexsized

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeader(t *testing.T) {
	var header Header

	header.ValueSize = 42
	header.NumBuckets = 43

	encoded := header.Bytes()
	{
		mustBeEncoded := concatBytes(
			// magic
			Magic[:],
			// header size
			i32tob(14),
			// value size
			[]byte{42, 0, 0, 0, 0, 0, 0, 0},
			// num buckets
			[]byte{43, 0, 0, 0},
			[]byte{1}, // version
			[]byte{0}, // how many kv pairs
		)
		require.Equal(t, mustBeEncoded, encoded)
	}
}
