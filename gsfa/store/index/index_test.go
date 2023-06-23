package index

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/primary/inmemory"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

const (
	bucketBits uint8  = 24
	fileSize   uint32 = 1024 * 1024 * 1024

	// File cache size for testing.
	testFCSize = 64
)

func TestFirstNonCommonByte(t *testing.T) {
	require.Equal(t, firstNonCommonByte([]byte{0}, []byte{1}), 0)
	require.Equal(t, firstNonCommonByte([]byte{0}, []byte{0}), 1)
	require.Equal(t, firstNonCommonByte([]byte{0, 1, 2, 3}, []byte{0}), 1)
	require.Equal(t, firstNonCommonByte([]byte{0}, []byte{0, 1, 2, 3}), 1)
	require.Equal(t, firstNonCommonByte([]byte{0, 1, 2}, []byte{0, 1, 2, 3}), 3)
	require.Equal(t, firstNonCommonByte([]byte{0, 1, 2, 3}, []byte{0, 1, 2}), 3)
	require.Equal(t, firstNonCommonByte([]byte{3, 2, 1, 0}, []byte{0, 1, 2}), 0)
	require.Equal(t, firstNonCommonByte([]byte{0, 1, 1, 0}, []byte{0, 1, 2}), 2)
	require.Equal(t, firstNonCommonByte([]byte{180, 9, 113, 0}, []byte{180, 0, 113, 0}), 1)
}

func assertHeader(t *testing.T, headerPath string, bucketsBits uint8) {
	header, err := readHeader(headerPath)
	require.NoError(t, err)
	require.Equal(t, header.Version, IndexVersion)
	require.Equal(t, header.BucketsBits, bucketsBits)
}

// Asserts that given two keys that on the first insert the key is trimmed to a single byte and on
// the second insert they are trimmed to the minimal distinguishable prefix
func assertCommonPrefixTrimmed(t *testing.T, key1 []byte, key2 []byte, expectedKeyLength int) {
	primaryStorage := inmemory.New([][2][]byte{{key1, {0x20}}, {key2, {0x30}}})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	iter := NewRawIterator(i.basePath, i.fileNum)
	defer iter.Close()

	// The record list is append only, hence the first record list only contains the first insert
	data, _, done, err := iter.Next()
	require.NoError(t, err)
	require.False(t, done)
	recordlist := NewRecordList(data)
	recordIter := recordlist.Iter()
	var keyLengths []int
	for !recordIter.Done() {
		record := recordIter.Next()
		keyLengths = append(keyLengths, len(record.Key))
	}
	require.Equal(t, keyLengths, []int{1}, "Single key has the expected length of 1")

	// The second block contains both keys
	data, _, done, err = iter.Next()
	require.NoError(t, err)
	require.False(t, done)
	recordlist = NewRecordList(data)
	recordIter = recordlist.Iter()
	keyLengths = []int{}
	for !recordIter.Done() {
		record := recordIter.Next()
		keyLengths = append(keyLengths, len(record.Key))
	}
	require.Equal(t,
		keyLengths,
		[]int{expectedKeyLength, expectedKeyLength},
		"All keys are trimmed to their minimal distringuishable prefix",
	)
}

// This test is about making sure that inserts into an empty bucket result in a key that is trimmed
// to a single byte.

func TestIndexPutSingleKey(t *testing.T) {
	primaryStorage := inmemory.New([][2][]byte{})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 222, Size: 10})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	// Test double close.
	err = i.Close()
	require.NoError(t, err)

	// Skip header
	iter := NewRawIterator(i.basePath, i.fileNum)
	defer iter.Close()
	data, _, done, err := iter.Next()
	require.NoError(t, err)
	require.False(t, done)
	recordlist := NewRecordList(data)
	recordIter := recordlist.Iter()
	require.False(t, recordIter.Done())
	record := recordIter.Next()
	require.Equal(t,
		len(record.Key),
		1,
		"Key is trimmed to one bytes it's the only key in the record list",
	)
}

// This test is about making sure that we remove the record for a key successfully

func TestIndexRemoveKey(t *testing.T) {
	k1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	k2 := []byte{1, 2, 3, 55, 5, 6, 7, 8, 9, 10}
	b1 := types.Block{Offset: 0, Size: 1}
	b2 := types.Block{Offset: 1, Size: 2}

	primaryStorage := inmemory.New([][2][]byte{})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	// Put key 1
	err = i.Put(k1, b1)
	require.NoError(t, err)
	// Put key 2
	err = i.Put(k2, b2)
	require.NoError(t, err)

	// Remove key
	removed, err := i.Remove(k1)
	require.NoError(t, err)
	require.True(t, removed)

	_, found, err := i.Get(k1)
	require.NoError(t, err)
	require.False(t, found)

	secondKeyBlock, found, err := i.Get(k2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, b2)

	// Removing the same key again
	removed, err = i.Remove(k1)
	require.NoError(t, err)
	require.False(t, removed)

	// Trying to remove a non-existing key
	removed, err = i.Remove([]byte{1, 2, 3, 78, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	require.False(t, removed)

	// Flush and check if it holds
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)

	_, found, err = i.Get(k1)
	require.NoError(t, err)
	require.False(t, found)

	secondKeyBlock, found, err = i.Get(k2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, b2)

	// Removing all keys from storage
	removed, err = i.Remove(k2)
	require.NoError(t, err)
	require.True(t, removed)

	// Removing over empty record
	removed, err = i.Remove(k2)
	require.NoError(t, err)
	require.False(t, removed)

	err = i.Close()
	require.NoError(t, err)
}

// This test is about making sure that a new key that doesn't share any prefix with other keys
// within the same bucket is trimmed to a single byte.
func TestIndexPutDistinctKey(t *testing.T) {
	primaryStorage := inmemory.New([][2][]byte{})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 222, Size: 10})
	require.NoError(t, err)
	err = i.Put([]byte{1, 2, 3, 55, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 333, Size: 10})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	iter := NewRawIterator(i.basePath, i.fileNum)
	defer iter.Close()

	// The record list is append only, hence the first record list only contains the first insert
	var data []byte
	var hasData bool
	for {
		next, _, done, err := iter.Next()
		require.NoError(t, err)
		if done {
			break
		}
		data = next
		hasData = true
	}
	require.True(t, hasData)
	recordlist := NewRecordList(data)
	recordIter := recordlist.Iter()
	var keys [][]byte
	for !recordIter.Done() {
		record := recordIter.Next()
		keys = append(keys, record.Key)
	}
	require.Equal(t, keys, [][]byte{{4}, {55}}, "All keys are trimmed to a single byte")
}

func TestCorrectCacheReading(t *testing.T) {
	primaryStorage := inmemory.New([][2][]byte{})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	// put key in, then flush the cache
	err = i.Put([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 222, Size: 10})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	// now put two keys in the same bucket
	err = i.Put([]byte{1, 2, 3, 55, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 333, Size: 10})
	require.NoError(t, err)
	err = i.Put([]byte{1, 2, 3, 88, 5, 6, 7, 8, 9, 10}, types.Block{Offset: 500, Size: 10})
	require.NoError(t, err)

	block, found, err := i.Get([]byte{1, 2, 3, 55, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.Block{Offset: 333, Size: 10}, block)

	err = i.Close()
	require.NoError(t, err)
}

// This test is about making sure that a key is trimmed correctly if it shares a prefix with the
// previous key

func TestIndexPutPrevKeyCommonPrefix(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key2 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	assertCommonPrefixTrimmed(t, key1, key2, 4)
}

// This test is about making sure that a key is trimmed correctly if it shares a prefix with the
// next key
func TestIndexPutNextKeyCommonPrefix(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	assertCommonPrefixTrimmed(t, key1, key2, 4)
}

// This test is about making sure that a key is trimmed correctly if it shares a prefix with the
// previous and the next key, where the common prefix with the next key is longer.
func TestIndexPutPrevAndNextKeyCommonPrefix(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key3 := []byte{1, 2, 3, 4, 5, 6, 9, 8, 8, 8}

	primaryStorage := inmemory.New([][2][]byte{
		{key1, {0x10}},
		{key2, {0x20}},
		{key3, {0x30}},
	})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	err = i.Put(key3, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)
	err = i.Close()
	require.NoError(t, err)

	iter := NewRawIterator(i.basePath, i.fileNum)
	defer iter.Close()

	var data []byte
	for {
		next, _, done, err := iter.Next()
		require.NoError(t, err)
		if done {
			break
		}
		data = next
	}
	recordlist := NewRecordList(data)
	recordIter := recordlist.Iter()
	var keys [][]byte
	for !recordIter.Done() {
		record := recordIter.Next()
		keys = append(keys, record.Key)
	}
	require.Equal(t,
		keys,
		[][]byte{{4, 5, 6, 7}, {4, 5, 6, 9, 8}, {4, 5, 6, 9, 9}},
		"Keys are correctly sorted and trimmed",
	)
}

func TestIndexGetEmptyIndex(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	primaryStorage := inmemory.New([][2][]byte{})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	index, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	_, found, err := index.Get(key)
	require.NoError(t, err)
	require.False(t, found, "Key was not found")
	err = index.Close()
	require.NoError(t, err)
}

func TestIndexGet(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key3 := []byte{1, 2, 3, 4, 5, 6, 9, 8, 8, 8}

	primaryStorage := inmemory.New([][2][]byte{
		{key1, {0x10}},
		{key2, {0x20}},
		{key3, {0x30}},
	})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	err = i.Put(key3, types.Block{Offset: 2, Size: 1})
	require.NoError(t, err)

	firstKeyBlock, found, err := i.Get(key1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, firstKeyBlock, types.Block{Offset: 0, Size: 1})

	secondKeyBlock, found, err := i.Get(key2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, types.Block{Offset: 1, Size: 1})

	thirdKeyBlock, found, err := i.Get(key3)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, thirdKeyBlock, types.Block{Offset: 2, Size: 1})

	// It still hits a bucket where there are keys, but that key doesn't exist.
	_, found, err = i.Get([]byte{1, 2, 3, 4, 5, 9})
	require.False(t, found)
	require.NoError(t, err)

	// A key that matches some prefixes but it shorter than the prefixes.
	_, found, err = i.Get([]byte{1, 2, 3, 4, 5})
	require.False(t, found)
	require.NoError(t, err)

	// same should hold true after flush
	_, err = i.Flush()
	require.NoError(t, err)
	err = i.Sync()
	require.NoError(t, err)

	firstKeyBlock, found, err = i.Get(key1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, firstKeyBlock, types.Block{Offset: 0, Size: 1})

	secondKeyBlock, found, err = i.Get(key2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, types.Block{Offset: 1, Size: 1})

	thirdKeyBlock, found, err = i.Get(key3)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, thirdKeyBlock, types.Block{Offset: 2, Size: 1})

	// It still hits a bucket where there are keys, but that key doesn't exist.
	_, found, err = i.Get([]byte{1, 2, 3, 4, 5, 9})
	require.False(t, found)
	require.NoError(t, err)

	// A key that matches some prefixes but it shorter than the prefixes.
	_, found, err = i.Get([]byte{1, 2, 3, 4, 5})
	require.False(t, found)
	require.NoError(t, err)

	err = i.Close()
	require.NoError(t, err)
	i, err = Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)

	// same should hold true when index is closed and reopened

	firstKeyBlock, found, err = i.Get(key1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, firstKeyBlock, types.Block{Offset: 0, Size: 1})

	secondKeyBlock, found, err = i.Get(key2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, types.Block{Offset: 1, Size: 1})

	thirdKeyBlock, found, err = i.Get(key3)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, thirdKeyBlock, types.Block{Offset: 2, Size: 1})

	err = i.Close()
	require.NoError(t, err)
	bucketsFileName := indexPath + ".buckets"
	require.FileExists(t, bucketsFileName)

	// Open index reading bucket state.
	i, err = Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	t.Cleanup(func() { i.Close() })
	require.NoFileExists(t, bucketsFileName)

	// Open index scanning index files.
	i2, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	t.Cleanup(func() { i2.Close() })

	// Check that both indexes have same buckets.
	require.Equal(t, i.buckets, i2.buckets)
}

func TestIndexHeader(t *testing.T) {
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")

	primaryStorage := inmemory.New([][2][]byte{})
	i1, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	t.Cleanup(func() { i1.Close() })
	assertHeader(t, i1.headerPath, bucketBits)

	// Check that the header doesn't change if the index is opened again.
	i2, err := Open(context.Background(), indexPath, inmemory.New([][2][]byte{}), bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	t.Cleanup(func() { i2.Close() })
	assertHeader(t, i2.headerPath, bucketBits)
}

func TestIndexGetBad(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key3 := []byte{1, 2, 3, 4, 5, 6, 9, 8, 8, 8}
	key4 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	primaryStorage := inmemory.New([][2][]byte{
		{key1, {0x10}},
		{[]byte("X"), {0x20}},
		{key3, {0x30}},
	})

	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, 0, 0, 0, filecache.New(testFCSize))

	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	err = i.Put(key3, types.Block{Offset: 2, Size: 1})
	require.NoError(t, err)

	firstKeyBlock, found, err := i.Get(key1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, firstKeyBlock, types.Block{Offset: 0, Size: 1})

	secondKeyBlock, found, err := i.Get(key2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, types.Block{Offset: 1, Size: 1})

	thirdKeyBlock, found, err := i.Get(key3)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, thirdKeyBlock, types.Block{Offset: 2, Size: 1})

	// This should result in the record for key2 being replaced.
	err = i.Put(key4, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)

	fourthKeyBlock, found, err := i.Get(key4)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, fourthKeyBlock, secondKeyBlock)

	// Index for key2 should be same as index for key4
	secondKeyBlock, found, err = i.Get(key2)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, secondKeyBlock, fourthKeyBlock)

	err = i.Close()
	require.NoError(t, err)
}

func TestFlushRace(t *testing.T) {
	const goroutines = 64
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key3 := []byte{1, 2, 3, 4, 5, 6, 9, 8, 8, 8}

	primaryStorage := inmemory.New([][2][]byte{
		{key1, {0x10}},
		{key2, {0x20}},
		{key3, {0x30}},
	})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)
	err = i.Put(key3, types.Block{Offset: 2, Size: 1})
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error)
	for n := 0; n < goroutines; n++ {
		go func() {
			<-start
			_, err := i.Flush()
			errs <- err
		}()
	}
	close(start)
	for n := 0; n < goroutines; n++ {
		err := <-errs
		require.NoError(t, err)
	}

	require.NoError(t, i.Close())
}

func TestFlushExcess(t *testing.T) {
	key1 := []byte{1, 2, 3, 4, 5, 6, 9, 9, 9, 9}
	key2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	key3 := []byte{1, 2, 3, 4, 5, 6, 9, 8, 8, 8}

	primaryStorage := inmemory.New([][2][]byte{
		{key1, {0x10}},
		{key2, {0x20}},
		{key3, {0x30}},
	})
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "storethehash.index")
	i, err := Open(context.Background(), indexPath, primaryStorage, bucketBits, fileSize, 0, 0, filecache.New(testFCSize))
	require.NoError(t, err)
	err = i.Put(key1, types.Block{Offset: 0, Size: 1})
	require.NoError(t, err)
	err = i.Put(key2, types.Block{Offset: 1, Size: 1})
	require.NoError(t, err)

	work, err := i.Flush()
	require.NoError(t, err)
	require.NotZero(t, work)

	err = i.Put(key3, types.Block{Offset: 2, Size: 1})
	require.NoError(t, err)

	work, err = i.Flush()
	require.NoError(t, err)
	require.NotZero(t, work)

	// Another flush with no new data should not do work.
	work, err = i.Flush()
	require.NoError(t, err)
	require.Zero(t, work)

	require.NoError(t, i.Close())
}
