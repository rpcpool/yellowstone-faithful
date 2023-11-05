// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
package freelist_test

import (
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/store/types"
	"github.com/stretchr/testify/require"
)

func TestFLPut(t *testing.T) {
	tempDir := t.TempDir()
	flPath := filepath.Join(tempDir, "storethehash.free")
	fl, err := freelist.Open(flPath)
	require.NoError(t, err)

	blks := generateFreeListEntries(100)
	for _, blk := range blks {
		err := fl.Put(blk)
		require.NoError(t, err)
	}

	outstandingWork := fl.OutstandingWork()
	expectedStorage := 100 * (types.SizeBytesLen + types.OffBytesLen)
	require.Equal(t, types.Work(expectedStorage), outstandingWork)
	work, err := fl.Flush()
	require.NoError(t, err)
	require.Equal(t, types.Work(expectedStorage), work)
	err = fl.Sync()
	require.NoError(t, err)
	err = fl.Close()
	require.NoError(t, err)

	file, err := os.Open(flPath)
	t.Cleanup(func() { file.Close() })
	require.NoError(t, err)
	iter := freelist.NewIterator(file)
	for _, expectedBlk := range blks {
		blk, err := iter.Next()
		require.NoError(t, err)
		require.Equal(t, expectedBlk.Size, blk.Size)
		require.Equal(t, expectedBlk.Offset, blk.Offset)
	}
	_, err = iter.Next()
	require.EqualError(t, err, io.EOF.Error())

	err = file.Close()
	require.NoError(t, err)
}

func TestToGC(t *testing.T) {
	tempDir := t.TempDir()
	flPath := filepath.Join(tempDir, "storethehash.free")
	fl, err := freelist.Open(flPath)
	require.NoError(t, err)
	t.Cleanup(func() { fl.Close() })

	blks := generateFreeListEntries(100)
	for _, blk := range blks {
		err := fl.Put(blk)
		require.NoError(t, err)
	}
	_, err = fl.Flush()
	require.NoError(t, err)

	flsize, err := fl.StorageSize()
	require.NoError(t, err)

	gcfile, err := fl.ToGC()
	require.NoError(t, err)

	fi, err := os.Stat(gcfile)
	require.NoError(t, err)
	require.Equal(t, flsize, fi.Size())

	flsize, err = fl.StorageSize()
	require.NoError(t, err)
	require.Zero(t, flsize)
}

func generateFreeListEntries(n int) []types.Block {
	blks := make([]types.Block, 0)
	for i := 0; i < n; i++ {
		blks = append(blks, types.Block{
			Size:   types.Size(rand.Int31()),
			Offset: types.Position(rand.Int63()),
		})
	}
	return blks
}
