package linkedlog

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/indexes"
)

type LinkedLog struct {
	file    *os.File
	buffer  *bufio.Writer
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
	buf := bufio.NewWriterSize(file, MiB*12)
	ll := &LinkedLog{
		file:   file,
		buffer: buf,
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
	if err = c.buffer.Flush(); err != nil {
		return err
	}
	err = c.file.Close()
	if err != nil {
		return err
	}
	return
}

// Flush flushes the buffer to disk
func (s *LinkedLog) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Flush()
}

// getCurrentOffset returns the number of bytes in the file
func (s *LinkedLog) getCurrentOffset() (uint64, error) {
	stat, err := s.file.Stat()
	if err != nil {
		return 0, err
	}
	return uint64(stat.Size()), nil
}

// getSize returns the size of the file in bytes considering the buffer
func (s *LinkedLog) getSize() (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return int64(s.offset), nil
}

// write writes the given bytes to the file and returns the offset at which
// they were written.
func (s *LinkedLog) write(b []byte) (uint64, uint32, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	numWritten, err := s.buffer.Write(b)
	if err != nil {
		return 0, 0, err
	}
	startOffset := s.offset
	s.offset += uint64(numWritten)
	return startOffset, uint32(numWritten), nil
}

const mib = 1024 * 1024

// Read reads the block stored at the given offset.
func (s *LinkedLog) Read(offset uint64) ([]OffsetAndSizeAndBlocktime, indexes.OffsetAndSize, error) {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := s.file.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return nil, indexes.OffsetAndSize{}, err
	}
	// debugln_(func() []any { return []any{"lenBuf:", bin.FormatByteSlice(lenBuf)} })
	// Read the length of the compressed indexes
	compactedIndexesLen, n := binary.Uvarint(lenBuf)
	if n <= 0 {
		return nil, indexes.OffsetAndSize{}, errors.New("invalid compacted indexes length")
	}
	return s.ReadWithSize(offset, compactedIndexesLen)
}

func sizeOfUvarint(n uint64) int {
	return binary.PutUvarint(make([]byte, binary.MaxVarintLen64), n)
}

func (s *LinkedLog) ReadWithSize(offset uint64, size uint64) ([]OffsetAndSizeAndBlocktime, indexes.OffsetAndSize, error) {
	if size > 256*mib {
		return nil, indexes.OffsetAndSize{}, fmt.Errorf("compacted indexes length too large: %d", size)
	}
	// debugln("compactedIndexesLen:", compactedIndexesLen)
	// Read the compressed indexes
	data := make([]byte, size-uint64(sizeOfUvarint(size))) // The size bytes have already been read.
	_, err := s.file.ReadAt(data, int64(offset)+int64(sizeOfUvarint(size)))
	if err != nil {
		return nil, indexes.OffsetAndSize{}, err
	}
	// debugln_(func() []any { return []any{"data:", bin.FormatByteSlice(data)} })
	// the indexesBytes are up until the last 8 bytes, which are the `next` offset.
	indexesBytes := data[:len(data)-9]
	var nextOffset indexes.OffsetAndSize
	err = nextOffset.FromBytes(data[len(data)-9:])
	if err != nil {
		return nil, indexes.OffsetAndSize{}, fmt.Errorf("error while reading next offset: %w", err)
	}
	// fmt.Println("nextOffset:", nextOffset, offset, size, bin.FormatByteSlice(data)) // DEBUG
	// Decompress the indexes
	sigIndexes, err := decompressIndexes(indexesBytes)
	if err != nil {
		return nil, indexes.OffsetAndSize{}, fmt.Errorf("error while decompressing indexes: %w", err)
	}
	return sigIndexes, nextOffset, nil
}

func decompressIndexes(data []byte) ([]OffsetAndSizeAndBlocktime, error) {
	decompressed, err := decompressZSTD(data)
	if err != nil {
		return nil, fmt.Errorf("error while decompressing data: %w", err)
	}
	return OffsetAndSizeAndBlocktimeSliceFromBytes(decompressed)
}

type KeyToOffsetAndSizeAndBlocktimeSlice []KeyToOffsetAndSizeAndBlocktime

// Has returns true if the given public key is in the slice.
func (s KeyToOffsetAndSizeAndBlocktimeSlice) Has(key solana.PublicKey) bool {
	for _, k := range s {
		if k.Key == key {
			return true
		}
	}
	return false
}

type KeyToOffsetAndSizeAndBlocktime struct {
	Key    solana.PublicKey
	Values []*OffsetAndSizeAndBlocktime
}

func (s *LinkedLog) Put(
	callbackBefore func(pk solana.PublicKey) (indexes.OffsetAndSize, error),
	callbackAfter func(pk solana.PublicKey, offset uint64, ln uint32) error,
	values ...KeyToOffsetAndSizeAndBlocktime,
) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// sort by public key:
	sort.Slice(values, func(i, j int) bool {
		return bytes.Compare(values[i].Key[:], values[j].Key[:]) < 0
	})

	previousSize, err := s.getSize()
	if err != nil {
		return 0, err
	}

	for pkIndex := range values {
		val := values[pkIndex]
		if len(val.Values) == 0 {
			continue
		}
		slices.Reverse[[]*OffsetAndSizeAndBlocktime](val.Values) // reverse the slice so that the most recent indexes are first
		err := func() error {
			encodedIndexes, err := createIndexesPayload(val.Values)
			if err != nil {
				return fmt.Errorf("error while creating payload: %w", err)
			}
			payloadLen := uint64(len(encodedIndexes)) + indexes.IndexValueSize_CidToOffsetAndSize
			payloadLenAsBytes := encodeUvarint(payloadLen)

			// The payload:
			finalPayload := make([]byte, 0, len(payloadLenAsBytes)+len(encodedIndexes)+indexes.IndexValueSize_CidToOffsetAndSize)
			// 1/3 - the size of the compressed indexes
			finalPayload = append(finalPayload, payloadLenAsBytes...)
			// 2/3 - the compressed indexes
			finalPayload = append(finalPayload, encodedIndexes...)

			{
				previousListOffset, err := callbackBefore(val.Key)
				if err != nil {
					return err
				}
				// 3/3 - the offset and size of the previous list for this pubkey:
				finalPayload = append(finalPayload, previousListOffset.Bytes()...)
			}

			offset, numWrittenBytes, err := s.write(finalPayload)
			if err != nil {
				return err
			}
			// fmt.Printf("offset=%d, numWrittenBytes=%d ll=%d\n", offset, numWrittenBytes, ll) // DEBUG
			// fmt.Println("finalPayload:", bin.FormatByteSlice(finalPayload))                  // DEBUG
			return callbackAfter(val.Key, offset, numWrittenBytes)
		}()
		if err != nil {
			return 0, err
		}
	}
	return uint64(previousSize), nil
}

func createIndexesPayload(indexes []*OffsetAndSizeAndBlocktime) ([]byte, error) {
	buf := make([]byte, 0, 9*len(indexes))
	for _, index := range indexes {
		buf = append(buf, index.Bytes()...)
	}
	buf = slices.Clip(buf)
	return (compressZSTD(buf))
}

func encodeUvarint(n uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	written := binary.PutUvarint(buf, n)
	return buf[:written]
}
