package compactindexsized

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbauerster/mpb/v8/decor"
)

func itob(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}

func btoi(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

func i32tob(i uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, i)
	return b
}

func TestBuilder8(t *testing.T) {
	const numBuckets = 3
	const valueSize = 8

	// Create a table with 3 buckets.
	builder, err := NewBuilderSized("", numBuckets*targetEntriesPerBucket, valueSize)
	require.NoError(t, err)
	require.NotNil(t, builder)
	assert.Len(t, builder.buckets, 3)
	defer builder.Close()

	// Insert a few entries.
	require.NoError(t, builder.Insert([]byte("hello"), itob(1)))
	require.NoError(t, builder.Insert([]byte("world"), itob(2)))
	require.NoError(t, builder.Insert([]byte("blub"), itob(3)))

	// Create index file.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal index.
	require.NoError(t, builder.Seal(context.TODO(), targetFile))
	require.NoError(t, targetFile.Sync())

	// Assert binary content.
	actual, err := os.ReadFile(targetFile.Name())
	require.NoError(t, err)
	assert.Equal(t, concatBytes(
		// --- File header
		// magic
		Magic[:],
		// header size
		i32tob(14),
		// value size
		[]byte{8, 0, 0, 0, 0, 0, 0, 0},
		// num buckets
		[]byte{3, 0, 0, 0},
		[]byte{1}, // version
		[]byte{0}, // how many kv pairs

		// --- Bucket header 0
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00},
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00},
		// hash len
		[]byte{0x03},
		// padding
		[]byte{0x00},
		// file offset
		[]byte{74, 0x00, 0x00, 0x00, 0x00, 0x00},

		// --- Bucket header 1
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00},
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00},
		// hash len
		[]byte{0x03},
		// padding
		[]byte{0x00},
		// file offset
		[]byte{85, 0x00, 0x00, 0x00, 0x00, 0x00},

		// --- Bucket header 2
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00},
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00},
		// hash len
		[]byte{0x03},
		// padding
		[]byte{0x00},
		// file offset
		[]byte{96, 0x00, 0x00, 0x00, 0x00, 0x00},

		// --- Bucket 0
		// hash
		[]byte{0xe2, 0xdb, 0x55},
		// value
		[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},

		// --- Bucket 1
		// hash
		[]byte{0x92, 0xcd, 0xbb},
		// value
		[]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},

		// --- Bucket 2
		// hash
		[]byte{0xe3, 0x09, 0x6b},
		// value
		[]byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	), actual)

	// Reset file offset.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)

	// Open index.
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")
	require.NotNil(t, db)

	// File header assertions.
	assert.Equal(t, &Header{
		ValueSize:  valueSize,
		NumBuckets: numBuckets,
		Metadata:   &indexmeta.Meta{},
	}, db.Header)

	// Get bucket handles.
	buckets := make([]*Bucket, numBuckets)
	for i := range buckets {
		buckets[i], err = db.GetBucket(uint(i))
		require.NoError(t, err)
	}

	// Ensure out-of-bounds bucket accesses fail.
	_, wantErr := db.GetBucket(numBuckets)
	assert.EqualError(t, wantErr, "out of bounds bucket index: 3 >= 3")

	// Bucket header assertions.
	assert.Equal(t, BucketDescriptor{
		BucketHeader: BucketHeader{
			HashDomain: 0x00,
			NumEntries: 1,
			HashLen:    3,
			FileOffset: 74,
			headerSize: 26,
		},
		Stride:      11, // 3 + 8
		OffsetWidth: 8,
	}, buckets[0].BucketDescriptor)

	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 1,
		HashLen:    3,
		FileOffset: 85,
		headerSize: 26,
	}, buckets[1].BucketHeader)

	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 1,
		HashLen:    3,
		FileOffset: 96,
		headerSize: 26,
	}, buckets[2].BucketHeader)

	// Test lookups.
	entries, err := buckets[2].Load( /*batchSize*/ 4)
	require.NoError(t, err)
	assert.Equal(t, []Entry{
		{
			Hash:  0x6b09e3,
			Value: itob(3),
		},
	}, entries)
}

func TestBuilder8_Random(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}

	const numKeys = uint(500000)
	const keySize = uint(16)
	const valueSize = 8
	const queries = int(10000)

	// Create new builder session.
	builder, err := NewBuilderSized("", numKeys, valueSize)
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.NotEmpty(t, builder.buckets)

	// Ensure we cleaned up after ourselves.
	defer func() {
		_, statErr := os.Stat(builder.tmpDir)
		assert.Truef(t, errors.Is(statErr, fs.ErrNotExist), "Delete failed: %v", statErr)
	}()
	defer builder.Close()

	// Insert items to temp buckets.
	preInsert := time.Now()
	key := make([]byte, keySize)
	for i := uint(0); i < numKeys; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		v := uint64(rand.Int63n(int64(100000))) + 1
		err := builder.Insert(key, itob(v))
		require.NoError(t, err)
	}
	t.Logf("Inserted %d keys in %s", numKeys, time.Since(preInsert))

	// Create file for final index.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal to final index.
	preSeal := time.Now()
	sealErr := builder.Seal(context.TODO(), targetFile)
	require.NoError(t, sealErr, "Seal failed")
	t.Logf("Sealed in %s", time.Since(preSeal))

	// Print some stats.
	targetStat, err := targetFile.Stat()
	require.NoError(t, err)
	t.Logf("Index size: %d (% .2f)", targetStat.Size(), decor.SizeB1000(targetStat.Size()))
	t.Logf("Bytes per entry: %f", float64(targetStat.Size())/float64(numKeys))
	t.Logf("Indexing speed: %f/s", float64(numKeys)/time.Since(preInsert).Seconds())

	// Open index.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")

	// Run query benchmark.
	preQuery := time.Now()
	for i := queries; i != 0; i-- {
		keyN := uint64(rand.Int63n(int64(numKeys)))
		binary.LittleEndian.PutUint64(key, keyN)

		bucket, err := db.LookupBucket(key)
		require.NoError(t, err)

		value, err := bucket.Lookup(key)
		require.NoError(t, err)
		require.Greater(t, btoi(value), uint64(0), "The found value must be > 0")
	}
	t.Logf("Queried %d items", queries)
	t.Logf("Query speed: %f/s", float64(queries)/time.Since(preQuery).Seconds())
}
