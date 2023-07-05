package store_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/davecgh/go-spew/spew"
	store "github.com/rpcpool/yellowstone-faithful/gsfa/store"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/testutil"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

func initStore(t *testing.T, dir string) (*store.Store, error) {
	indexPath := filepath.Join(dir, "storethehash.index")
	dataPath := filepath.Join(dir, "storethehash.data")
	store, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, store.GCInterval(0))
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store, nil
}

func TestUpdate(t *testing.T) {
	t.Run("when not immutable", func(t *testing.T) {
		tempDir := t.TempDir()
		s, err := initStore(t, tempDir)
		require.NoError(t, err)
		blks := testutil.GenerateEntries(2)

		t.Logf("Putting a new block")
		err = s.Put(blks[0].Key.Bytes(), blks[0].RawValue())
		require.NoError(t, err)
		value, found, err := s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[0].RawValue())

		{
			_, err = s.Primary().Flush()
			require.NoError(t, err)
			require.NoError(t, s.Flush())
			require.NoError(t, s.Primary().Sync())
		}

		t.Logf("Overwrite same key with different value")
		spew.Dump(blks)
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.NoError(t, err)

		{
			_, err = s.Primary().Flush()
			require.NoError(t, err)
			require.NoError(t, s.Flush())
			require.NoError(t, s.Primary().Sync())
		}

		value, found, err = s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, blks[1].RawValue()[0:8], value[0:8], "value should be overwritten")
		require.Equal(t, blks[1].RawValue(), value, "value should be overwritten")
		require.NotEqual(t, blks[0].RawValue(), value, "value should be overwritten")
		{
			it, err := s.Primary().Iter()
			require.NoError(t, err)
			key, value, err := it.Next()
			require.NoError(t, err)
			require.Equal(t, blks[0].Key.Bytes(), key)
			require.Equal(t, blks[1].RawValue(), value)
		}

		t.Logf("Overwrite same key with same value")
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.NoError(t, err)
		value, found, err = s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[1].RawValue())

		s.Flush()

		storeIter := s.NewIterator()
		var count int
		for {
			key, val, err := storeIter.Next()
			if err == io.EOF {
				break
			}
			require.Zero(t, count)
			require.NoError(t, err)
			require.Equal(t, blks[0].Key.Bytes(), key)
			require.Equal(t, blks[1].RawValue(), val)
			count++
		}
	})
}

func TestRemove(t *testing.T) {
	tempDir := t.TempDir()
	s, err := initStore(t, tempDir)
	require.NoError(t, err)
	blks := testutil.GenerateEntries(2)

	t.Logf("Putting blocks")
	err = s.Put(blks[0].Key.Bytes(), blks[0].RawValue())
	require.NoError(t, err)
	err = s.Put(blks[1].Key.Bytes(), blks[1].RawValue())
	require.NoError(t, err)

	t.Logf("Removing the first block")
	removed, err := s.Remove(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.True(t, removed)

	t.Logf("Checking if the block has been removed successfully")
	value, found, err := s.Get(blks[1].Key.Bytes())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, value, blks[1].RawValue())
	_, found, err = s.Get(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.False(t, found)

	t.Logf("Trying to remove non-existing key")
	removed, err = s.Remove(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.False(t, removed)

	s.Flush()

	// Start iterator
	flPath := filepath.Join(tempDir, "storethehash.index.free")
	file, err := os.Open(flPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })

	iter := freelist.NewIterator(file)
	// Check freelist for the only removal. Should be the first position
	blk, err := iter.Next()
	require.Equal(t, blk.Offset, types.Position(0))
	require.NoError(t, err)
	// Check that is the last
	_, err = iter.Next()
	require.EqualError(t, err, io.EOF.Error())
}

func TestTranslate(t *testing.T) {
	tempDir := t.TempDir()

	indexPath := filepath.Join(tempDir, "storethehash.index")
	dataPath := filepath.Join(tempDir, "storethehash.data")

	t.Logf("Createing store with 16-bit index")
	s1, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, store.IndexBitSize(16), store.GCInterval(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s1.Close()) })

	// Store blocks.
	blks := testutil.GenerateEntries(5)
	for i := range blks {
		err = s1.Put(blks[i].Key.Bytes(), blks[i].RawValue())
		require.NoError(t, err)
	}
	// REmove on block.
	removed, err := s1.Remove(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.True(t, removed)

	require.NoError(t, s1.Close())

	// Translate to 26 bits
	t.Logf("Translating store index from 16-bit to 24-bit")
	s2, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, store.IndexBitSize(24), store.GCInterval(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s2.Close()) })

	// Check that blocks still exist.
	for i := 1; i < len(blks); i++ {
		value, found, err := s2.Get(blks[i].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[i].RawValue())
	}

	// Check that removed block was not found.
	_, found, err := s2.Get(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, s2.Close())

	// Translate back to 24 bits.
	t.Logf("Translating store index from 24-bit to 16-bit")
	s3, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, store.IndexBitSize(16), store.GCInterval(0))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s3.Close()) })

	// Check that blocks still exist.
	for i := 1; i < len(blks); i++ {
		value, found, err := s3.Get(blks[i].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[i].RawValue())
	}

	// Check that removed block was not found.
	_, found, err = s3.Get(blks[0].Key.Bytes())
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, s3.Close())

	// Check that double close of store is ok.
	require.NoError(t, s3.Close())
}
