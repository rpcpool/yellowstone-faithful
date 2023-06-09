package linkedlog

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/ronanh/intcomp"
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
func (s *LinkedLog) Read(offset uint64) ([]uint64, uint64, error) {
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
	if compactedIndexesLen > 256*mib {
		return nil, 0, fmt.Errorf("compacted indexes length too large: %d", compactedIndexesLen)
	}
	// debugln("compactedIndexesLen:", compactedIndexesLen)
	// Read the compressed indexes
	data := make([]byte, compactedIndexesLen)
	_, err = s.file.ReadAt(data, int64(offset)+int64(n))
	if err != nil {
		return nil, 0, err
	}
	// debugln_(func() []any { return []any{"data:", bin.FormatByteSlice(data)} })
	// the indexes are up until the last 8 bytes, which are the `next` offset.
	indexes := data[:len(data)-8]
	nextOffset := binary.LittleEndian.Uint64(data[len(data)-8:])
	// Decompress the indexes
	sigIndexes := intcomp.UncompressUint64(uint64SliceFromBytes(indexes), make([]uint64, 0))
	return sigIndexes, nextOffset, nil
}

// Put map[PublicKey][]uint64 to file
func (s *LinkedLog) Put(
	dataMap map[solana.PublicKey][]uint64,
	callbackBefore func(pk solana.PublicKey) (uint64, error),
	callbackAfter func(pk solana.PublicKey, offset uint64, ln uint32) error,
) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pubkeys := make(solana.PublicKeySlice, 0, len(dataMap))
	for k := range dataMap {
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
		sigIndexes := dataMap[pk]
		reverseUint64Slice(sigIndexes) // reverse the slice so that the most recent indexes are first
		wg.Go(func() error {
			compactedIndexes := intcomp.CompressUint64(sigIndexes, make([]uint64, 0))

			encodedIndexes := uint64SliceToBytes(compactedIndexes)
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

func reverseUint64Slice(s []uint64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}

func uint64SliceFromBytes(buf []byte) []uint64 {
	if len(buf)%8 != 0 {
		panic(fmt.Sprintf("buf length must be a multiple of 8, got %d", len(buf)))
	}
	slice := make([]uint64, len(buf)/8)
	for i := 0; i < len(slice); i++ {
		slice[i] = binary.LittleEndian.Uint64(buf[i*8:])
	}
	return slice
}

func uint64SliceToBytes(slice []uint64) []byte {
	buf := make([]byte, len(slice)*8)
	for i, num := range slice {
		binary.LittleEndian.PutUint64(buf[i*8:], num)
	}
	return buf
}

func encodeUvarint(n uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	written := binary.PutUvarint(buf, n)
	return buf[:written]
}
