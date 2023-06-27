package inmemory

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"io"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/primary"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
)

//! In-memory primary storage implementation.
//!
//! It's using a vector of tuples containing the key-value pairs.

type InMemory [][2][]byte

func New(data [][2][]byte) *InMemory {
	value := InMemory(data)
	return &value
}

func (im *InMemory) Get(blk types.Block) (key []byte, value []byte, err error) {
	max := len(*im)
	if blk.Offset >= types.Position(max) {
		return nil, nil, types.ErrOutOfBounds
	}
	val := (*im)[blk.Offset]
	return val[0], val[1], nil
}

func (im *InMemory) Put(key []byte, value []byte) (blk types.Block, err error) {
	pos := len(*im)
	*im = append(*im, [2][]byte{key, value})
	return types.Block{Offset: types.Position(pos), Size: 1}, nil
}

func (im *InMemory) Flush() (types.Work, error) {
	return 0, nil
}

func (im *InMemory) Sync() error {
	return nil
}

func (im *InMemory) Close() error {
	return nil
}

func (im *InMemory) OutstandingWork() types.Work {
	return 0
}

func (im *InMemory) IndexKey(key []byte) ([]byte, error) {
	return key, nil
}

func (im *InMemory) GetIndexKey(blk types.Block) ([]byte, error) {
	key, _, err := im.Get(blk)
	if err != nil {
		return nil, err
	}
	return im.IndexKey(key)
}

func (im *InMemory) Iter() (primary.PrimaryStorageIter, error) {
	return &inMemoryIter{im, 0}, nil
}

type inMemoryIter struct {
	im  *InMemory
	idx int
}

func (imi *inMemoryIter) Next() ([]byte, []byte, error) {
	key, value, err := imi.im.Get(types.Block{Offset: types.Position(imi.idx)})
	if err == types.ErrOutOfBounds {
		return nil, nil, io.EOF
	}
	imi.idx++
	return key, value, nil
}

func (im *InMemory) StorageSize() (int64, error) {
	return 0, nil
}

func (im *InMemory) Overwrite(blk types.Block, key []byte, value []byte) error {
	return nil
}

var _ primary.PrimaryStorage = &InMemory{}
