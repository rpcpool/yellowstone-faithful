package readasonecar

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/rpcpool/yellowstone-faithful/carreader"
)

type MultiReader struct {
	currentIndex int
	files        []string
	onClose      []func() error
	readers      []*carreader.CarReader
}

type CarReader interface {
	NextInfo() (cid.Cid, uint64, error)
	NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error)
	NextNodeBytes() (cid.Cid, uint64, []byte, error)

	HeadeSizeUntilNow() (uint64, error)
	Close() error
}

func NewMultiReader(files ...string) (*MultiReader, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided")
	}
	// check that each file exists
	for _, fn := range files {
		if _, err := os.Stat(fn); err != nil {
			return nil, fmt.Errorf("file %q does not exist: %w", fn, err)
		}
	}
	readers := make([]*carreader.CarReader, len(files))
	onClose := make([]func() error, len(files))
	for i, fn := range files {
		file, err := os.Open(fn)
		if err != nil {
			return nil, fmt.Errorf("failed to open car file %q: %w", fn, err)
		}
		onClose[i] = file.Close
		reader, err := carreader.New(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create car reader for file %q: %w", fn, err)
		}
		readers[i] = reader
	}
	return &MultiReader{files: files}, nil
}

func (mr *MultiReader) NextInfo() (cid.Cid, uint64, error) {
	if mr.currentIndex >= len(mr.files) {
		return cid.Cid{}, 0, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, offset, err := r.NextInfo()
	if err == io.EOF {
		mr.currentIndex++
		return mr.NextInfo()
	}
	return cid, offset, err
}

func (mr *MultiReader) NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error) {
	if mr.currentIndex >= len(mr.files) {
		return cid.Cid{}, 0, nil, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, offset, block, err := r.NextNode()
	if err == io.EOF {
		mr.currentIndex++
		return mr.NextNode()
	}
	return cid, offset, block, err
}

func (mr *MultiReader) NextNodeBytes() (cid.Cid, uint64, []byte, error) {
	if mr.currentIndex >= len(mr.files) {
		return cid.Cid{}, 0, nil, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, offset, block, err := r.NextNodeBytes()
	if err == io.EOF {
		mr.currentIndex++
		return mr.NextNodeBytes()
	}
	return cid, offset, block, err
}

func (mr *MultiReader) Close() error {
	var errs []error
	for _, f := range mr.onClose {
		if e := f(); e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (mr *MultiReader) Files() []string {
	return mr.files
}

func (mr *MultiReader) CurrentIndex() int {
	if mr.currentIndex >= len(mr.files) {
		return -1
	}
	return mr.currentIndex
}

func (mr *MultiReader) CurrentReader() *carreader.CarReader {
	if mr.currentIndex >= len(mr.files) {
		return nil
	}
	return mr.readers[mr.currentIndex]
}

func (mr *MultiReader) Readers() []*carreader.CarReader {
	return mr.readers
}

// HeaderSize returns the size of the header of the CAR file at the given index.
func (mr *MultiReader) HeaderSize(index int) (uint64, error) {
	if index >= len(mr.files) {
		return 0, fmt.Errorf("index %d out of bounds", index)
	}
	return mr.readers[index].HeaderSize()
}

// HeadeSizeUntilNow returns the size of the headers of all the CAR files read so far (including the current one).
// E.g. if the current index is 2, this will return the sum of the header sizes of the CAR files at indices 0, 1, and 2.
func (mr *MultiReader) HeadeSizeUntilNow() (uint64, error) {
	if mr.currentIndex >= len(mr.files) {
		cumulativeSize := uint64(0)
		for i := 0; i < len(mr.files); i++ {
			size, err := mr.readers[i].HeaderSize()
			if err != nil {
				return 0, err
			}
			cumulativeSize += size
		}
		return cumulativeSize, nil
	}
	cumulativeSize := uint64(0)
	for i := 0; i <= mr.currentIndex; i++ {
		size, err := mr.readers[i].HeaderSize()
		if err != nil {
			return 0, err
		}
		cumulativeSize += size
	}
	return cumulativeSize, nil
}
