package gsfaprimary

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

const testPrimaryPath = "valuestore_test/storethehash.data"

// testFileSizeLimt is the maximum size for new primary files.  Using a small
// file size for testing so that the test primary gets split into multiple files.
const testFileSizeLimit = 1024

func TestUpgradePrimary(t *testing.T) {
	t.Skip("Skipping upgrade test because there upgrade is not supported yet.")
	oldFile, err := os.OpenFile(testPrimaryPath, os.O_RDONLY, 0o644)
	require.NoError(t, err)
	defer oldFile.Close()

	// Scan the old file.
	t.Log("Scanning old primary")
	oldRecs, err := testScanPrimaryFile(oldFile)
	require.NoError(t, err)

	// Return to beginning of old file.
	_, err = oldFile.Seek(0, io.SeekStart)
	require.NoError(t, err)

	newPrimaryPath := filepath.Join(t.TempDir(), "storethehash.data")

	// Copy test file to new location.
	err = copyFile(testPrimaryPath, newPrimaryPath)
	require.NoError(t, err)

	newFreeListPath := filepath.Join(t.TempDir(), "storethehash.index.free")
	freeList, err := freelist.Open(newFreeListPath)
	require.NoError(t, err)
	defer freeList.Close()

	// Do the upgrade to split the primary into multiple files.
	headerPath := newPrimaryPath + ".info"
	updated, err := upgradePrimary(context.Background(), newPrimaryPath, headerPath, testFileSizeLimit, freeList)
	require.NoError(t, err)
	require.NotZero(t, updated)

	lastChunkNum, err := findLastPrimary(newPrimaryPath, 0)
	require.NoError(t, err)

	t.Logf("Split old primary into %d files", lastChunkNum)
	require.Equal(t, int(lastChunkNum), 198)

	// Make sure original file was removed.
	_, err = os.Stat(newPrimaryPath)
	require.True(t, os.IsNotExist(err))

	var newRecs [][]byte
	var fileNum, lastFileNum uint32
	for {
		fileName := primaryFileName(newPrimaryPath, fileNum)
		newFile, err := os.OpenFile(fileName, os.O_RDONLY, 0o644)
		if os.IsNotExist(err) {
			break
		}
		require.NoError(t, err)

		_, err = newFile.Stat()
		require.NoError(t, err)

		recs, err := testScanPrimaryFile(newFile)
		newFile.Close()
		require.NoError(t, err)

		newRecs = append(newRecs, recs...)

		lastFileNum = fileNum
		fileNum++
	}
	require.Equal(t, lastFileNum, lastChunkNum)

	t.Log("Compare old to new records")
	require.Equal(t, len(oldRecs), len(newRecs))
	for i := 0; i < len(oldRecs); i++ {
		require.Equal(t, len(oldRecs[i]), len(newRecs[i]))
		require.Equal(t, oldRecs[i], newRecs[i])
	}

	// Check that header was created
	header, err := readHeader(headerPath)
	require.NoError(t, err)
	require.Equal(t, header.Version, 1)
	require.Equal(t, header.MaxFileSize, uint32(testFileSizeLimit))
	require.Equal(t, header.FirstFile, uint32(0))

	fc := filecache.New(16)
	_, err = Open(newPrimaryPath, nil, fc, 0)
	require.Equal(t, err, types.ErrPrimaryWrongFileSize{testFileSizeLimit, defaultMaxFileSize})

	mp, err := Open(newPrimaryPath, nil, fc, testFileSizeLimit)
	require.NoError(t, err)
	require.NoError(t, mp.Close())

	// Run upgrade again to make sure it does nothing.
	updated, err = upgradePrimary(context.Background(), newPrimaryPath, headerPath, testFileSizeLimit, freeList)
	require.NoError(t, err)
	require.Zero(t, updated)
}

func testScanPrimaryFile(file *os.File) ([][]byte, error) {
	var recs [][]byte

	buffered := bufio.NewReader(file)
	sizeBuffer := make([]byte, sizePrefixSize)
	scratch := make([]byte, 256)
	for {
		_, err := io.ReadFull(buffered, sizeBuffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		size := binary.LittleEndian.Uint32(sizeBuffer)

		if int(size) > len(scratch) {
			scratch = make([]byte, size)
		}
		data := scratch[:size]
		_, err = io.ReadFull(buffered, data)
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("unexpected EOF")
			}
			return nil, err
		}

		rec := make([]byte, len(sizeBuffer)+len(data))
		copy(rec, sizeBuffer)
		copy(rec[len(sizeBuffer):], data)
		recs = append(recs, rec)
	}
	return recs, nil
}

func copyFile(src, dst string) error {
	fin, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fin.Close()

	fout, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fout.Close()

	_, err = io.Copy(fout, fin)
	return err
}
