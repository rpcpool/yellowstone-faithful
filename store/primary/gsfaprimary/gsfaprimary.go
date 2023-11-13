package gsfaprimary

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gagliardetto/solana-go"
	logging "github.com/ipfs/go-log/v2"
	"github.com/rpcpool/yellowstone-faithful/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/store/primary"
	"github.com/rpcpool/yellowstone-faithful/store/types"
)

var log = logging.Logger("storethehash/gsfaprimary")

const (
	// PrimaryVersion is stored in the header data to indicate how to interpret
	// primary data.
	PrimaryVersion = 1

	// defaultMaxFileSize is largest the max file size is allowed to be.
	defaultMaxFileSize = uint32(1024 * 1024 * 1024)

	// blockBufferSize is the size of primary I/O buffers. If has the same size
	// as the linux pipe size.
	blockBufferSize = 16 * 4096
	// blockPoolSize is the size of the primary cache.
	blockPoolSize = 1024

	// TODO: remove sizePrefixSize anywhere it is used.
	sizePrefixSize = 1

	// TODO: remove deletedBit anywhere it is used.
	// TODO: replace deletedBit with a byte? or do the same thing
	deletedBit = uint32(1 << 31)
)
const primaryRecordSize = 32 + 8

// A primary storage that is multihash aware.
type GsfaPrimary struct {
	basePath          string
	file              *os.File
	headerPath        string
	maxFileSize       uint32
	writer            *bufio.Writer
	outstandingWork   types.Work
	curPool, nextPool blockPool
	poolLk            sync.RWMutex
	flushLock         sync.Mutex
	fileCache         *filecache.FileCache

	// fileNum and length track flushed data.
	fileNum uint32
	length  types.Position

	// recFileNum and recPos track where each record will be written when they
	// are flushed to disk.
	recFileNum uint32
	recPos     types.Position

	closed bool
}

type blockRecord struct {
	key   []byte
	value []byte
}
type blockPool struct {
	refs   map[types.Block]int
	blocks []blockRecord
}

func newBlockPool() blockPool {
	return blockPool{
		refs:   make(map[types.Block]int, blockPoolSize),
		blocks: make([]blockRecord, 0, blockPoolSize),
	}
}

func _clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append(b[:0:0], b...)
}

// Open opens the gsfa primary storage file. The primary is created if
// there is no existing primary at the specified path. If there is an older
// version primary, then it is automatically upgraded.
func Open(path string, freeList *freelist.FreeList, fileCache *filecache.FileCache, maxFileSize uint32) (*GsfaPrimary, error) {
	headerPath := filepath.Clean(path) + ".info"

	if maxFileSize == 0 {
		maxFileSize = defaultMaxFileSize
	} else if maxFileSize > defaultMaxFileSize {
		return nil, fmt.Errorf("maximum primary file size cannot exceed %d", defaultMaxFileSize)
	}

	var lastPrimaryNum uint32
	header, err := readHeader(headerPath)
	if os.IsNotExist(err) {
		// If header does not exist, then upgrade primary.
		lastPrimaryNum, err = upgradePrimary(context.Background(), path, headerPath, maxFileSize, freeList)
		if err != nil {
			return nil, fmt.Errorf("error upgrading primary: %w", err)
		}

		// Header does not exist, so create new one.
		header = newHeader(maxFileSize)
		if err = writeHeader(headerPath, header); err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, err
		}

		if header.MaxFileSize != maxFileSize {
			return nil, types.ErrPrimaryWrongFileSize{header.MaxFileSize, maxFileSize}
		}

		// Find last primary file.
		lastPrimaryNum, err = findLastPrimary(path, header.FirstFile)
		if err != nil {
			return nil, err
		}
	}

	file, err := os.OpenFile(primaryFileName(path, lastPrimaryNum), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	length, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	mp := &GsfaPrimary{
		basePath:    path,
		file:        file,
		fileCache:   fileCache,
		headerPath:  headerPath,
		maxFileSize: maxFileSize,
		writer:      bufio.NewWriterSize(file, blockBufferSize),
		curPool:     newBlockPool(),
		nextPool:    newBlockPool(),

		fileNum: lastPrimaryNum,
		length:  types.Position(length),

		recFileNum: lastPrimaryNum,
		recPos:     types.Position(length),
	}

	return mp, nil
}

func (cp *GsfaPrimary) FileSize() uint32 {
	return cp.maxFileSize
}

// upgradeCachedValue updates the cached value for the given key if it exists.
// This is used to make sure that the cached value is updated when a new value
// is written to the primary (overwriting the old value), otherwise the cached
// value will be stale.
func (cp *GsfaPrimary) upgradeCachedValue(blk types.Block, key []byte, value []byte) {
	idx, ok := cp.nextPool.refs[blk]
	if ok {
		if !bytes.Equal(cp.nextPool.blocks[idx].key, key) {
			return
		}
		cp.nextPool.blocks[idx].value = value
	}
	idx, ok = cp.curPool.refs[blk]
	if ok {
		if !bytes.Equal(cp.curPool.blocks[idx].key, key) {
			return
		}
		cp.curPool.blocks[idx].value = value
	}
}

func (cp *GsfaPrimary) getCached(blk types.Block) ([]byte, []byte, error) {
	cp.poolLk.RLock()
	defer cp.poolLk.RUnlock()
	idx, ok := cp.nextPool.refs[blk]
	if ok {
		br := cp.nextPool.blocks[idx]
		return br.key, br.value, nil
	}
	idx, ok = cp.curPool.refs[blk]
	if ok {
		br := cp.curPool.blocks[idx]
		return br.key, br.value, nil
	}
	if blk.Offset >= absolutePrimaryPos(cp.recPos, cp.recFileNum, cp.maxFileSize) {
		return nil, nil, fmt.Errorf("error getting cached multihashed primary: %w", types.ErrOutOfBounds)
	}
	return nil, nil, nil
}

func (cp *GsfaPrimary) Get(blk types.Block) ([]byte, []byte, error) {
	key, value, err := cp.getCached(blk)
	if err != nil {
		return nil, nil, err
	}
	if key != nil && value != nil {
		return key, value, nil
	}

	localPos, fileNum := localizePrimaryPos(blk.Offset, cp.maxFileSize)

	file, err := cp.fileCache.Open(primaryFileName(cp.basePath, fileNum))
	if err != nil {
		return nil, nil, err
	}
	defer cp.fileCache.Close(file)

	read := make([]byte, int(blk.Size))
	if _, err = file.ReadAt(read, int64(localPos)); err != nil {
		return nil, nil, fmt.Errorf("error reading data from gsfa primary: %w", err)
	}

	return readNode(read)
}

type Pubkey []byte

// readNode extracts the pubkey from the data read and splits key and value.
func readNode(data []byte) (Pubkey, []byte, error) {
	c, n, err := readPubkey(data)
	if err != nil {
		return Pubkey{}, nil, err
	}

	return c, data[n:], nil
}

func readPubkey(buf []byte) (Pubkey, int, error) {
	// the pubkey is 32 bytes
	if len(buf) < 32 {
		return Pubkey{}, 0, fmt.Errorf("error reading pubkey from primary: expected at least 32 bytes, got %d", len(buf))
	}
	pk := buf[:32]
	return pk, 32, nil
}

// Put adds a new pending blockRecord to the pool and returns a Block that
// contains the location that the block will occupy in the primary. The
// returned primary location must be an absolute position across all primary
// files.
func (cp *GsfaPrimary) Put(key []byte, value []byte) (types.Block, error) {
	recSize := int64(len(key) + len(value))
	dataSize := primaryRecordSize
	if recSize != int64(dataSize) {
		return types.Block{}, fmt.Errorf("expected record size %d, got %d", dataSize, recSize)
	}

	cp.poolLk.Lock()
	defer cp.poolLk.Unlock()

	if cp.recPos >= types.Position(cp.maxFileSize) {
		cp.recFileNum++
		cp.recPos = 0
	}

	// Tell index the location that this record will be writtten.
	absRecPos := absolutePrimaryPos(cp.recPos, cp.recFileNum, cp.maxFileSize)
	blk := types.Block{Offset: absRecPos, Size: types.Size(recSize)}

	cp.recPos += types.Position(dataSize)

	cp.nextPool.refs[blk] = len(cp.nextPool.blocks)
	cp.nextPool.blocks = append(cp.nextPool.blocks, blockRecord{_clone(key), _clone(value)})
	cp.outstandingWork += types.Work(dataSize)
	return blk, nil
}

func (cp *GsfaPrimary) Overwrite(blk types.Block, key []byte, value []byte) error {
	recSize := int64(len(key) + len(value))

	if recSize != int64(blk.Size) {
		return fmt.Errorf("expected record size %d, got %d", blk.Size, recSize)
	}
	cp.poolLk.Lock()
	defer cp.poolLk.Unlock()

	localPos, fileNum := localizePrimaryPos(blk.Offset, cp.maxFileSize)

	fi, err := os.OpenFile(primaryFileName(cp.basePath, fileNum), os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	defer fi.Close()
	payload := append(key, value...)

	// overwrite the record
	if _, err = fi.WriteAt(payload, int64(localPos)); err != nil {
		return fmt.Errorf("error writing data to gsfa primary: %w", err)
	}
	cp.upgradeCachedValue(blk, _clone(key), _clone(value))
	return nil
}

func (cp *GsfaPrimary) flushBlock(key []byte, value []byte) (types.Work, error) {
	if cp.length >= types.Position(cp.maxFileSize) {
		fileNum := cp.fileNum + 1
		primaryPath := primaryFileName(cp.basePath, fileNum)
		// If the primary file being opened already exists then fileNum has
		// wrapped and there are max uint32 of index files. This means that
		// maxFileSize is set far too small or GC is disabled.
		if _, err := os.Stat(primaryPath); !os.IsNotExist(err) {
			return 0, fmt.Errorf("creating primary file overwrites existing, check file size, gc and path (maxFileSize=%d) (path=%s)", cp.maxFileSize, primaryPath)
		}

		file, err := os.OpenFile(primaryPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			return 0, fmt.Errorf("cannot open new primary file %s: %w", primaryPath, err)
		}
		if err = cp.writer.Flush(); err != nil {
			return 0, fmt.Errorf("cannot write to primary file %s: %w", cp.file.Name(), err)
		}

		cp.file.Close()
		cp.writer.Reset(file)
		cp.file = file
		cp.fileNum = fileNum
		cp.length = 0
	}

	size := len(key) + len(value)
	if _, err := cp.writer.Write(append(key, value...)); err != nil {
		return 0, err
	}

	writeSize := size
	cp.length += types.Position(writeSize)

	return types.Work(writeSize), nil
}

func (cp *GsfaPrimary) IndexKey(key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: %d", len(key))
	}
	// This is a sanity-check to see if it really is a solana pubkey
	decoded := solana.PublicKeyFromBytes(key)
	return decoded[:], nil
}

func (cp *GsfaPrimary) GetIndexKey(blk types.Block) ([]byte, error) {
	key, _, err := cp.Get(blk)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}
	return cp.IndexKey(key)
}

// Flush writes outstanding work and buffered data to the primary file.
func (cp *GsfaPrimary) Flush() (types.Work, error) {
	// Only one Flush at a time, otherwise the 2nd Flush can swap the pools
	// while the 1st Flush is still reading the pool being flushed. That could
	// cause the pool being read by the 1st Flush to be written to
	// concurrently.
	cp.flushLock.Lock()
	defer cp.flushLock.Unlock()

	cp.poolLk.Lock()
	// If no new data, then nothing to do.
	if len(cp.nextPool.blocks) == 0 {
		cp.poolLk.Unlock()
		return 0, nil
	}
	cp.curPool = cp.nextPool
	cp.nextPool = newBlockPool()
	cp.outstandingWork = 0
	cp.poolLk.Unlock()

	// The pool lock is released allowing Put to write to nextPool. The
	// flushLock is still held, preventing concurrent flushes from changing the
	// pools or accessing writer.

	var work types.Work
	for _, record := range cp.curPool.blocks {
		blockWork, err := cp.flushBlock(record.key, record.value)
		if err != nil {
			return 0, err
		}
		work += blockWork
	}
	err := cp.writer.Flush()
	if err != nil {
		return 0, fmt.Errorf("cannot flush data to primary file %s: %w", cp.file.Name(), err)
	}

	return work, nil
}

// Sync commits the contents of the primary file to disk. Flush should be
// called before calling Sync.
func (mp *GsfaPrimary) Sync() error {
	mp.flushLock.Lock()
	defer mp.flushLock.Unlock()
	return mp.file.Sync()
}

// Close calls Flush to write work and data to the primary file, and then
// closes the file.
func (mp *GsfaPrimary) Close() error {
	if mp.closed {
		return nil
	}

	mp.fileCache.Clear()

	_, err := mp.Flush()
	if err != nil {
		mp.file.Close()
		return err
	}

	return mp.file.Close()
}

func (cp *GsfaPrimary) OutstandingWork() types.Work {
	cp.poolLk.RLock()
	defer cp.poolLk.RUnlock()
	return cp.outstandingWork
}

type Iterator struct {
	// The index data we are iterating over
	file *os.File
	// The current position within the index
	pos int64
	// The base index file path
	base string
	// The current index file number
	fileNum uint32
}

func (cp *GsfaPrimary) Iter() (primary.PrimaryStorageIter, error) {
	header, err := readHeader(cp.headerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return NewIterator(cp.basePath, header.FirstFile), nil
}

func NewIterator(basePath string, fileNum uint32) *Iterator {
	return &Iterator{
		base:    basePath,
		fileNum: fileNum,
	}
}

func (iter *Iterator) Next() ([]byte, []byte, error) {
	if iter == nil {
		return nil, nil, nil
	}

	if iter.file == nil {
		file, err := os.OpenFile(primaryFileName(iter.base, iter.fileNum), os.O_RDONLY, 0o644)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, io.EOF
			}
			return nil, nil, err
		}
		iter.file = file
		iter.pos = 0
	}

	size := primaryRecordSize
	pos := iter.pos
	data := make([]byte, size)
	_, err := iter.file.ReadAt(data, pos)
	if err != nil {
		iter.file.Close()
		// if errors.Is(err, io.EOF) {
		// 	err = io.ErrUnexpectedEOF
		// }
		return nil, nil, err
	}

	iter.pos += int64(size)
	return readNode(data)
}

func (iter *Iterator) Close() error {
	if iter.file == nil {
		return nil
	}
	return iter.file.Close()
}

// StorageSize returns bytes of storage used by the primary files.
func (cp *GsfaPrimary) StorageSize() (int64, error) {
	header, err := readHeader(cp.headerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	fi, err := os.Stat(cp.headerPath)
	if err != nil {
		return 0, err
	}
	size := fi.Size()

	fileNum := header.FirstFile
	for {
		primaryName := primaryFileName(cp.basePath, fileNum)

		// Get size of primary file.
		fi, err = os.Stat(primaryName)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return 0, err
		}
		size += fi.Size()

		fileNum++
	}
	return size, nil
}

func primaryFileName(basePath string, fileNum uint32) string {
	return fmt.Sprintf("%s.%d", basePath, fileNum)
}

func primaryPosToFileNum(pos types.Position, maxFileSize uint32) (bool, uint32) {
	// Primary pos 0 means there is no data in the primary, so indicate empty.
	if pos == 0 {
		return false, 0
	}
	// The start of the entry determines which is file is used.
	return true, uint32(pos / types.Position(maxFileSize))
}

// localizePrimaryPos decodes a position into a local primary offset and file number.
func localizePrimaryPos(pos types.Position, maxFileSize uint32) (types.Position, uint32) {
	ok, fileNum := primaryPosToFileNum(pos, maxFileSize)
	if !ok {
		// Return 0 local pos to indicate empty bucket.
		return 0, 0
	}
	// Subtract file offset to get pos within its local file.
	localPos := pos - (types.Position(fileNum) * types.Position(maxFileSize))
	return localPos, fileNum
}

func absolutePrimaryPos(localPos types.Position, fileNum, maxFileSize uint32) types.Position {
	return types.Position(maxFileSize)*types.Position(fileNum) + localPos
}

func findLastPrimary(basePath string, fileNum uint32) (uint32, error) {
	var lastFound uint32
	for {
		_, err := os.Stat(primaryFileName(basePath, fileNum))
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return 0, err
		}
		lastFound = fileNum
		fileNum++
	}
	return lastFound, nil
}

var _ primary.PrimaryStorage = &GsfaPrimary{}
