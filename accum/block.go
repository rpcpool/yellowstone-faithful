package accum

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/filecoin-project/go-leb128"
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

// RawSection returns the CAR object as it would be written to a CAR file.
func (obj ObjectWithMetadata) RawSection() ([]byte, error) {
	buf := make([]byte, 0)
	// section is an encoded CAR object
	// length = len(cid) + len(data)
	// section = leb128(length) || cid || data

	sectionLen := len(obj.Cid.Bytes()) + len(obj.ObjectData)
	// write uvarint length of the section
	buf = append(buf, leb128.FromUInt64(uint64(sectionLen))...)
	// write cid
	buf = append(buf, obj.Cid.Bytes()...)
	// write data
	buf = append(buf, obj.ObjectData...)
	return buf, nil
}

func (obj ObjectWithMetadata) RawSectionSize() int {
	sectionLen := len(obj.Cid.Bytes()) + len(obj.ObjectData)
	lenBytes := leb128.FromUInt64(uint64(sectionLen))

	// Size is:
	// length of LEB128-encoded section length +
	// length of CID bytes +
	// length of object data
	return len(lenBytes) + sectionLen
}

// {
// 	raw, err := objm.RawSection()
// 	if err != nil {
// 		panic(err)
// 	}
// 	rawLen := (len(raw))
// 	if rawLen != int(sectionLength) {
// 		panic(fmt.Sprintf("section length mismatch: got %d, expected %d", rawLen, sectionLength))
// 	}

// 	_c, _sectionLen, _data, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(raw)))
// 	if err != nil {
// 		panic(err)
// 	}
// 	if _c != c {
// 		panic(fmt.Sprintf("cid mismatch: got %s, expected %s", _c, c))
// 	}
// 	if _sectionLen != sectionLength {
// 		panic(fmt.Sprintf("section length mismatch: got %d, expected %d", _sectionLen, sectionLength))
// 	}
// 	if !bytes.Equal(_data, data) {
// 		panic(fmt.Sprintf("data mismatch: got %x, expected %x", _data, data))
// 	}
// }

func clone[T any](s []T) []T {
	v := make([]T, len(s))
	copy(v, s)
	return v
}
