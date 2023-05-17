package compactindex

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbauerster/mpb/v8/decor"
)

func TestBuilder(t *testing.T) {
	const numBuckets = 3
	const maxValue = math.MaxUint64

	// Create a table with 3 buckets.
	builder, err := NewBuilder("", numBuckets*targetEntriesPerBucket, maxValue)
	require.NoError(t, err)
	require.NotNil(t, builder)
	assert.Len(t, builder.buckets, 3)
	defer builder.Close()

	// Insert a few entries.
	require.NoError(t, builder.Insert([]byte("hello"), 1))
	require.NoError(t, builder.Insert([]byte("world"), 2))
	require.NoError(t, builder.Insert([]byte("blub"), 3))

	// Create index file.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal index.
	require.NoError(t, builder.Seal(context.TODO(), targetFile))

	// Assert binary content.
	buf, err := os.ReadFile(targetFile.Name())
	require.NoError(t, err)
	assert.Equal(t, []byte{
		// --- File header
		// magic
		0x72, 0x64, 0x63, 0x65, 0x63, 0x69, 0x64, 0x78,
		// max file size
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		// num buckets
		0x03, 0x00, 0x00, 0x00,
		// padding
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

		// --- Bucket header 0
		// hash domain
		0x00, 0x00, 0x00, 0x00,
		// num entries
		0x01, 0x00, 0x00, 0x00,
		// hash len
		0x03,
		// padding
		0x00,
		// file offset
		0x50, 0x00, 0x00, 0x00, 0x00, 0x00,

		// --- Bucket header 1
		// hash domain
		0x00, 0x00, 0x00, 0x00,
		// num entries
		0x00, 0x00, 0x00, 0x00,
		// hash len
		0x03,
		// padding
		0x00,
		// file offset
		0x5b, 0x00, 0x00, 0x00, 0x00, 0x00,

		// --- Bucket header 2
		// hash domain
		0x00, 0x00, 0x00, 0x00,
		// num entries
		0x02, 0x00, 0x00, 0x00,
		// hash len
		0x03,
		// padding
		0x00,
		// file offset
		0x5b, 0x00, 0x00, 0x00, 0x00, 0x00,

		// --- Bucket 0
		// hash
		0xe2, 0xdb, 0x55,
		// value
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

		// --- Bucket 2
		// hash
		0xe3, 0x09, 0x6b,
		// value
		0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// hash
		0x92, 0xcd, 0xbb,
		// value
		0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}, buf)

	// Reset file offset.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)

	// Open index.
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")
	require.NotNil(t, db)

	// File header assertions.
	assert.Equal(t, Header{
		FileSize:   maxValue,
		NumBuckets: numBuckets,
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
			FileOffset: 0x50,
		},
		Stride:      11, // 3 + 8
		OffsetWidth: 8,
	}, buckets[0].BucketDescriptor)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 0,
		HashLen:    3,
		FileOffset: 0x5b,
	}, buckets[1].BucketHeader)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 2,
		HashLen:    3,
		FileOffset: 0x5b,
	}, buckets[2].BucketHeader)

	// Test lookups.
	entries, err := buckets[2].Load( /*batchSize*/ 4)
	require.NoError(t, err)
	assert.Equal(t, []Entry{
		{
			Hash:  0x6b09e3,
			Value: 3,
		},
		{
			Hash:  0xbbcd92,
			Value: 2,
		},
	}, entries)
}

func TestBuilder_Random(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}

	const numKeys = uint(500000)
	const keySize = uint(16)
	const maxOffset = uint64(1000000)
	const queries = int(10000)

	// Create new builder session.
	builder, err := NewBuilder("", numKeys, maxOffset)
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.NotEmpty(t, builder.buckets)

	// Ensure we cleaned up after ourselves.
	defer func() {
		_, statErr := os.Stat(builder.dir)
		assert.Truef(t, errors.Is(statErr, fs.ErrNotExist), "Delete failed: %v", statErr)
	}()
	defer builder.Close()

	// Insert items to temp buckets.
	preInsert := time.Now()
	key := make([]byte, keySize)
	for i := uint(0); i < numKeys; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		err := builder.Insert(key, uint64(rand.Int63n(int64(maxOffset))))
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
		require.True(t, value > 0)
	}
	t.Logf("Queried %d items", queries)
	t.Logf("Query speed: %f/s", float64(queries)/time.Since(preQuery).Seconds())
}
