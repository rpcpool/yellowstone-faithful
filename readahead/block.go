package readahead

import (
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

type ObjectAccumulator struct {
	flushOnKind iplddecoders.Kind
	callback    func(blocks.Block, []blocks.Block)
	objects     []blocks.Block
}

type ObjectWithMetadata struct {
	Offset uint64
	Length uint64
	Object blocks.Block
}

func NewObjectAccumulator(flushOnKind iplddecoders.Kind, callback func(blocks.Block, []blocks.Block)) *ObjectAccumulator {
	return &ObjectAccumulator{
		flushOnKind: flushOnKind,
		callback:    callback,
	}
}
