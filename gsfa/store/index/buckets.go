package index

import "github.com/rpcpool/yellowstone-faithful/gsfa/store/types"

// BucketIndex is an index to a bucket
type BucketIndex uint32

// Buckets contains pointers to file offsets
//
// The generic specifies how many bits are used to create the buckets. The number of buckets is
// 2 ^ bits.
type Buckets []types.Position

// NewBuckets returns a list of buckets for the given index size in bits
func NewBuckets(indexSizeBits uint8) (Buckets, error) {
	if indexSizeBits > 32 {
		return nil, types.ErrIndexTooLarge
	}
	return make(Buckets, 1<<indexSizeBits), nil
}

// Put updates a bucket value
func (b Buckets) Put(index BucketIndex, offset types.Position) error {
	if int(index) > len(b)-1 {
		return types.ErrOutOfBounds
	}
	b[int(index)] = offset
	return nil
}

// Get updates returns the value at the given index
func (b Buckets) Get(index BucketIndex) (types.Position, error) {
	if int(index) > len(b)-1 {
		return 0, types.ErrOutOfBounds
	}
	return b[int(index)], nil
}
