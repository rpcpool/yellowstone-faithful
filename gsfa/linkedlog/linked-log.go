package linkedlog

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/tidwall/hashmap"
	"golang.org/x/sync/errgroup"
)

type LinkedLog struct {
	file    *os.File
	cache   *bufio.Writer
	mu      sync.Mutex
	offset  uint64
	writeMu sync.Mutex
}

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)

func NewLinkedLog(filename string) (*LinkedLog, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// seek to the end of the file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	cache := bufio.NewWriterSize(file, MiB*256)
	ll := &LinkedLog{
		file:  file,
		cache: cache,
	}
	currentOffset, err := ll.getCurrentOffset()
	if err != nil {
		return nil, err
	}
	ll.offset = currentOffset
	return ll, nil
}

func (s *LinkedLog) Close() error {
	return s.close()
}

func (c *LinkedLog) close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = c.cache.Flush(); err != nil {
		return err
	}
	err = c.file.Close()
	if err != nil {
		return err
	}
	return
}

// Flush flushes the cache to disk
func (s *LinkedLog) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache.Flush()
}

// getCurrentOffset returns the number of bytes in the file
func (s *LinkedLog) getCurrentOffset() (uint64, error) {
	size, err := s.getSize()
	if err != nil {
		return 0, err
	}
	return uint64(size), nil
}

// getSize returns the size of the file in bytes
func (s *LinkedLog) getSize() (int64, error) {
	fi, err := s.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// write writes the given bytes to the file and returns the offset at which
// they were written.
func (s *LinkedLog) write(b []byte) (uint64, uint32, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	numWritten, err := s.cache.Write(b)
	if err != nil {
		return 0, 0, err
	}
	startOffset := s.offset
	s.offset += uint64(numWritten)
	return startOffset, uint32(numWritten), nil
}

const mib = 1024 * 1024

// Read reads the block stored at the given offset.
func (s *LinkedLog) Read(offset uint64) ([]indexes.OffsetAndSize, uint64, error) {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := s.file.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return nil, 0, err
	}
	// debugln_(func() []any { return []any{"lenBuf:", bin.FormatByteSlice(lenBuf)} })
	// Read the length of the compressed indexes
	compactedIndexesLen, n := binary.Uvarint(lenBuf)
	if n <= 0 {
		return nil, 0, errors.New("invalid compacted indexes length")
	}
	return s.ReadWithSize(offset, compactedIndexesLen)
}

func uvarintSize(n uint64) int {
	return binary.PutUvarint(make([]byte, binary.MaxVarintLen64), n)
}

func (s *LinkedLog) ReadWithSize(offset uint64, size uint64) ([]indexes.OffsetAndSize, uint64, error) {
	if size > 256*mib {
		return nil, 0, fmt.Errorf("compacted indexes length too large: %d", size)
	}
	// debugln("compactedIndexesLen:", compactedIndexesLen)
	// Read the compressed indexes
	data := make([]byte, size)
	_, err := s.file.ReadAt(data, int64(offset)+int64(uvarintSize(size)))
	if err != nil {
		return nil, 0, err
	}
	// debugln_(func() []any { return []any{"data:", bin.FormatByteSlice(data)} })
	// the indexes are up until the last 8 bytes, which are the `next` offset.
	indexes := data[:len(data)-8]
	nextOffset := binary.LittleEndian.Uint64(data[len(data)-8:])
	// Decompress the indexes
	sigIndexes, err := decompressIndexes(indexes)
	if err != nil {
		return nil, 0, fmt.Errorf("error while decompressing indexes: %w", err)
	}
	return sigIndexes, nextOffset, nil
}

func decompressIndexes(data []byte) ([]indexes.OffsetAndSize, error) {
	decompressed, err := decompressZSTD(data)
	if err != nil {
		return nil, fmt.Errorf("error while decompressing data: %w", err)
	}
	return indexes.OffsetAndSizeSliceFromBytes(decompressed)
}

// Put map[PublicKey][]uint64 to file
func (s *LinkedLog) Put(
	dataMap *hashmap.Map[solana.PublicKey, []indexes.OffsetAndSize],
	callbackBefore func(pk solana.PublicKey) (uint64, error),
	callbackAfter func(pk solana.PublicKey, offset uint64, ln uint32) error,
) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pubkeys := make(solana.PublicKeySlice, 0, dataMap.Len())
	for _, k := range dataMap.Keys() {
		pubkeys = append(pubkeys, k)
	}
	// Sort pubkeys
	pubkeys.Sort()

	previousSize, err := s.getSize()
	if err != nil {
		return 0, err
	}

	wg := new(errgroup.Group)
	wg.SetLimit(256)
	for pkIndex := range pubkeys {
		pk := pubkeys[pkIndex]
		sigIndexes, ok := dataMap.Get(pk)
		if !ok {
			return 0, errors.New("public key not found in dataMap")
		}
		slices.Reverse[[]indexes.OffsetAndSize](sigIndexes) // reverse the slice so that the most recent indexes are first
		wg.Go(func() error {
			encodedIndexes, err := createIndexesPayload(sigIndexes)
			if err != nil {
				return fmt.Errorf("error while creating payload: %w", err)
			}
			finalPayload := make([]byte, 0)

			// Write the size of the compressed indexes
			uvLen := encodeUvarint(uint64(len(encodedIndexes)) + 8)
			finalPayload = append(finalPayload, uvLen...)
			// Write the compressed indexes
			finalPayload = append(finalPayload, encodedIndexes...)

			{
				previousListOffset, err := callbackBefore(pk)
				if err != nil {
					return err
				}
				// Write the offset of the previous list for this pubkey:
				finalPayload = append(finalPayload, uint64ToBytes(previousListOffset)...)
			}

			offset, numWrittenBytes, err := s.write(finalPayload)
			if err != nil {
				return err
			}
			return callbackAfter(pk, offset, numWrittenBytes)
		})
	}
	return uint64(previousSize), wg.Wait()
}

func createIndexesPayload(indexes []indexes.OffsetAndSize) ([]byte, error) {
	buf := make([]byte, 0, 9*len(indexes))
	for _, index := range indexes {
		buf = append(buf, index.Bytes()...)
	}
	return compressZSTD(buf)
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}

func encodeUvarint(n uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	written := binary.PutUvarint(buf, n)
	return buf[:written]
}
