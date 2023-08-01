package index

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/store/filecache"
	"github.com/stretchr/testify/require"
)

func TestGC(t *testing.T) {
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, filepath.Base(testIndexPath))

	// Copy test file.
	err := copyFile(testIndexPath, indexPath)
	require.NoError(t, err)

	fc := filecache.New(1)

	// Open index and with nil primary to avoid attempting to remap.
	idx, err := Open(context.Background(), indexPath, nil, 24, 1024, 0, 0, fc)
	require.NoError(t, err)
	defer idx.Close()

	// All index files in use, so gc should not remove any files.
	reclaimed, emptied, err := idx.gc(context.Background(), true)
	require.NoError(t, err)
	require.Zero(t, reclaimed)
	require.Zero(t, emptied)

	require.NoError(t, idx.Close())

	// Copy the first two files as the last two files so that the indexes in
	// them are associated with the last files.
	err = copyFile(indexPath+".0", fmt.Sprintf("%s.%d", indexPath, idx.fileNum+1))
	require.NoError(t, err)
	err = copyFile(indexPath+".1", fmt.Sprintf("%s.%d", indexPath, idx.fileNum+2))
	require.NoError(t, err)

	// Adding index files invalidates the saved bucket state.
	require.NoError(t, RemoveSavedBuckets(indexPath))

	// Open the index with the duplicated files.
	idx, err = Open(context.Background(), indexPath, nil, 24, 1024, 0, 0, fc)
	require.NoError(t, err)
	defer idx.Close()

	// GC should now remove the first 2 files only.
	reclaimed, emptied, err = idx.gc(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, int64(2068), reclaimed)
	require.Equal(t, 2, emptied)

	// Another GC should not remove files.
	reclaimed, emptied, err = idx.gc(context.Background(), true)
	require.NoError(t, err)
	require.Zero(t, reclaimed)
	require.Zero(t, emptied)

	// Check that first file is .2 and last file is .24
	header, err := readHeader(idx.headerPath)
	require.NoError(t, err)
	require.Equal(t, header.FirstFile, uint32(2))
	require.Equal(t, idx.fileNum, uint32(24))

	// --- Test truncation ---

	// Remove buckets for last two records in 2nd to last index file.
	bucketY := 7143210
	bucketZ := 12228148
	idx.buckets[bucketY] = 0
	idx.buckets[bucketZ] = 0

	recordSize := int64(18 + sizePrefixSize)

	fileName := indexFileName(idx.basePath, 23)
	fi, err := os.Stat(fileName)
	require.NoError(t, err)
	sizeBefore := fi.Size()
	t.Log("File size before truncation:", sizeBefore)

	// Run GC and check that second to last file was truncated by two records.
	reclaimed, _, err = idx.gc(context.Background(), false)
	require.NoError(t, err)
	require.Zero(t, reclaimed)

	fi, err = os.Stat(fileName)
	require.NoError(t, err)
	sizeAfter := fi.Size()
	t.Log("File size after trucation:", sizeAfter)
	require.Equal(t, sizeAfter, sizeBefore-(2*recordSize))

	// --- Test dead record merge ---

	// Remove buckets for first two records in 2nd to last index file.
	bucketY = 719032
	bucketZ = 5851659
	idx.buckets[bucketY] = 0
	idx.buckets[bucketZ] = 0

	sizeBefore = fi.Size()

	var deleted bool
	sizeBuffer := make([]byte, sizePrefixSize)

	// Read first record size and deleted bit before GC.
	file, err := openFileForScan(fileName)
	require.NoError(t, err)
	_, err = file.ReadAt(sizeBuffer, 0)
	require.NoError(t, err)
	size := binary.LittleEndian.Uint32(sizeBuffer)
	if size&deletedBit != 0 {
		deleted = true
		size ^= deletedBit
	}
	size1Before := size
	require.False(t, deleted)
	file.Close()
	t.Log("Record size before:", size1Before)

	// Run GC and check that first and second records were merged into one free record.
	reclaimed, _, err = idx.gc(context.Background(), false)
	require.NoError(t, err)
	require.Zero(t, reclaimed)

	fi, err = os.Stat(fileName)
	require.NoError(t, err)
	sizeAfter = fi.Size()

	// File should not have changed size.
	require.Equal(t, sizeAfter, sizeBefore)

	// Read first record size and deleted bit before GC.
	file, err = openFileForScan(fileName)
	require.NoError(t, err)
	_, err = file.ReadAt(sizeBuffer, 0)
	require.NoError(t, err)
	size = binary.LittleEndian.Uint32(sizeBuffer)
	if size&deletedBit != 0 {
		deleted = true
		size ^= deletedBit
	}
	t.Log("Record size after:", size)
	require.True(t, deleted)
	require.Equal(t, size1Before+sizePrefixSize+size1Before, size)
	file.Close()
}
