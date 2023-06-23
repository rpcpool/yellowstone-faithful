package store_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	store "github.com/rpcpool/yellowstone-faithful/gsfa/store"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/testutil"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

func initStore(t *testing.T, dir string, immutable bool) (*store.Store, error) {
	indexPath := filepath.Join(dir, "storethehash.index")
	dataPath := filepath.Join(dir, "storethehash.data")
	store, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, immutable, store.GCInterval(0))
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store, nil
}

func TestUpdate(t *testing.T) {
	t.Run("when not immutable", func(t *testing.T) {
		tempDir := t.TempDir()
		s, err := initStore(t, tempDir, false)
		require.NoError(t, err)
		blks := testutil.GenerateEntries(2)

		t.Logf("Putting a new block")
		err = s.Put(blks[0].Key.Bytes(), blks[0].RawValue())
		require.NoError(t, err)
		value, found, err := s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[0].RawValue())

		t.Logf("Overwrite same key with different value")
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.NoError(t, err)
		value, found, err = s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		// the put must only have updated the second half of the value (the first half is immutable)
		// i.e. [0:8] of the updated value should be the same as [0:8] of the original value.
		require.Equal(t, append(value[0:8], blks[1].RawValue()[8:16]...), blks[1].RawValue())
		require.Equal(t, value[0:8], blks[0].RawValue()[0:8])
		require.Equal(t, value[8:16], blks[1].RawValue()[8:16])

		t.Logf("Overwrite same key with same value")
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.NoError(t, err) // immutable would return error
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
	t.Run("when immutable", func(t *testing.T) {
		tempDir := t.TempDir()
		s, err := initStore(t, tempDir, true)
		require.NoError(t, err)
		blks := testutil.GenerateEntries(2)

		t.Logf("Putting a new block")
		err = s.Put(blks[0].Key.Bytes(), blks[0].RawValue())
		require.NoError(t, err)
		value, found, err := s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[0].RawValue())

		t.Logf("Overwrite same key with different value")
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.Error(t, err, types.ErrKeyExists.Error())
		value, found, err = s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[0].RawValue())

		t.Logf("Overwrite same key with same value")
		err = s.Put(blks[0].Key.Bytes(), blks[1].RawValue())
		require.Error(t, err, types.ErrKeyExists.Error())
		value, found, err = s.Get(blks[0].Key.Bytes())
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, value, blks[0].RawValue())

		s.Flush()

		// Start iterator
		flPath := filepath.Join(tempDir, "storethehash.index.free")
		file, err := os.Open(flPath)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, file.Close()) })

		iter := freelist.NewIterator(file)
		// Check freelist -- no updates
		_, err = iter.Next()
		require.EqualError(t, err, io.EOF.Error())

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
			require.Equal(t, blks[0].RawValue(), val)
			count++
		}
	})
}

func TestRemove(t *testing.T) {
	tempDir := t.TempDir()
	s, err := initStore(t, tempDir, false)
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
	s1, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, false, store.IndexBitSize(16), store.GCInterval(0))
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
	s2, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, false, store.IndexBitSize(24), store.GCInterval(0))
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
	s3, err := store.OpenStore(context.Background(), store.GsfaPrimary, dataPath, indexPath, false, store.IndexBitSize(16), store.GCInterval(0))
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
