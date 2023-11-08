package index

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/store/types"
	"github.com/stretchr/testify/require"
)

const testIndexPath = "valuestore_test/storethehash.index"

// testFileSizeLimt is the maximum size for new index files.  Using a small
// file size for testing so that the test index gets split into multiple files.
const testFileSizeLimit = 1024

func TestReadOldHeader(t *testing.T) {
	inFile, err := os.Open(testIndexPath)
	require.NoError(t, err)
	defer inFile.Close()

	version, bucketBits, _, err := readOldHeader(inFile)
	require.NoError(t, err)
	require.Equal(t, version, byte(2))
	require.Equal(t, bucketBits, byte(24))
}

func TestChunkOldIndex(t *testing.T) {
	oldFile, err := openFileForScan(testIndexPath)
	require.NoError(t, err)
	defer oldFile.Close()

	// Skip header in old file.
	_, bucketBits, headerSize, err := readOldHeader(oldFile)
	require.NoError(t, err)

	// Allocate old buckets.
	oldBuckets, err := NewBuckets(bucketBits)
	require.NoError(t, err)

	// Scan the old file into the buckets.
	t.Log("Scanning old index")
	err = testScanIndexFile(oldFile, 0, oldBuckets, 0)
	require.NoError(t, err)

	// Return to beginning of old file.
	_, err = oldFile.Seek(int64(headerSize), 0)
	require.NoError(t, err)

	newIndexPath := filepath.Join(t.TempDir(), "storethehash.index")

	// Do the upgrade to split the index into multiple files.
	t.Log("Chunking old index into new index files")
	lastChunkNum, err := chunkOldIndex(context.Background(), oldFile, newIndexPath, testFileSizeLimit)
	require.NoError(t, err)
	t.Logf("Split old index into %d files", lastChunkNum)

	// Allocate new buckets.
	newBuckets, err := NewBuckets(bucketBits)
	require.NoError(t, err)

	var fileNum, lastFileNum uint32
	var prevSize int64
	for {
		fileName := indexFileName(newIndexPath, fileNum)
		t.Logf("Scanning new index file %s", fileName)
		newFile, err := openFileForScan(fileName)
		if os.IsNotExist(err) {
			break
		}
		require.NoError(t, err)

		fi, err := newFile.Stat()
		require.NoError(t, err)

		err = testScanIndexFile(newFile, fileNum, newBuckets, prevSize)
		newFile.Close()
		require.NoError(t, err)

		prevSize += fi.Size()

		lastFileNum = fileNum
		fileNum++
	}
	require.Equal(t, lastFileNum, lastChunkNum)

	t.Log("Compare old to new buckets")
	for i := 0; i < len(oldBuckets); i++ {
		require.Equal(t, oldBuckets[i], newBuckets[i])
	}
}

func testScanIndexFile(file *os.File, fileNum uint32, buckets Buckets, prevSize int64) error {
	buffered := bufio.NewReader(file)
	sizeBuffer := make([]byte, sizePrefixSize)
	scratch := make([]byte, 256)
	var iterPos int64
	for {
		_, err := io.ReadFull(buffered, sizeBuffer)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		size := binary.LittleEndian.Uint32(sizeBuffer)

		pos := iterPos + sizePrefixSize
		iterPos = pos + int64(size)
		if int(size) > len(scratch) {
			scratch = make([]byte, size)
		}
		data := scratch[:size]
		_, err = io.ReadFull(buffered, data)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("unexpected EOF")
			}
			return err
		}

		bucketPrefix := BucketIndex(binary.LittleEndian.Uint32(data))
		err = buckets.Put(bucketPrefix, types.Position(pos+prevSize))
		if err != nil {
			return err
		}
	}
	return nil
}
