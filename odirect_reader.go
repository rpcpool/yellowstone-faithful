package main

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// ODirectReader wraps a file opened with O_DIRECT flag
type ODirectReader struct {
	file *os.File
}

// NewODirectReader opens a file with O_DIRECT flag for direct I/O
func NewODirectReader(path string) (*ODirectReader, error) {
	// Open file with O_DIRECT flag
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open file with O_DIRECT: %w", err)
	}

	return &ODirectReader{
		file: file,
	}, nil
}

// ReadAt implements io.ReaderAt interface
func (r *ODirectReader) ReadAt(p []byte, off int64) (n int, err error) {
	// For O_DIRECT, we need to ensure the buffer is aligned
	// and the size is a multiple of the filesystem block size
	blockSize, err := getBlockSize(r.file)
	if err != nil {
		return 0, fmt.Errorf("failed to get block size: %w", err)
	}

	// If the request is small and aligned, try direct read first
	if len(p) == 0 {
		return 0, nil
	}

	// Align the offset to block boundary
	alignedOffset := (off / int64(blockSize)) * int64(blockSize)
	
	// Calculate how much we need to read to cover the requested range
	// and align to block boundary
	startOffset := off - alignedOffset
	endOffset := off + int64(len(p))
	alignedEndOffset := ((endOffset + int64(blockSize) - 1) / int64(blockSize)) * int64(blockSize)
	alignedSize := alignedEndOffset - alignedOffset

	// Allocate aligned buffer
	alignedBuffer := make([]byte, alignedSize)
	
	// Read aligned data
	readBytes, err := r.file.ReadAt(alignedBuffer, alignedOffset)
	if err != nil && err != io.EOF {
		return 0, err
	}

	// Copy the requested portion to the output buffer
	copySize := len(p)
	availableBytes := readBytes - int(startOffset)
	if availableBytes < copySize {
		copySize = availableBytes
		if copySize < 0 {
			copySize = 0
		}
	}

	if copySize > 0 {
		copy(p, alignedBuffer[startOffset:startOffset+int64(copySize)])
	}
	
	// Return the actual number of bytes copied to the output buffer
	if copySize < len(p) && err == nil {
		err = io.EOF
	}
	return copySize, err
}

// Close implements io.Closer interface
func (r *ODirectReader) Close() error {
	return r.file.Close()
}

// getBlockSize gets the filesystem block size for the file
func getBlockSize(file *os.File) (int, error) {
	var stat syscall.Stat_t
	err := syscall.Fstat(int(file.Fd()), &stat)
	if err != nil {
		return 0, err
	}
	return int(stat.Blksize), nil
}

// isODirectSupported checks if O_DIRECT is supported on this system
func isODirectSupported() bool {
	// Try to open a temporary file with O_DIRECT to test support
	tmpFile, err := os.CreateTemp("", "odirect_test")
	if err != nil {
		return false
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Try to open with O_DIRECT
	testFile, err := os.OpenFile(tmpFile.Name(), os.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return false
	}
	testFile.Close()
	return true
}

// ODirectReaderAtCloser combines ReaderAt and Closer interfaces
type ODirectReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

// NewODirectReaderAtCloser creates a new O_DIRECT reader that implements ReaderAtCloser
func NewODirectReaderAtCloser(path string) (ODirectReaderAtCloser, error) {
	return NewODirectReader(path)
}
