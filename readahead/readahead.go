package readahead

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
)

const DefaultChunkSize = 12 * MiB

type CachingReader struct {
	file      io.ReadCloser
	buffer    *bufio.Reader
	chunkSize int
}

// NewCachingReader returns a reader that reads from the given file, but caches
// the last chunkSize bytes in memory. This is useful for reading CAR files
// because the CAR format is optimized for sequential reads, but the CAR reader
// needs to first read the object size before reading the object data.
func NewCachingReader(filePath string, chunkSize int) (*CachingReader, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	chunkSize = alignValueToPageSize(chunkSize)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return &CachingReader{file: file, buffer: bufio.NewReaderSize(file, chunkSize), chunkSize: chunkSize}, nil
}

func NewCachingReaderFromReader(file io.ReadCloser, chunkSize int) (*CachingReader, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	chunkSize = alignValueToPageSize(chunkSize)
	return &CachingReader{file: file, buffer: bufio.NewReaderSize(file, chunkSize), chunkSize: chunkSize}, nil
}

func alignValueToPageSize(value int) int {
	pageSize := os.Getpagesize()
	return (value + pageSize - 1) &^ (pageSize - 1)
}

func (cr *CachingReader) Read(p []byte) (int, error) {
	if cr.file == nil {
		return 0, fmt.Errorf("file not open")
	}
	if len(p) == 0 {
		return 0, nil
	}
	return cr.buffer.Read(p)
}

func (cr *CachingReader) Close() error {
	return cr.file.Close()
}
