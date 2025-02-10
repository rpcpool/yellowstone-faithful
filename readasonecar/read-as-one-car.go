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

var _ CarReader = (*MultiReader)(nil)

type CarReader interface {
	Next() (blocks.Block, error)
	NextInfo() (cid.Cid, uint64, error)
	NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error)
	NextNodeBytes() (cid.Cid, uint64, []byte, error)

	GetGlobalOffsetForNextRead() (uint64, bool)
	Close() error
	io.ReaderAt
}

type MultiReader struct {
	currentIndex int
	globalOffset uint64
	readers      []*Container
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
	readers := make([]*Container, len(files))
	for i, fn := range files {
		r, err := OpenFile(fn)
		if err != nil {
			return nil, fmt.Errorf("failed to create reader for file %q: %w", fn, err)
		}
		readers[i] = r
	}
	return &MultiReader{
		globalOffset: readers[0].HeaderSize,
		currentIndex: 0,
		readers:      readers,
	}, nil
}

type Container struct {
	Path       string
	Size       uint64 // is the whole file size
	HeaderSize uint64 // is the size of the header
	File       *os.File
	CarReader  *carreader.CarReader
}

// Close closes the underlying file.
func (frs *Container) Close() error {
	return frs.File.Close()
}

func OpenFile(path string) (*Container, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", path, err)
	}
	reader, err := carreader.New(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create car reader for file %q: %w", path, err)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for file %q: %w", path, err)
	}
	headerSize, err := reader.HeaderSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get header size for file %q: %w", path, err)
	}
	return &Container{
		Path:       path,
		Size:       uint64(stat.Size()),
		HeaderSize: headerSize,
		File:       file,
		CarReader:  reader,
	}, nil
}

func (mr *MultiReader) Next() (blocks.Block, error) {
	_, _, block, err := mr.NextNode()
	return block, err
}

func (mr *MultiReader) NextInfo() (cid.Cid, uint64, error) {
	if mr.currentIndex >= len(mr.readers) {
		return cid.Cid{}, 0, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, sectionLen, err := r.CarReader.NextInfo()
	if errors.Is(err, io.EOF) {
		mr.move()
		return mr.NextInfo()
	}
	if err == nil {
		mr.incrGlobalOffset(sectionLen)
	}
	return cid, sectionLen, err
}

func (mr *MultiReader) NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error) {
	if mr.currentIndex >= len(mr.readers) {
		return cid.Cid{}, 0, nil, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, sectionLen, block, err := r.CarReader.NextNode()
	if errors.Is(err, io.EOF) {
		mr.move()
		return mr.NextNode()
	}
	if err == nil {
		mr.incrGlobalOffset(sectionLen)
	}
	return cid, sectionLen, block, err
}

func (mr *MultiReader) NextNodeBytes() (cid.Cid, uint64, []byte, error) {
	if mr.currentIndex >= len(mr.readers) {
		return cid.Cid{}, 0, nil, io.EOF
	}
	r := mr.readers[mr.currentIndex]
	cid, sectionLen, block, err := r.CarReader.NextNodeBytes()
	if errors.Is(err, io.EOF) {
		mr.move()
		return mr.NextNodeBytes()
	}
	if err == nil {
		mr.incrGlobalOffset(sectionLen)
	}
	return cid, sectionLen, block, err
}

func (mr *MultiReader) move() {
	mr.currentIndex++
	if mr.currentIndex >= len(mr.readers) {
		return
	}
	mr.incrGlobalOffset(mr.readers[mr.currentIndex-1].HeaderSize)
}

func (mr *MultiReader) incrGlobalOffset(offset uint64) {
	mr.globalOffset += offset
}

func (mr *MultiReader) Close() error {
	var errs []error
	for _, f := range mr.readers {
		if e := f.Close(); e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (mr *MultiReader) CurrentIndex() int {
	if mr.currentIndex >= len(mr.readers) {
		return -1
	}
	return mr.currentIndex
}

func (mr *MultiReader) Current() (*Container, int) {
	if mr.currentIndex >= len(mr.readers) {
		return nil, -1
	}
	return mr.readers[mr.currentIndex], mr.currentIndex
}

// HeaderSize returns the size of the header of the CAR file at the given index.
func (mr *MultiReader) HeaderSize(index int) (uint64, error) {
	if index >= len(mr.readers) {
		return 0, fmt.Errorf("index %d out of bounds", index)
	}
	return mr.readers[index].HeaderSize, nil
}

// SizeOfPreviousFiles returns the cumulative size of all files BEFORE the given index.
func (mr *MultiReader) SizeOfPreviousFiles(index int) (uint64, error) {
	if index >= len(mr.readers) {
		return 0, fmt.Errorf("index %d out of bounds", index)
	}
	var size uint64
	for i := 0; i < index; i++ {
		size += mr.readers[i].Size
	}
	return size, nil
}

// GetGlobalOffsetForNextRead returns the global offset for the next read operation.
// It also returns whether there is more data to read (if false, then it means that the returned offset is the end of the last file).
func (mr *MultiReader) GetGlobalOffsetForNextRead() (uint64, bool) {
	if mr.currentIndex >= len(mr.readers) {
		return 0, false
	}
	// if the global offset is the same size as the sum of this and the previous files, we are at the beginning of the next file, so we need to add the header size of the current file
	sum, err := mr.SizeOfPreviousFiles(mr.currentIndex)
	if err != nil {
		return 0, false
	}
	sum += mr.readers[mr.currentIndex].Size
	if mr.globalOffset == sum {
		isLast := mr.currentIndex == len(mr.readers)-1
		if !isLast {
			return sum + mr.readers[mr.currentIndex].HeaderSize, true
		}
		return sum, false
	}
	return mr.globalOffset, true
}

func (mr *MultiReader) ReadAt(p []byte, off int64) (n int, err error) {
	file, realOff, err := mr.offsetToFile(uint64(off))
	if err != nil {
		return 0, err
	}
	return file.File.ReadAt(p, realOff)
}

func (mr *MultiReader) offsetToFile(offset uint64) (*Container, int64, error) {
	var size uint64
	sizeDelta := uint64(0)
	for i, f := range mr.readers {
		size += f.Size
		if offset < size {
			return mr.readers[i], int64(offset - sizeDelta), nil
		}
		sizeDelta += f.Size
	}
	return nil, -1, fmt.Errorf("offset %d out of bounds", offset)
}

func (mr *MultiReader) FindRoot() (cid.Cid, error) {
	if len(mr.readers) == 0 {
		return cid.Undef, fmt.Errorf("no files to read")
	}
	// get the Root CID from the last file
	last := mr.readers[len(mr.readers)-1]
	if last == nil {
		return cid.Undef, fmt.Errorf("last file is nil")
	}
	if last.CarReader == nil {
		return cid.Undef, fmt.Errorf("last file reader is nil")
	}
	roots := last.CarReader.Header.Roots
	if len(roots) == 0 {
		return cid.Undef, fmt.Errorf("no roots found in the last file")
	}
	if len(roots) > 1 {
		return cid.Undef, fmt.Errorf("more than one root found in the last file")
	}
	return roots[0], nil
}
