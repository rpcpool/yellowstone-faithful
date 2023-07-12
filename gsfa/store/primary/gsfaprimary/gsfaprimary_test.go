package gsfaprimary_test

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"io"
	"path/filepath"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/primary/gsfaprimary"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/testutil"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

// This test is about making sure that inserts into an empty bucket result in a key that is trimmed
// to a single byte.

func TestIndexPut(t *testing.T) {
	tempDir := t.TempDir()
	primaryPath := filepath.Join(tempDir, "storethehash.primary")
	primaryStorage, err := gsfaprimary.Open(primaryPath, nil, filecache.New(1), 0)
	require.NoError(t, err)

	blks := testutil.GenerateEntries(5)
	expectedOffset := types.Position(0)
	for _, blk := range blks {
		expectedSize := len(blk.Key.Bytes()) + len(blk.RawValue())
		loc, err := primaryStorage.Put(blk.Key.Bytes(), blk.RawValue())
		require.NoError(t, err)
		require.Equal(t, expectedOffset, loc.Offset)
		require.Equal(t, types.Size(expectedSize), loc.Size)
		expectedOffset += types.Position(expectedSize)
	}

	outstandingWork := primaryStorage.OutstandingWork()
	require.Equal(t, types.Work(expectedOffset), outstandingWork)
	work, err := primaryStorage.Flush()
	require.NoError(t, err)
	require.Equal(t, types.Work(expectedOffset), work)
	err = primaryStorage.Sync()
	require.NoError(t, err)

	iter := gsfaprimary.NewIterator(primaryPath, 0)
	t.Cleanup(func() { iter.Close() })

	for _, expectedBlk := range blks {
		key, value, err := iter.Next()
		require.NoError(t, err)
		blk := testutil.Entry{Key: solana.PublicKeyFromBytes(key), Value: value}
		require.NoError(t, err)
		require.True(t, expectedBlk.Key.Equals(blk.Key))
		require.Equal(t, expectedBlk.Value, blk.Value)
	}
	_, _, err = iter.Next()
	require.EqualError(t, err, io.EOF.Error())

	err = primaryStorage.Close()
	require.NoError(t, err)
}

func TestIndexGetEmptyIndex(t *testing.T) {
	tempDir := t.TempDir()
	primaryPath := filepath.Join(tempDir, "storethehash.primary")
	primaryStorage, err := gsfaprimary.Open(primaryPath, nil, filecache.New(1), 0)
	require.NoError(t, err)
	defer primaryStorage.Close()

	key, value, err := primaryStorage.Get(types.Block{
		Offset: 0,
		Size:   50,
	})
	require.Nil(t, key)
	require.Nil(t, value)
	require.Error(t, err)
}

func TestIndexGet(t *testing.T) {
	tempDir := t.TempDir()
	primaryPath := filepath.Join(tempDir, "storethehash.primary")
	primaryStorage, err := gsfaprimary.Open(primaryPath, nil, filecache.New(1), 0)
	require.NoError(t, err)

	// load blocks
	blks := testutil.GenerateEntries(5)
	var locs []types.Block
	for _, blk := range blks {
		loc, err := primaryStorage.Put(blk.Key.Bytes(), blk.Value)
		require.NoError(t, err)
		locs = append(locs, loc)
	}

	// should fetch from memory before flush
	for i, loc := range locs {
		expectedBlk := blks[i]
		key, value, err := primaryStorage.Get(loc)
		require.NoError(t, err)
		blk := testutil.Entry{Key: solana.PublicKeyFromBytes(key), Value: value}
		require.NoError(t, err)
		require.True(t, expectedBlk.Key.Equals(blk.Key))
		require.Equal(t, expectedBlk.Value, blk.Value)
	}

	// should fetch from disk after flush
	_, err = primaryStorage.Flush()
	require.NoError(t, err)
	err = primaryStorage.Sync()
	require.NoError(t, err)

	for i, loc := range locs {
		expectedBlk := blks[i]
		key, value, err := primaryStorage.Get(loc)
		require.NoError(t, err)
		blk := testutil.Entry{Key: solana.PublicKeyFromBytes(key), Value: value}
		require.NoError(t, err)
		require.True(t, expectedBlk.Key.Equals(blk.Key))
		require.Equal(t, expectedBlk.Value, blk.Value)
	}

	err = primaryStorage.Close()
	require.NoError(t, err)
}

func TestFlushRace(t *testing.T) {
	const goroutines = 64
	tempDir := t.TempDir()
	primaryPath := filepath.Join(tempDir, "storethehash.primary")
	primaryStorage, err := gsfaprimary.Open(primaryPath, nil, filecache.New(1), 0)
	require.NoError(t, err)

	// load blocks
	blks := testutil.GenerateEntries(5)
	for _, blk := range blks {
		_, err := primaryStorage.Put(blk.Key.Bytes(), blk.Value)
		require.NoError(t, err)
	}

	start := make(chan struct{})
	errs := make(chan error)
	for n := 0; n < goroutines; n++ {
		go func() {
			<-start
			_, err := primaryStorage.Flush()
			errs <- err
		}()
	}
	close(start)
	for n := 0; n < goroutines; n++ {
		err := <-errs
		require.NoError(t, err)
	}

	require.NoError(t, primaryStorage.Close())
}

func TestFlushExcess(t *testing.T) {
	tempDir := t.TempDir()
	primaryPath := filepath.Join(tempDir, "storethehash.primary")
	primaryStorage, err := gsfaprimary.Open(primaryPath, nil, filecache.New(1), 0)
	require.NoError(t, err)

	// load blocks
	blks := testutil.GenerateEntries(5)
	for _, blk := range blks {
		_, err := primaryStorage.Put(blk.Key.Bytes(), blk.Value)
		require.NoError(t, err)
	}

	work, err := primaryStorage.Flush()
	require.NoError(t, err)
	require.NotZero(t, work)

	blks = testutil.GenerateEntries(5)
	for _, blk := range blks {
		_, err := primaryStorage.Put(blk.Key.Bytes(), blk.Value)
		require.NoError(t, err)
	}

	work, err = primaryStorage.Flush()
	require.NoError(t, err)
	require.NotZero(t, work)

	// Another flush with no new data should not do work.
	work, err = primaryStorage.Flush()
	require.NoError(t, err)
	require.Zero(t, work)

	require.NoError(t, primaryStorage.Close())
}
