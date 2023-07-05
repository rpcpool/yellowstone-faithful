package manifest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "test_manifest")
	m, err := NewManifest(fp)
	require.NoError(t, err)
	defer m.Close()
	require.NotNil(t, m)

	size, err := m.Size()
	require.NoError(t, err)
	require.Equal(t, int64(0), size)
	{
		err := m.Put(111, 222)
		require.NoError(t, err)
		size, err := m.Size()
		require.NoError(t, err)
		require.Equal(t, int64(16), size)
	}
	{
		err := m.Put(333, 444)
		require.NoError(t, err)
		size, err := m.Size()
		require.NoError(t, err)
		require.Equal(t, int64(32), size)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, [][2]uint64{
			{111, 222},
			{333, 444},
		}, all)
	}
	{
		// now close and reopen
		m.Close()
		m, err = NewManifest(fp)
		require.NoError(t, err)
		defer m.Close()
		require.NotNil(t, m)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, [][2]uint64{
			{111, 222},
			{333, 444},
		}, all)
	}
	{
		err := m.Put(555, 666)
		require.NoError(t, err)
		size, err := m.Size()
		require.NoError(t, err)
		require.Equal(t, int64(48), size)
	}
	{
		all, err := m.ReadAll()
		require.NoError(t, err)
		require.Equal(t, [][2]uint64{
			{111, 222},
			{333, 444},
			{555, 666},
		}, all)
	}
}
