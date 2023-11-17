package compactindexsized

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeaderMeta(t *testing.T) {
	require.Equal(t, (255), MaxKeySize)
	require.Equal(t, (255), MaxValueSize)
	require.Equal(t, (255), MaxNumKVs)

	var meta Meta
	require.NoError(t, meta.Add([]byte("foo"), []byte("bar")))
	require.NoError(t, meta.Add([]byte("foo"), []byte("baz")))

	require.Equal(t, 2, meta.Count([]byte("foo")))

	got, ok := meta.Get([]byte("foo"))
	require.True(t, ok)
	require.Equal(t, []byte("bar"), got)

	require.Equal(t, [][]byte{[]byte("bar"), []byte("baz")}, meta.GetAll([]byte("foo")))

	require.Equal(t, [][]byte(nil), meta.GetAll([]byte("bar")))

	got, ok = meta.Get([]byte("bar"))
	require.False(t, ok)
	require.Equal(t, []byte(nil), got)

	require.Equal(t, 0, meta.Count([]byte("bar")))

	encoded, err := meta.MarshalBinary()
	require.NoError(t, err)
	{
		mustBeEncoded := concatBytes(
			[]byte{2}, // number of key-value pairs

			[]byte{3},     // length of key
			[]byte("foo"), // key

			[]byte{3},     // length of value
			[]byte("bar"), // value

			[]byte{3},     // length of key
			[]byte("foo"), // key

			[]byte{3},     // length of value
			[]byte("baz"), // value
		)
		require.Equal(t, mustBeEncoded, encoded)
	}

	var decoded Meta
	require.NoError(t, decoded.UnmarshalBinary(encoded))

	require.Equal(t, meta, decoded)
}

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
