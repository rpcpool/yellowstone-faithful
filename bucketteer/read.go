// read.go
package bucketteer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"golang.org/x/exp/mmap"
)

// ErrNotFound is returned when a hash is not in the bucket.
var ErrNotFound = errors.New("not found")

type Reader struct {
	contentReader  io.ReaderAt
	meta           *indexmeta.Meta
	prefixToOffset *bucketToOffset
	prefixToSize   map[uint16]uint64
}

type bucketToOffset [math.MaxUint16 + 1]uint64

func newUint16LayoutPointer() *bucketToOffset {
	var layout bucketToOffset
	for i := range layout {
		layout[i] = math.MaxUint64
	}
	return &layout
}

func prefixToUint16(prefix [2]byte) uint16 {
	return binary.LittleEndian.Uint16(prefix[:])
}

// OpenMMAP opens using memory-mapped IO.
func OpenMMAP(path string) (*Reader, error) {
	file, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	r, err := NewReaderWithSizer(file, info.Size())
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return r, nil
}

func Open(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	r, err := NewReaderWithSizer(file, info.Size())
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return r, nil
}

func NewReader(reader io.ReaderAt) (*Reader, error) {
	size := int64(-1)
	if s, ok := reader.(interface{ Size() int64 }); ok {
		size = s.Size()
	} else if f, ok := reader.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			size = info.Size()
		}
	}
	if size <= 0 {
		return nil, errors.New("cannot detect size or file is empty; use NewReaderWithSizer")
	}
	return NewReaderWithSizer(reader, size)
}

func NewReaderWithSizer(reader io.ReaderAt, totalSize int64) (*Reader, error) {
	if reader == nil {
		return nil, errors.New("reader is nil")
	}

	prefixToOffset, meta, headerTotalSize, err := readHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if int64(headerTotalSize) > totalSize {
		return nil, errors.New("header size exceeds total file size")
	}

	contentSize := uint64(totalSize - headerTotalSize)
	return &Reader{
		meta:           meta,
		prefixToOffset: prefixToOffset,
		prefixToSize:   calcSizeOfBuckets(*prefixToOffset, contentSize),
		contentReader:  io.NewSectionReader(reader, headerTotalSize, int64(contentSize)),
	}, nil
}

func (r *Reader) Close() error {
	if closer, ok := r.contentReader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (r *Reader) CopyBucket(prefix [2]byte) (*bytes.Buffer, error) {
	p := prefixToUint16(prefix)
	offset := r.prefixToOffset[p]
	if offset == math.MaxUint64 {
		return nil, fmt.Errorf("bucket not found for prefix %x", prefix)
	}
	size := r.prefixToSize[p]
	if size == 0 {
		return bytes.NewBuffer(nil), nil
	}

	buf := bytes.NewBuffer(make([]byte, 0, size))
	section := io.NewSectionReader(r.contentReader, int64(offset), int64(size))
	_, err := io.Copy(buf, section)
	return buf, err
}

func calcSizeOfBuckets(offsets bucketToOffset, totalContentSize uint64) map[uint16]uint64 {
	sizeMap := make(map[uint16]uint64)
	var active []uint16
	for p, off := range offsets {
		if off != math.MaxUint64 {
			active = append(active, uint16(p))
		}
	}
	slices.Sort(active)

	for i, p := range active {
		offset := offsets[p]
		if i+1 < len(active) {
			sizeMap[p] = offsets[active[i+1]] - offset
		} else {
			if totalContentSize >= offset {
				sizeMap[p] = totalContentSize - offset
			} else {
				sizeMap[p] = 0
			}
		}
	}
	return sizeMap
}

func readHeader(reader io.ReaderAt) (*bucketToOffset, *indexmeta.Meta, int64, error) {
	sizeBuf := make([]byte, 4)
	if _, err := reader.ReadAt(sizeBuf, 0); err != nil {
		return nil, nil, 0, err
	}
	headerSize := int64(binary.LittleEndian.Uint32(sizeBuf))

	headerBuf := make([]byte, headerSize)
	if _, err := reader.ReadAt(headerBuf, 4); err != nil {
		return nil, nil, 0, err
	}

	decoder := bin.NewBorshDecoder(headerBuf)
	magic := make([]byte, 8)
	if _, err := decoder.Read(magic); err != nil || !bytes.Equal(magic, _Magic[:]) {
		return nil, nil, 0, errors.New("invalid magic")
	}

	ver, err := decoder.ReadUint64(bin.LE)
	if err != nil || ver != Version {
		return nil, nil, 0, fmt.Errorf("version mismatch: %d", ver)
	}

	var meta indexmeta.Meta
	if err := meta.UnmarshalWithDecoder(decoder); err != nil {
		return nil, nil, 0, err
	}

	numPrefixes, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return nil, nil, 0, err
	}

	prefixToOffset := newUint16LayoutPointer()
	for i := uint64(0); i < numPrefixes; i++ {
		var p [2]byte
		if _, err := decoder.Read(p[:]); err != nil {
			return nil, nil, 0, err
		}
		off, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, err
		}
		prefixToOffset[prefixToUint16(p)] = off
	}
	return prefixToOffset, &meta, headerSize + 4, nil
}

func (r *Reader) Has(sig [64]byte) (bool, error) {
	pUint := prefixToUint16([2]byte{sig[0], sig[1]})
	offset := r.prefixToOffset[pUint]
	if offset == math.MaxUint64 {
		return false, nil
	}

	size := r.prefixToSize[pUint]
	if size < 12 || (size-4)%8 != 0 {
		return false, nil
	}

	numHashes := int((size - 4) / 8)
	wantedHash := Hash(sig)

	var scratch [8]byte
	got, err := searchEytzinger(0, numHashes, wantedHash, func(index int) (uint64, error) {
		pos := int64(offset) + 4 + int64(index*8)
		if _, err := r.contentReader.ReadAt(scratch[:], pos); err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(scratch[:]), nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return got == wantedHash, nil
}

func searchEytzinger(min, max int, x uint64, getter func(int) (uint64, error)) (uint64, error) {
	idx := 0
	for idx < max {
		k, err := getter(idx)
		if err != nil {
			return 0, err
		}
		if k == x {
			return k, nil
		}
		idx = idx<<1 | 1
		if k < x {
			idx++
		}
	}
	return 0, ErrNotFound
}

func (r *Reader) Meta() *indexmeta.Meta {
	return r.meta
}
