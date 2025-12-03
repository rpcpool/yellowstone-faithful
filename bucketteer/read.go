package bucketteer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"slices"
	"time"
	"unsafe"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/exp/mmap"
	"golang.org/x/sys/unix"
)

type Reader struct {
	contentReader   io.ReaderAt
	meta            *indexmeta.Meta
	prefixToOffset  *bucketToOffset
	prefixToSize    map[uint16]uint64
	headerTotalSize int64 // Store this to calculate real file offset
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
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	return NewReader(file, stat.Size())
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
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	return NewReader(file, stat.Size())
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

func NewReader(reader io.ReaderAt, fileSize int64) (*Reader, error) {
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
	r.headerTotalSize = headerTotalSize
	r.contentReader = io.NewSectionReader(reader, headerTotalSize, fileSize-headerTotalSize)

	type fileDescriptor interface {
		Fd() uintptr
		Name() string
	}
	if f, ok := reader.(fileDescriptor); ok {
		// fadvise random access pattern for the whole file
		err := unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_RANDOM)
		if err != nil {
			slog.Warn("fadvise(RANDOM) failed", "error", err)
		}
		{
			slog.Info("Warming up drives for bucket offsets (bucketteer)...", "file", f.Name())
			startedWarmup := time.Now()
			dummyBuf := make([]byte, 1)
			warmedBuckets := 0
			for _, offset := range *r.prefixToOffset {
				if offset != math.MaxUint64 {
					if _, err := r.contentReader.ReadAt(dummyBuf, int64(offset)); err != nil {
						if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
							slog.Warn("Cache warmup read failed", "offset", offset, "error", err)
						}
					}
					warmedBuckets++
				}
			}
			slog.Info(
				"Drive warmup complete",
				"buckets_warmed", warmedBuckets,
				"duration", time.Since(startedWarmup).String(),
				"file", f.Name(),
			)
		}
	} else {
		slog.Warn("Reader does not have an Fd(); cannot use posix_fadvise to manage cache.")
	}

	// if klog.V(4).Enabled() {
	// 	// debug: print all prefixes and their offsets and sizes
	// 	sizeSum := uint64(0)
	// 	for prefix, offset := range *prefixToOffset {
	// 		if offset == math.MaxUint64 {
	// 			continue
	// 		}
	// 		prefixAsUint16 := uint16(prefix)
	// 		size, ok := r.prefixToSize[prefixAsUint16]
	// 		if !ok {
	// 			continue
	// 		}
	// 		sizeSum += size
	// 	}

	// 	// try reading one random bucket andtime it
	// 	startedReadAt := time.Now()
	// 	prefix := uint16(0x1234)
	// 	offset := r.prefixToOffset[prefix]
	// 	if offset != math.MaxUint64 {
	// 		size, ok := r.prefixToSize[prefix]
	// 		if ok && size > 0 {
	// 			bucketReader := io.NewSectionReader(r.contentReader, int64(offset)+4, int64(size-4))
	// 			buf := make([]byte, size-4)
	// 			_, err := bucketReader.Read(buf)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("failed to read bucket for prefix %x: %w", uint16ToPrefix(prefix), err)
	// 			}
	// 			slog.Info(
	// 				"debug_read_bucket",
	// 				"prefix", uint16ToPrefix(prefix),
	// 				"offset", offset,
	// 				"size", size,
	// 				"duration", time.Since(startedReadAt).String(),
	// 			)
	// 		}
	// 	}
	// 	latencies := make([]time.Duration, 0)
	// 	for range 50 {
	// 		// now do a search in the bucket
	// 		sig := [64]byte{}
	// 		rand.Read(sig[:])
	// 		startedSearchAt := time.Now()
	// 		found, err := r.Has(sig)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to search in bucket for prefix %x: %w", uint16ToPrefix(prefix), err)
	// 		}
	// 		dur := time.Since(startedSearchAt)
	// 		slog.Info(
	// 			"debug_search_bucket",
	// 			"prefix", uint16ToPrefix(prefix),
	// 			"found", found,
	// 			"duration", dur.String(),
	// 		)
	// 		latencies = append(latencies, dur)
	// 	}
	// 	{
	// 		sig := solana.MustSignatureFromBase58("2oSE6aiUGWCXUupFnYHgjofV8VSARaUepNDJ3vj2NCK2zFFUNNP6cinjy56vgGXD4WYrKWRkRFcvvC41TgRHM5ML")
	// 		startedSearchAt := time.Now()
	// 		found, err := r.Has(sig)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to search in bucket for prefix %x: %w", uint16ToPrefix(prefix), err)
	// 		}
	// 		dur := time.Since(startedSearchAt)
	// 		slog.Info(
	// 			"debug_search_bucket_known_sig",
	// 			"prefix", uint16ToPrefix(prefix),
	// 			"found", found,
	// 			"duration", dur.String(),
	// 		)
	// 		if !found {
	// 			return nil, fmt.Errorf("known signature not found in bucket for prefix %x", uint16ToPrefix(prefix))
	// 		}
	// 	}
	// 	// porint all latencies
	// 	totalLatency := time.Duration(0)
	// 	for _, lat := range latencies {
	// 		totalLatency += lat
	// 	}
	// 	avgLatency := totalLatency / time.Duration(len(latencies))
	// 	slog.Info(
	// 		"debug_search_bucket_summary",
	// 		"prefix", uint16ToPrefix(prefix),
	// 		"num_searches", len(latencies),
	// 		"average_duration", avgLatency.String(),
	// 	)
	// 	for _, lat := range latencies {
	// 		slog.Info(
	// 			"debug_search_bucket_latency",
	// 			"prefix", uint16ToPrefix(prefix),
	// 			"duration", lat.String(),
	// 		)
	// 	}
	// }
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
	if !ok {
		return false, nil
	}
	if size < 4 {
		return false, fmt.Errorf("invalid bucket size for prefix %x", prefix)
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
	// slog.Info(
	// 	"has_lookup_bucket_details",
	// 	"prefix", prefix,
	// 	"offset", offset,
	// 	"size", size,
	// 	"num_hashes", numHashes,
	// 	"duration", time.Since(start).String(),
	// )
	// startSectionReaderGet := time.Now()
	// bucketReader := r.sectionReaders[prefixToUint16(prefix)]
	// bucketReader := io.NewSectionReader(r.contentReader, int64(offset)+4, int64(numHashes*8))
	bucketReader := io.NewSectionReader(r.contentReader, int64(offset+4), int64(size-4))
	{
		// startReadWhole := time.Now()
		wholeBucketBuf := bytebufferpool.Get()
		defer bytebufferpool.Put(wholeBucketBuf)
		wholeBucketBuf.Reset()
		// wholeBucketBuf := make([]byte, sizeMinus4)
		_, err := wholeBucketBuf.ReadFrom(bucketReader)
		if err != nil {
			return false, fmt.Errorf("failed to read whole bucket for prefix %x: %w", prefix, err)
		}
		// tookReadWhole := time.Since(startReadWhole)
		// create zero-copy []uint64 from wholeBucketBuf
		hashes := unsafe.Slice((*uint64)(unsafe.Pointer(&wholeBucketBuf.B[0])), numHashes)
		wantedHash := Hash(sig)
		foundHash, err := searchEytzingerSlice(hashes, wantedHash)
		// slog.Info(
		// 	"has_lookup_bucket_search_whole",
		// 	"prefix", prefix,
		// 	"offset", offset,
		// 	"size", size,
		// 	"num_hashes", numHashes,
		// 	"wanted_hash", wantedHash,
		// 	"found_hash", foundHash,
		// 	"duration", time.Since(start).String(),
		// 	"duration_read_whole", tookReadWhole.String(),
		// 	"duration_section_reader_get", time.Since(startSectionReaderGet).String(),
		// )
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return false, nil
			}
			return false, err
		}
		if foundHash == wantedHash {
			return true, nil
		}
		return false, nil
	}
}

func searchEytzingerSlice(hashes []uint64, x uint64) (uint64, error) {
	var index int
	max := len(hashes)
	for index < max {
		k := hashes[index]
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

func init() {
	// panic if os is big endian
	var i uint16 = 0x1
	bs := (*[2]byte)(unsafe.Pointer(&i))
	if bs[0] == 0 {
		panic("big endian not supported")
	}
}

var ErrNotFound = fmt.Errorf("not found")
