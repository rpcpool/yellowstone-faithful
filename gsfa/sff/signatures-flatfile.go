package sff

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	SignatureSize = 64
)

const (
	writeBufSize = SignatureSize * 1024
)

type SignaturesFlatFile struct {
	file  *os.File
	cache *bufio.Writer
	mu    sync.Mutex
	count uint64
}

func NewSignaturesFlatFile(filename string) (*SignaturesFlatFile, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	cache := bufio.NewWriterSize(file, writeBufSize)
	sfl := &SignaturesFlatFile{
		file:  file,
		cache: cache,
	}
	currentCount, err := sfl.getCurrentCount()
	if err != nil {
		return nil, err
	}
	sfl.count = currentCount
	return sfl, nil
}

// getSize returns the size of the file in bytes
func (s *SignaturesFlatFile) getSize() (int64, error) {
	fi, err := s.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// getCurrentCount returns the number of signatures in the file
func (s *SignaturesFlatFile) getCurrentCount() (uint64, error) {
	size, err := s.getSize()
	if err != nil {
		return 0, err
	}
	// if it's not a multiple of the signature size, it's corrupt
	if size != 0 && size%SignatureSize != 0 {
		return 0, fmt.Errorf("file size is not a multiple of signature size: %d", size)
	}
	return uint64(size / SignatureSize), nil
}

// NumSignatures returns the number of signatures in the file
func (s *SignaturesFlatFile) NumSignatures() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func (s *SignaturesFlatFile) Close() error {
	return s.close()
}

func (c *SignaturesFlatFile) close() (err error) {
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
func (s *SignaturesFlatFile) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache.Flush()
}

func (s *SignaturesFlatFile) Put(sig [SignatureSize]byte) (uint64, error) {
	if sig == EmptySignature {
		return 0, os.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	numWritten, err := s.cache.Write(sig[:])
	if err != nil {
		return 0, err
	}
	if numWritten != SignatureSize {
		return 0, os.ErrInvalid
	}
	s.count++
	return s.count - 1, nil
}

var EmptySignature = [SignatureSize]byte{}

// IsEmpty returns true if the signature is empty
func IsEmpty(sig [SignatureSize]byte) bool {
	return sig == EmptySignature
}

// Get returns the signature at the given index.
// If the index is out of bounds, os.ErrNotExist is returned.
// NOTE: Just-written signatures may not be available until the cache is flushed.
func (s *SignaturesFlatFile) Get(index uint64) ([SignatureSize]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= s.count {
		return EmptySignature, os.ErrNotExist
	}
	sectionReader := io.NewSectionReader(s.file, int64(index*SignatureSize), SignatureSize)
	var sig [SignatureSize]byte
	_, err := io.ReadFull(sectionReader, sig[:])
	if err != nil {
		return EmptySignature, err
	}
	return sig, nil
}
