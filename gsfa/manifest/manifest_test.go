package manifest

import (
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "test_manifest")
	meta := indexmeta.Meta{}
	meta.Add([]byte("epoch"), []byte("test"))
	m, err := NewManifest(fp, meta)
	require.NoError(t, err)
	defer m.Close()
	require.NotNil(t, m)

	size, err := m.ContentSizeBytes()
	require.NoError(t, err)
	require.Equal(t, int64(0), size)
	{
		err := m.Put(111, 222)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(16), size)
	}
	{
		err := m.Put(333, 444)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(32), size)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, Values{
			{111, 222},
			{333, 444},
		}, all)
	}
	{
		// now close and reopen
		m.Close()
		m, err = NewManifest(fp, indexmeta.Meta{})
		require.NoError(t, err)
		defer m.Close()
		require.NotNil(t, m)

		epoch, ok := m.header.meta.Get([]byte("epoch"))
		require.True(t, ok)
		require.Equal(t, []byte("test"), epoch)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, Values{
			{111, 222},
			{333, 444},
		}, all)
	}
	{
		err := m.Put(555, 666)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(48), size)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, Values{
			{111, 222},
			{333, 444},
			{555, 666},
		}, all)
	}
}
