package indexmeta_test

import (
	"testing"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/require"
)

func TestHeaderMeta(t *testing.T) {
	require.Equal(t, (255), indexmeta.MaxKeySize)
	require.Equal(t, (255), indexmeta.MaxValueSize)
	require.Equal(t, (255), indexmeta.MaxNumKVs)

	var meta indexmeta.Meta
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

	var decoded indexmeta.Meta
	require.NoError(t, decoded.UnmarshalBinary(encoded))

	require.Equal(t, meta, decoded)
}

func concatBytes(bs ...[]byte) []byte {
	var out []byte
	for _, b := range bs {
		out = append(out, b...)
	}
	return out
}
