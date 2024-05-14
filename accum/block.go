package accum

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

type ObjectAccumulator struct {
	flushOnKind iplddecoders.Kind
	reader      *carreader.CarReader
	callback    func(*ObjectWithMetadata, []ObjectWithMetadata) error
	flushWg     sync.WaitGroup
	flushQueue  chan flushBuffer
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
		flushQueue:  make(chan flushBuffer, 1000),
	}
}

type flushBuffer struct {
	head  *ObjectWithMetadata
	other []ObjectWithMetadata
}

type ObjectWithMetadata struct {
	Cid           cid.Cid
	Offset        uint64
	SectionLength uint64
	Object        *blocks.BasicBlock
}

func (oa *ObjectAccumulator) startFlusher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fb := <-oa.flushQueue:
			if err := oa.flush(fb.head, fb.other); err != nil {
				if isStop(err) {
					return
				}
				panic(err)
			}
			oa.flushWg.Done()
		}
	}
}

func (oa *ObjectAccumulator) sendToFlusher(head *ObjectWithMetadata, other []ObjectWithMetadata) {
	oa.flushQueue <- flushBuffer{head, other}
}

func (oa *ObjectAccumulator) Run(ctx context.Context) error {
	go oa.startFlusher(ctx)
	defer func() {
		close(oa.flushQueue)
		oa.flushWg.Wait()
	}()
	totalOffset := uint64(0)
	{
		if size, err := oa.reader.HeaderSize(); err != nil {
			return err
		} else {
			totalOffset += size
		}
	}
	objectCap := 5000
	objects := make([]ObjectWithMetadata, 0, objectCap)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c, sectionLength, obj, err := oa.reader.NextNode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
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
			oa.flushWg.Add(1)
			oa.sendToFlusher(&objm, clone(objects))
			clear(objects)
			objects = make([]ObjectWithMetadata, 0, objectCap)
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
