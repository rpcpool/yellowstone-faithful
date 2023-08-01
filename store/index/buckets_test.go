package index_test

import (
	"testing"

	"github.com/rpcpool/yellowstone-faithful/store/index"
	"github.com/rpcpool/yellowstone-faithful/store/types"
	"github.com/stretchr/testify/require"
)

func TestNewBuckets(t *testing.T) {
	var bucketBits uint8 = 24
	buckets, err := index.NewBuckets(bucketBits)
	require.NoError(t, err)
	require.Equal(t, len(buckets), 1<<bucketBits)
}

func TestNewBucketsError(t *testing.T) {
	var bucketBits uint8 = 64
	_, err := index.NewBuckets(bucketBits)
	require.EqualError(t, err, types.ErrIndexTooLarge.Error())
}

func TestPut(t *testing.T) {
	var bucketBits uint8 = 3
	buckets, err := index.NewBuckets(bucketBits)
	require.NoError(t, err)
	err = buckets.Put(3, 54321)
	require.NoError(t, err)
	value, err := buckets.Get(3)
	require.NoError(t, err)
	require.Equal(t, types.Position(54321), value)
}

func TestPutError(t *testing.T) {
	var bucketBits uint8 = 3
	buckets, err := index.NewBuckets(bucketBits)
	require.NoError(t, err)
	err = buckets.Put(333, 54321)
	require.EqualError(t, err, types.ErrOutOfBounds.Error())
}

func TestGet(t *testing.T) {
	var bucketBits uint8 = 3
	buckets, err := index.NewBuckets(bucketBits)
	require.NoError(t, err)
	value, err := buckets.Get(3)
	require.NoError(t, err)
	require.Equal(t, types.Position(0), value)
	err = buckets.Put(3, 54321)
	require.NoError(t, err)
	value, err = buckets.Get(3)
	require.NoError(t, err)
	require.Equal(t, types.Position(54321), value)
}

func TestGetErrir(t *testing.T) {
	var bucketBits uint8 = 3
	buckets, err := index.NewBuckets(bucketBits)
	require.NoError(t, err)
	_, err = buckets.Get(333)
	require.EqualError(t, err, types.ErrOutOfBounds.Error())
}
