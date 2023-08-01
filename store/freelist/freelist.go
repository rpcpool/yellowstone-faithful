package freelist

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rpcpool/yellowstone-faithful/store/types"
)

const CIDSizePrefix = 4

// A primary storage that is CID aware.
type FreeList struct {
	file            *os.File
	writer          *bufio.Writer
	outstandingWork types.Work
	blockPool       []types.Block
	poolLk          sync.RWMutex
	flushLock       sync.Mutex
}

const (
	// blockBufferSize is the size of I/O buffers. If has the same size as the
	// linux pipe size.
	blockBufferSize = 16 * 4096
	// blockPoolSize is the size of the freelist cache.
	blockPoolSize = 1024
)

func Open(path string) (*FreeList, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	return &FreeList{
		file:      file,
		writer:    bufio.NewWriterSize(file, blockBufferSize),
		blockPool: make([]types.Block, 0, blockPoolSize),
	}, nil
}

func (cp *FreeList) Put(blk types.Block) error {
	cp.poolLk.Lock()
	defer cp.poolLk.Unlock()
	cp.blockPool = append(cp.blockPool, blk)
	// Offset = 8bytes + Size = 4bytes = 12 Bytes
	cp.outstandingWork += types.Work(types.SizeBytesLen + types.OffBytesLen)
	return nil
}

func (cp *FreeList) flushBlock(blk types.Block) (types.Work, error) {
	sizeBuf := make([]byte, types.SizeBytesLen)
	offBuf := make([]byte, types.OffBytesLen)
	// NOTE: If Position or Size types change, this needs to change.
	binary.LittleEndian.PutUint64(offBuf, uint64(blk.Offset))
	binary.LittleEndian.PutUint32(sizeBuf, uint32(blk.Size))
	// We append offset to size in free list
	if _, err := cp.writer.Write(offBuf); err != nil {
		return 0, err
	}
	if _, err := cp.writer.Write(sizeBuf); err != nil {
		return 0, err
	}
	return types.Work(types.SizeBytesLen + types.OffBytesLen), nil
}

// Flush writes outstanding work and buffered data to the freelist file.
func (cp *FreeList) Flush() (types.Work, error) {
	cp.flushLock.Lock()
	defer cp.flushLock.Unlock()

	cp.poolLk.Lock()
	if len(cp.blockPool) == 0 {
		cp.poolLk.Unlock()
		return 0, nil
	}
	blocks := cp.blockPool
	cp.blockPool = make([]types.Block, 0, blockPoolSize)
	cp.outstandingWork = 0
	cp.poolLk.Unlock()

	// The pool lock is released allowing Put to write to nextPool. The
	// flushLock is still held, preventing concurrent flushes from changing the
	// pool or accessing writer.

	if len(blocks) == 0 {
		return 0, nil
	}

	var work types.Work
	for _, record := range blocks {
		blockWork, err := cp.flushBlock(record)
		if err != nil {
			return 0, err
		}
		work += blockWork
	}
	err := cp.writer.Flush()
	if err != nil {
		return 0, fmt.Errorf("cannot flush data to freelist file %s: %w", cp.file.Name(), err)
	}

	return work, nil
}

// Sync commits the contents of the freelist file to disk. Flush should be
// called before calling Sync.
func (cp *FreeList) Sync() error {
	cp.flushLock.Lock()
	defer cp.flushLock.Unlock()
	return cp.file.Sync()
}

// Close calls Flush to write work and data to the freelist file, and then
// closes the file.
func (cp *FreeList) Close() error {
	_, err := cp.Flush()
	if err != nil {
		cp.file.Close()
		return err
	}
	return cp.file.Close()
}

func (cp *FreeList) OutstandingWork() types.Work {
	cp.poolLk.RLock()
	defer cp.poolLk.RUnlock()
	return cp.outstandingWork
}

func (cp *FreeList) Iter() (*Iterator, error) {
	return NewIterator(cp.file), nil
}

func NewIterator(reader io.Reader) *Iterator {
	return &Iterator{
		reader: reader,
	}
}

type Iterator struct {
	reader io.Reader
}

func (cpi *Iterator) Next() (*types.Block, error) {
	data := make([]byte, types.OffBytesLen+types.SizeBytesLen)
	_, err := io.ReadFull(cpi.reader, data)
	if err != nil {
		return nil, err
	}
	offset := binary.LittleEndian.Uint64(data)
	size := binary.LittleEndian.Uint32(data[types.OffBytesLen:])
	return &types.Block{Size: types.Size(size), Offset: types.Position(offset)}, nil
}

// StorageSize returns bytes of storage used by the freelist.
func (fl *FreeList) StorageSize() (int64, error) {
	fi, err := fl.file.Stat()
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	return fi.Size(), nil
}

// ToGC moves the current freelist file into a ".gc" file and creates a new
// freelist file. This allows the garbage collector to then process the .gc
// freelist file while allowing the freelist to continue to operate on a new
// file.
func (cp *FreeList) ToGC() (string, error) {
	fileName := cp.file.Name()
	workFilePath := fileName + ".gc"

	// If a .gc file already exists, return the existing one becuase it means
	// that GC did not finish processing it.
	_, err := os.Stat(workFilePath)
	if !os.IsNotExist(err) {
		if err != nil {
			return "", err
		}
		return workFilePath, nil
	}

	_, err = cp.Flush()
	if err != nil {
		return "", err
	}

	cp.flushLock.Lock()
	defer cp.flushLock.Unlock()

	// Flush any buffered data and close the file. Safe to do with flushLock
	// acquired.
	cp.writer.Flush()
	cp.file.Close()
	err = os.Rename(fileName, workFilePath)
	if err != nil {
		return "", err
	}

	cp.file, err = os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return "", err
	}
	cp.writer.Reset(cp.file)

	return workFilePath, nil
}
