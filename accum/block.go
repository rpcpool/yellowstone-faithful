package accum

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

type ObjectAccumulator struct {
	flushOnKind iplddecoders.Kind
	reader      *carreader.CarReader
	ignoreKinds iplddecoders.KindSlice
	callback    func(*ObjectWithMetadata, []ObjectWithMetadata) error
	flushWg     sync.WaitGroup
	flushQueue  chan *flushBuffer
}

var ErrStop = errors.New("stop")

func isStop(err error) bool {
	return errors.Is(err, ErrStop)
}

func NewObjectAccumulator(
	reader *carreader.CarReader,
	flushOnKind iplddecoders.Kind,
	callback func(*ObjectWithMetadata, []ObjectWithMetadata) error,
	ignoreKinds ...iplddecoders.Kind,
) *ObjectAccumulator {
	return &ObjectAccumulator{
		reader:      reader,
		ignoreKinds: ignoreKinds,
		flushOnKind: flushOnKind,
		callback:    callback,
		flushQueue:  make(chan *flushBuffer, 1000),
	}
}

var flushBufferPool = sync.Pool{
	New: func() interface{} {
		return &flushBuffer{}
	},
}

func getFlushBuffer() *flushBuffer {
	return flushBufferPool.Get().(*flushBuffer)
}

func putFlushBuffer(fb *flushBuffer) {
	fb.Reset()
	flushBufferPool.Put(fb)
}

type flushBuffer struct {
	head  *ObjectWithMetadata
	other []ObjectWithMetadata
}

// Reset resets the flushBuffer.
func (fb *flushBuffer) Reset() {
	fb.head = nil
	clear(fb.other)
}

type ObjectWithMetadata struct {
	Cid           cid.Cid
	Offset        uint64
	SectionLength uint64
	ObjectData    []byte
}

func (oa *ObjectAccumulator) startFlusher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fb := <-oa.flushQueue:
			if fb == nil {
				return
			}
			if err := oa.flush(fb.head, fb.other); err != nil {
				if isStop(err) {
					return
				}
				panic(err)
			}
			oa.flushWg.Done()
			putFlushBuffer(fb)
		}
	}
}

func (oa *ObjectAccumulator) sendToFlusher(head *ObjectWithMetadata, other []ObjectWithMetadata) {
	oa.flushWg.Add(1)
	fb := getFlushBuffer()
	fb.head = head
	fb.other = clone(other)
	oa.flushQueue <- fb
}

func (oa *ObjectAccumulator) Run(ctx context.Context) error {
	go oa.startFlusher(ctx)
	defer func() {
		oa.flushWg.Wait()
		close(oa.flushQueue)
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
buffersLoop:
	for {
		objects := make([]ObjectWithMetadata, 0, objectCap)
	currentBufferLoop:
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c, sectionLength, data, err := oa.reader.NextNodeBytes()
			if err != nil {
				if errors.Is(err, io.EOF) {
					oa.sendToFlusher(nil, objects)
					break buffersLoop
				}
				return err
			}
			currentOffset := totalOffset
			totalOffset += sectionLength

			if data == nil {
				oa.sendToFlusher(nil, objects)
				break buffersLoop
			}

			objm := ObjectWithMetadata{
				Cid:           c,
				Offset:        currentOffset,
				SectionLength: sectionLength,
				ObjectData:    data,
			}

			kind := iplddecoders.Kind(data[1])
			if kind == oa.flushOnKind {
				oa.sendToFlusher(&objm, (objects))
				break currentBufferLoop
			} else {
				if len(oa.ignoreKinds) > 0 && oa.ignoreKinds.Has(kind) {
					continue
				}
				objects = append(objects, objm)
			}
		}
	}

	return nil
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
