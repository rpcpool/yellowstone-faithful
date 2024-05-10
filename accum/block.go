package accum

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

type ObjectAccumulator struct {
	flushOnKind iplddecoders.Kind
	reader      *carreader.CarReader
	callback    func(*ObjectWithMetadata, []ObjectWithMetadata) error
}

var ErrStop = errors.New("stop")

func isStop(err error) bool {
	return errors.Is(err, ErrStop)
}

func NewObjectAccumulator(
	reader *carreader.CarReader,
	flushOnKind iplddecoders.Kind,
	callback func(*ObjectWithMetadata, []ObjectWithMetadata) error,
) *ObjectAccumulator {
	return &ObjectAccumulator{
		reader:      reader,
		flushOnKind: flushOnKind,
		callback:    callback,
	}
}

type ObjectWithMetadata struct {
	Cid           cid.Cid
	Offset        uint64
	SectionLength uint64
	Object        *blocks.BasicBlock
}

func (oa *ObjectAccumulator) Run(ctx context.Context) error {
	totalOffset := uint64(0)
	{
		if size, err := oa.reader.HeaderSize(); err != nil {
			return err
		} else {
			totalOffset += size
		}
	}
	objects := make([]ObjectWithMetadata, 0, 1000)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c, sectionLength, obj, err := oa.reader.NextNode()
		if err != nil {
			return err
		}
		currentOffset := totalOffset
		totalOffset += sectionLength

		if obj == nil {
			break
		}

		objm := ObjectWithMetadata{
			Cid:           c,
			Offset:        currentOffset,
			SectionLength: sectionLength,
			Object:        obj,
		}

		kind := iplddecoders.Kind(obj.RawData()[1])
		if kind == oa.flushOnKind {
			if err := oa.flush(&objm, clone(objects)); err != nil {
				if isStop(err) {
					return nil
				}
				return err
			}
			clear(objects)
			objects = make([]ObjectWithMetadata, 0, 1000)
		} else {
			objects = append(objects, objm)
		}
	}

	return oa.flush(nil, objects)
}

func (oa *ObjectAccumulator) flush(head *ObjectWithMetadata, other []ObjectWithMetadata) error {
	if head == nil && len(other) == 0 {
		return nil
	}

	return oa.callback(head, other)
}

func clone[T any](s []T) []T {
	v := make([]T, len(s))
	copy(v, s)
	return v
}
