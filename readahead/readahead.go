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

const DefaultChunkSize = 4 * KiB

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
	return &CachingReader{file: file, buffer: bufio.NewReaderSize(file, chunkSize), chunkSize: chunkSize}, nil
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
