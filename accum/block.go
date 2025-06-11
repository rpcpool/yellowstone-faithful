package accum

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
)

type ObjectAccumulator struct {
	skipNodes   uint64
	flushOnKind iplddecoders.Kind
	reader      readasonecar.CarReader
	ignoreKinds iplddecoders.KindSlice
	callback    func(*ObjectWithMetadata, ObjectsWithMetadata) error
}

var ErrStop = errors.New("stop")

func isStop(err error) bool {
	return errors.Is(err, ErrStop)
}

func NewObjectAccumulator(
	reader readasonecar.CarReader,
	flushOnKind iplddecoders.Kind,
	callback func(*ObjectWithMetadata, ObjectsWithMetadata) error,
	ignoreKinds ...iplddecoders.Kind,
) *ObjectAccumulator {
	return &ObjectAccumulator{
		reader:      reader,
		ignoreKinds: ignoreKinds,
		flushOnKind: flushOnKind,
		callback:    callback,
	}
}

// SetSkip(n)
func (oa *ObjectAccumulator) SetSkip(n uint64) {
	oa.skipNodes = n
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
	parent   *ObjectWithMetadata
	children []ObjectWithMetadata
}

// Reset resets the flushBuffer.
func (fb *flushBuffer) Reset() {
	fb.parent = nil
	fb.children = fb.children[:0]
}

type ObjectWithMetadata struct {
	Cid           cid.Cid
	Offset        uint64
	SectionLength uint64
	ObjectData    []byte
}

type ObjectsWithMetadata []ObjectWithMetadata

func (oa ObjectsWithMetadata) GetTransactionsAndMeta(
	block *ipldbindcode.Block,
) ([]*TransactionWithSlot, error) {
	return ObjectsToTransactionsAndMetadata(block, oa)
}

func (oa *ObjectAccumulator) sendToFlusher(
	cancel context.CancelFunc,
	head *ObjectWithMetadata,
	other []ObjectWithMetadata,
) {
	err := oa.flush(head, other)
	if err != nil {
		if isStop(err) {
			cancel()
			return
		}
	}
}

func (oa *ObjectAccumulator) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
	}()

	numSkipped := uint64(0)
	objectCap := 5000
buffersLoop:
	for {
		children := make([]ObjectWithMetadata, 0, objectCap)
	currentBufferLoop:
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// check is context is done
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			offset, ok := oa.reader.GetGlobalOffsetForNextRead()
			if !ok {
				break buffersLoop
			}

			cid_, sectionLength, data, err := oa.reader.NextNodeBytes()
			if err != nil {
				if errors.Is(err, io.EOF) {
					oa.sendToFlusher(cancel, nil, children)
					break buffersLoop
				}
				return err
			}

			if numSkipped < oa.skipNodes {
				numSkipped++
				continue
			}

			if data == nil {
				oa.sendToFlusher(cancel, nil, children)
				break buffersLoop
			}

			element := ObjectWithMetadata{
				Cid:           cid_,
				Offset:        offset,
				SectionLength: sectionLength,
				ObjectData:    data,
			}

			kind := iplddecoders.Kind(data[1])
			if kind == oa.flushOnKind {
				// element is parent
				oa.sendToFlusher(cancel, &element, children)
				break currentBufferLoop
			} else {
				if len(oa.ignoreKinds) > 0 && oa.ignoreKinds.Has(kind) {
					continue
				}
				children = append(children, element)
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
