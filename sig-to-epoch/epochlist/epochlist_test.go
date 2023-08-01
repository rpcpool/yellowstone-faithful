package epochlist

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "test_manifest")
	m, err := New(fp)
	require.NoError(t, err)
	defer m.Close()
	require.NotNil(t, m)

	size, err := m.ContentSizeBytes()
	require.NoError(t, err)
	require.Equal(t, int64(0), size)
	{
		err := m.Put(111)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(2), size)
	}
	{
		err := m.Put(333)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(4), size)
	}
	{
		all, err := m.Load()
		require.NoError(t, err)
		require.Equal(t, Values{
			111,
			333,
		}, all)
	}
	{
		// now close and reopen
		m.Close()
		m, err = New(fp)
		require.NoError(t, err)
		defer m.Close()
		require.NotNil(t, m)
	}
	{
		all, err := m.Load()
		require.NoError(t, err)
		require.Equal(t, Values{
			111,
			333,
		}, all)
	}
	{
		err := m.Put(555)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(6), size)
	}
	{
		err := m.Put(222)
		require.NoError(t, err)
		size, err := m.ContentSizeBytes()
		require.NoError(t, err)
		require.Equal(t, int64(8), size)
	}
	{
		all, err := m.Load()
		require.NoError(t, err)
		require.Equal(t, Values{
			111,
			333,
			555,
			222,
		}, all)

		all.Sort()
		require.Equal(t, Values{
			111,
			222,
			333,
			555,
		}, all)
	}
}
