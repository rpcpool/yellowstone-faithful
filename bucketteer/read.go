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

type Reader struct {
	contentReader  io.ReaderAt
	meta           *indexmeta.Meta
	prefixToOffset *bucketToOffset
	prefixToSize   map[uint16]uint64
}

type bucketToOffset [math.MaxUint16 + 1]uint64

func newUint16Layout() bucketToOffset {
	var layout bucketToOffset
	for i := 0; i <= math.MaxUint16; i++ {
		layout[i] = math.MaxUint64
	}
	return layout
}

func newUint16LayoutPointer() *bucketToOffset {
	var layout bucketToOffset
	for i := 0; i <= math.MaxUint16; i++ {
		layout[i] = math.MaxUint64
	}
	return &layout
}

func prefixToUint16(prefix [2]byte) uint16 {
	return binary.LittleEndian.Uint16(prefix[:])
}

func uint16ToPrefix(num uint16) [2]byte {
	var prefix [2]byte
	binary.LittleEndian.PutUint16(prefix[:], num)
	return prefix
}

// OpenMMAP opens a Bucketteer file in read-only mode,
// using memory-mapped IO.
func OpenMMAP(path string) (*Reader, error) {
	empty, err := isEmptyFile(path)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	file, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}
	return NewReader(file)
}

func Open(path string) (*Reader, error) {
	empty, err := isEmptyFile(path)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewReader(file)
}

func isEmptyFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return false, err
	}
	return stat.Size() == 0, nil
}

func isReaderEmpty(reader io.ReaderAt) (bool, error) {
	if reader == nil {
		return false, errors.New("reader is nil")
	}
	buf := make([]byte, 1)
	_, err := reader.ReadAt(buf, 0)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return true, nil
		}
		return false, err
	}
	return len(buf) == 0, nil
}

func NewReader(reader io.ReaderAt) (*Reader, error) {
	empty, err := isReaderEmpty(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to check if reader is empty: %w", err)
	}
	if empty {
		return nil, fmt.Errorf("reader is empty")
	}
	r := &Reader{
		prefixToOffset: newUint16LayoutPointer(),
	}
	prefixToOffset, meta, headerTotalSize, err := readHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	r.meta = meta
	r.prefixToOffset = prefixToOffset
	r.prefixToSize = calcSizeOfBuckets(*prefixToOffset)
	r.contentReader = io.NewSectionReader(reader, headerTotalSize, 1<<63-1)
	return r, nil
}

func (r *Reader) Close() error {
	if closer, ok := r.contentReader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (r *Reader) Meta() *indexmeta.Meta {
	return r.meta
}

func readHeaderSize(reader io.ReaderAt) (int64, error) {
	// read header size:
	headerSizeBuf := make([]byte, 4)
	if _, err := reader.ReadAt(headerSizeBuf, 0); err != nil {
		return 0, err
	}
	headerSize := int64(binary.LittleEndian.Uint32(headerSizeBuf))
	return headerSize, nil
}

func calcSizeOfBuckets(prefixToOffset bucketToOffset) map[uint16]uint64 {
	prefixToBucketSize := make(map[uint16]uint64)
	var prefixes []uint16
	for prefixAsUint16 := range prefixToOffset {
		prefixes = append(prefixes, uint16(prefixAsUint16))
	}
	// sort prefixes
	sortUint16s(prefixes)
	for i, prefixAsUint16 := range prefixes {
		offset := prefixToOffset[prefixAsUint16]
		var nextOffset uint64
		if i+1 < len(prefixes) {
			nextPrefixAsUint16 := prefixes[i+1]
			nextOffset = prefixToOffset[nextPrefixAsUint16]
		} else {
			nextOffset = math.MaxUint64
		}
		if nextOffset == math.MaxUint64 {
			prefixToBucketSize[prefixAsUint16] = 0
		} else {
			prefixToBucketSize[prefixAsUint16] = nextOffset - offset
		}
	}
	return prefixToBucketSize
}

func sortUint16s(arr []uint16) {
	slices.Sort(arr)
}

func readHeader(reader io.ReaderAt) (*bucketToOffset, *indexmeta.Meta, int64, error) {
	// read header size:
	headerSize, err := readHeaderSize(reader)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read header size: %w", err)
	}
	// read header bytes:
	headerBuf := make([]byte, headerSize)
	if _, err := reader.ReadAt(headerBuf, 4); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read header bytes: %w", err)
	}
	// decode header:
	decoder := bin.NewBorshDecoder(headerBuf)

	// magic:
	{
		magicBuf := make([]byte, len(_Magic[:]))
		_, err := decoder.Read(magicBuf)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to read magic: %w", err)
		}
		if !bytes.Equal(magicBuf, _Magic[:]) {
			return nil, nil, 0, fmt.Errorf("invalid magic: %x", string(magicBuf))
		}
	}
	// version:
	{
		got, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to read version: %w", err)
		}
		if got != Version {
			return nil, nil, 0, fmt.Errorf("expected version %d, got %d", Version, got)
		}
	}
	// read meta:
	var meta indexmeta.Meta
	// read key-value pairs
	if err := meta.UnmarshalWithDecoder(decoder); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	// numPrefixes:
	numPrefixes, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read numPrefixes: %w", err)
	}
	// prefix -> offset:
	prefixToOffset := newUint16Layout()
	for i := uint64(0); i < numPrefixes; i++ {
		var prefix [2]byte
		_, err := decoder.Read(prefix[:])
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to read prefixes[%d]: %w", i, err)
		}
		offset, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to read offsets[%d]: %w", i, err)
		}
		prefixToOffset[prefixToUint16(prefix)] = offset
	}
	return &prefixToOffset, &meta, headerSize + 4, err
}

func (r *Reader) Has(sig [64]byte) (bool, error) {
	// start := time.Now()
	prefix := [2]byte{sig[0], sig[1]}
	offset := r.prefixToOffset[prefixToUint16(prefix)]
	if offset == math.MaxUint64 {
		// This prefix doesn't exist, so the signature can't.
		return false, nil
	}
	size, ok := r.prefixToSize[prefixToUint16(prefix)]
	if !ok || size == 0 {
		return false, nil
	}
	if size < 4 {
		return false, fmt.Errorf("invalid bucket size (%v) for prefix %x", size, prefix)
	}
	sizeMinus4 := size - 4
	numHashes := sizeMinus4 / 8
	if numHashes == 0 {
		// Empty bucket.
		return false, nil
	}
	// if remainer, then size is invalid
	if sizeMinus4%8 != 0 {
		return false, fmt.Errorf("invalid bucket size for prefix %x: size minus 4 is not multiple of 8", prefix)
	}
	bucketReader := io.NewSectionReader(r.contentReader, int64(offset)+4, int64(numHashes*8))

	// hashes:
	wantedHash := Hash(sig)
	got, err := searchEytzinger(0, int(numHashes), wantedHash, func(index int) (uint64, error) {
		pos := int64(index * 8)
		return readUint64Le(bucketReader, pos)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return got == wantedHash, nil
}

func searchEytzinger(min int, max int, x uint64, getter func(int) (uint64, error)) (uint64, error) {
	var index int
	for index < max {
		k, err := getter(index)
		if err != nil {
			return 0, err
		}
		if k == x {
			return k, nil
		}
		index = index<<1 | 1
		if k < x {
			index++
		}
	}
	return 0, ErrNotFound
}

var ErrNotFound = fmt.Errorf("not found")

func readUint64Le(reader io.ReaderAt, pos int64) (uint64, error) {
	buf := make([]byte, 8)
	_, err := reader.ReadAt(buf, pos)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf), nil
}
