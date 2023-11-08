package index

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rpcpool/yellowstone-faithful/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/store/primary"
	"github.com/rpcpool/yellowstone-faithful/store/primary/gsfaprimary"
	"github.com/rpcpool/yellowstone-faithful/store/types"
)

/* An append-only log [`recordlist`]s.

The format of that append only log is:

```text
    |                    Repeated                 |
    |                                             |
    |         4 bytes        |  Variable size | … |
    | Size of the Recordlist |   Recordlist   | … |
```
*/

// In-memory buckets are used to track the location of records within the index
// files. The buckets map a bit-prefix to a bucketPos value. The bucketPos
// encodes both the index file number and the record offset within that file.
// If 1GiB is the maximum size for a file, then the local data offset is kept
// in the first GiB worth of bits (30) of the bucketPos. The file number is
// kept in the bits above that. It is necessary for the file number to wrap
// before it reaches a value greater than the number of bits available to
// record it in the buckerPos. This results in a trade-off between allowing
// larger files or allowing more files, but with the same overall maximum
// storage.
//
// With a 1GiB local offset taking the first 30 bits of a 64 bit number, that
// leaves 34 bits left to encode the file number. Instead of having logic to
// wrap the file number at the largest value allowed by the available bits, the
// file number is represented as a 32-bit value that always wraps at 2^32.
//
// Since the file number wraps 2^32 this means there can never be more than
// 2^32 active index files. This also means that maxFileSize should never be
// greater than 2^32. Using a maxFileSize of 2^30, the default, and a 32-bit
// file number, results in 2 bits unused in the bucketPos address space. With a
// smaller maxFileSize more bits would be unused.
//
// Smaller values for maxFileSize result in more files needed to hold the
// index, but also more granular GC. A value too small risks running out of
// inodes on the file system, and a value too large means that there is more
// stale data that GC cannot remove. Using a 1GiB index file size limit offers
// a good balance, and this value should not be changed (other than for
// testing) by more than a factor of 4.

const (
	// IndexVersion is stored in the header data to indicate how to interpret
	// index data.
	IndexVersion = 3

	// defaultIndexSizeBits is the default number of bits in an index prefix.
	defaultIndexSizeBits = uint8(24)

	// defaultMaxFileSize is the default size at which to start a new file.
	defaultMaxFileSize = 1024 * 1024 * 1024

	// sizePrefixSize is the number of bytes used for the size prefix of a
	// record list.
	sizePrefixSize = 4

	// indexBufferSize is the size of I/O buffers. If has the same size as the
	// linux pipe size.
	indexBufferSize = 16 * 4096

	// bucketPoolSize is the bucket cache size.
	bucketPoolSize = 1024

	// deletedBit is the highest order bit in the uint32 size part of a file
	// record, and when set, indicates that the record is deleted. This means
	// that record sizes must be less than 2^31.
	deletedBit = uint32(1 << 31)
)

// stripBucketPrefix removes the prefix that is used for the bucket.
//
// The first bits of a key are used to determine the bucket to put the key
// into. This function removes those bytes. Only bytes that are fully covered
// by the bits are removed. E.g. a bit value of 19 will remove only 2 bytes,
// whereas 24 bits removes 3 bytes.
func stripBucketPrefix(key []byte, bits byte) []byte {
	prefixLen := int(bits / 8)
	if len(key) < prefixLen {
		return nil
	}
	return key[prefixLen:]
}

type Index struct {
	sizeBits          uint8
	maxFileSize       uint32
	buckets           Buckets
	file              *os.File
	fileNum           uint32
	headerPath        string
	writer            *bufio.Writer
	Primary           primary.PrimaryStorage
	bucketLk          sync.RWMutex
	flushLock         sync.Mutex
	outstandingWork   types.Work
	curPool, nextPool bucketPool
	length            types.Position
	basePath          string
	fileCache         *filecache.FileCache
	closeOnce         sync.Once

	gcDone     chan struct{}
	gcResumeAt uint32
	gcResume   bool
	gcStop     chan struct{}
}

type bucketPool map[BucketIndex][]byte

// Open opens the index for the given primary. The index is created if there is
// no existing index at the specified path. If there is an older version index,
// then it is automatically upgraded.
//
// Specifying 0 for indexSizeBits and maxFileSize results in using the values
// of the existing index, or default values if no index exists. A gcInterval of
// 0 disables garbage collection.
func Open(ctx context.Context, path string, primary primary.PrimaryStorage, indexSizeBits uint8, maxFileSize uint32, gcInterval, gcTimeLimit time.Duration, fileCache *filecache.FileCache) (*Index, error) {
	var file *os.File
	headerPath := headerName(path)

	if indexSizeBits != 0 && (indexSizeBits > 31 || indexSizeBits < 8) {
		return nil, fmt.Errorf("indexSizeBits must be between 8 and 31, default is %d", defaultIndexSizeBits)
	}
	if maxFileSize > defaultMaxFileSize {
		return nil, fmt.Errorf("maximum file size cannot exceed %d", defaultMaxFileSize)
	}

	upgradeFileSize := maxFileSize
	if upgradeFileSize == 0 {
		upgradeFileSize = defaultMaxFileSize
	}
	err := upgradeIndex(ctx, path, headerPath, upgradeFileSize)
	if err != nil {
		return nil, fmt.Errorf("could not upgrade index: %w", err)
	}

	var existingHeader bool
	header, err := readHeader(headerPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if indexSizeBits == 0 {
			indexSizeBits = defaultIndexSizeBits
		}
		if maxFileSize == 0 {
			maxFileSize = defaultMaxFileSize
		}
		header = newHeader(indexSizeBits, maxFileSize)
		mp, ok := primary.(*gsfaprimary.GsfaPrimary)
		if ok {
			header.PrimaryFileSize = mp.FileSize()
		}
		if err = writeHeader(headerPath, header); err != nil {
			return nil, err
		}
	} else {
		existingHeader = true
		if indexSizeBits == 0 {
			indexSizeBits = header.BucketsBits
		}
		if maxFileSize == 0 {
			maxFileSize = header.MaxFileSize
		}
	}

	buckets, err := NewBuckets(indexSizeBits)
	if err != nil {
		return nil, err
	}

	var rmPool bucketPool
	var lastIndexNum uint32
	if existingHeader {
		if header.BucketsBits != indexSizeBits {
			return nil, types.ErrIndexWrongBitSize{header.BucketsBits, indexSizeBits}
		}

		if header.MaxFileSize != maxFileSize {
			return nil, types.ErrIndexWrongFileSize{header.MaxFileSize, maxFileSize}
		}

		err = loadBucketState(ctx, path, buckets, maxFileSize)
		if err != nil {
			log.Warnw("Could not load bucket state, scanning index file", "err", err)
			lastIndexNum, err = scanIndex(ctx, path, header.FirstFile, buckets, maxFileSize)
			if err != nil {
				return nil, err
			}
		} else {
			lastIndexNum, err = findLastIndex(path, header.FirstFile)
			if err != nil {
				return nil, fmt.Errorf("could not find most recent index file: %w", err)
			}
		}

		mp, ok := primary.(*gsfaprimary.GsfaPrimary)
		if ok {
			switch header.PrimaryFileSize {
			case 0:
				// Primary file size is not yet known, so may need to remap index.
				rmPool, err = remapIndex(ctx, mp, buckets, path, headerPath, header)
				if err != nil {
					return nil, err
				}
			case mp.FileSize():
			default:
				return nil, types.ErrPrimaryWrongFileSize{mp.FileSize(), header.PrimaryFileSize}
			}
		}
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	file, err = openFileAppend(indexFileName(path, lastIndexNum))
	if err != nil {
		return nil, err
	}

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	idx := &Index{
		sizeBits:    indexSizeBits,
		maxFileSize: maxFileSize,
		buckets:     buckets,
		file:        file,
		fileNum:     lastIndexNum,
		headerPath:  headerPath,
		writer:      bufio.NewWriterSize(file, indexBufferSize),
		Primary:     primary,
		nextPool:    make(bucketPool, bucketPoolSize),
		length:      types.Position(fi.Size()),
		basePath:    path,
		fileCache:   fileCache,
	}

	if len(rmPool) != 0 {
		idx.nextPool = rmPool
		idx.Flush()
		idx.curPool = nil
	}

	if gcInterval == 0 {
		log.Warn("Index garbage collection disabled")
	} else {
		idx.gcDone = make(chan struct{})
		idx.gcStop = make(chan struct{})
		go idx.garbageCollector(gcInterval, gcTimeLimit)
	}

	return idx, nil
}

func indexFileName(basePath string, fileNum uint32) string {
	return fmt.Sprintf("%s.%d", basePath, fileNum)
}

func savedBucketsName(basePath string) string {
	return basePath + ".buckets"
}

func scanIndex(ctx context.Context, basePath string, fileNum uint32, buckets Buckets, maxFileSize uint32) (uint32, error) {
	var lastFileNum uint32
	for {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		err := scanIndexFile(ctx, basePath, fileNum, buckets, maxFileSize)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return 0, fmt.Errorf("error scanning index file %s: %w", indexFileName(basePath, fileNum), err)
		}
		lastFileNum = fileNum
		fileNum++
	}
	return lastFileNum, nil
}

// StorageSize returns bytes of storage used by the index files.
func (idx *Index) StorageSize() (int64, error) {
	header, err := readHeader(idx.headerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	fi, err := os.Stat(idx.headerPath)
	if err != nil {
		return 0, err
	}
	size := fi.Size()

	fileNum := header.FirstFile
	for {
		fi, err = os.Stat(indexFileName(idx.basePath, fileNum))
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

func scanIndexFile(ctx context.Context, basePath string, fileNum uint32, buckets Buckets, maxFileSize uint32) error {
	indexPath := indexFileName(basePath, fileNum)

	// This is a single sequential read across the index file.
	file, err := openFileForScan(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat index file: %w", err)
	}
	if fi.Size() == 0 {
		return nil
	}

	sizeBuffer := make([]byte, sizePrefixSize)
	scratch := make([]byte, 256)
	var pos int64
	var i int
	for {
		if _, err = file.ReadAt(sizeBuffer, pos); err != nil {
			if errors.Is(err, io.EOF) {
				// Finished reading entire index.
				break
			}
			if err == io.ErrUnexpectedEOF {
				log.Errorw("Unexpected EOF scanning index", "file", indexPath)
				file.Close()
				// Cut off incomplete data
				e := os.Truncate(indexPath, pos)
				if e != nil {
					log.Errorw("Error truncating file", "err", e, "file", indexPath)
				}
				break
			}
			return err
		}
		pos += sizePrefixSize

		size := binary.LittleEndian.Uint32(sizeBuffer)
		if size&deletedBit != 0 {
			// Record is deleted, so skip.
			pos += int64(size ^ deletedBit)
			continue
		}

		if int(size) > len(scratch) {
			scratch = make([]byte, size)
		}
		data := scratch[:size]
		if _, err = file.ReadAt(data, pos); err != nil {
			if err == io.ErrUnexpectedEOF || errors.Is(err, io.EOF) {
				// The file is corrupt since the expected data could not be
				// read. Take the usable data and move on.
				log.Errorw("Unexpected EOF scanning index record", "file", indexPath)
				file.Close()
				// Cut off incomplete data
				e := os.Truncate(indexPath, pos-sizePrefixSize)
				if e != nil {
					log.Errorw("Error truncating file", "err", e, "file", indexPath)
				}
				break
			}
			return err
		}

		i++
		if i&1023 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}

		bucketPrefix := BucketIndex(binary.LittleEndian.Uint32(data))
		err = buckets.Put(bucketPrefix, localPosToBucketPos(pos, fileNum, maxFileSize))
		if err != nil {
			return err
		}
		pos += int64(size)
	}
	log.Infof("Scanned %s", indexPath)
	return nil
}

// Put puts a key together with a file offset into the index.
//
// The key needs to be a cryptographically secure hash that is at least 4 bytes
// long.
func (idx *Index) Put(key []byte, location types.Block) error {
	// Get record list and bucket index
	bucket, err := idx.getBucketIndex(key)
	if err != nil {
		return err
	}

	// The key does not need the prefix that was used to find the right
	// bucket. For simplicity only full bytes are trimmed off.
	indexKey := stripBucketPrefix(key, idx.sizeBits)

	idx.bucketLk.Lock()
	defer idx.bucketLk.Unlock()

	records, err := idx.getRecordsFromBucket(bucket)
	if err != nil {
		return err
	}

	// No records stored in that bucket yet
	var newData []byte
	if records == nil {
		// As it's the first key a single byte is enough as it does not need to
		// be distinguished from other keys.
		trimmedIndexKey := indexKey[:1]
		newData = EncodeKeyPosition(KeyPositionPair{trimmedIndexKey, location})
	} else {
		// Read the record list from disk and insert the new key
		pos, prevRecord, has := records.FindKeyPosition(indexKey)

		if has && bytes.HasPrefix(indexKey, prevRecord.Key) {
			// The previous key is fully contained in the current key. We need to read the full
			// key from the main data file in order to retrieve a key that is distinguishable
			// from the one that should get inserted.
			fullPrevKey, err := idx.Primary.GetIndexKey(prevRecord.Block)
			if err != nil {
				return fmt.Errorf("error reading previous key from primary: %w", err)
			}
			// The index key has already removed the prefix that is used to determine the
			// bucket. Do the same for the full previous key.
			prevKey := stripBucketPrefix(fullPrevKey, idx.sizeBits)
			if prevKey == nil {
				// The previous key, read from the primary, was bad. This means
				// that the data in the primary at prevRecord.Bucket is not
				// good, or that data in the index is bad and prevRecord.Bucket
				// has a wrong location in the primary.  Log the error with
				// diagnostic information.
				cached, indexOffset, fileNum, err := idx.readBucketInfo(bucket)
				if err != nil {
					log.Errorw("Cannot read bucket", "err", err)
				} else {
					msg := "Read bad pevious key data, too short"
					if cached == nil {
						log.Errorw(msg, "offset", indexOffset, "size", indexFileName(idx.basePath, fileNum))
					} else {
						log.Error(msg)
					}
				}
				// Either way, the previous key record is not usable, so
				// overwrite it with a record for the new key.  Use the same
				// key in the index record as the previous record, since the
				// previous key is being replaced so there is no need to
				// differentiate old from new.
				//
				// This results in the data for the previous keys being lost,
				// but it may not have been present in the first place, in which
				// case that was the cause of this problem.
				newData = records.PutKeys([]KeyPositionPair{{prevRecord.Key, location}}, prevRecord.Pos, pos)
				idx.outstandingWork += types.Work(len(newData) + BucketPrefixSize + sizePrefixSize)
				idx.nextPool[bucket] = newData
				return nil
			}

			keyTrimPos := firstNonCommonByte(indexKey, prevKey)
			// Only store the new key if it doesn't exist yet.
			if keyTrimPos >= len(indexKey) {
				return nil
			}

			trimmedPrevKey := prevKey
			if keyTrimPos < len(prevKey) {
				trimmedPrevKey = prevKey[:keyTrimPos+1]
			} else {
				// trimmedPrevKey should always be a prefix. since it is not
				// here, collect some diagnostic logs.
				cached, indexOffset, fileNum, err := idx.readBucketInfo(bucket)
				if err != nil {
					log.Errorw("Cannot read bucket", "err", err)
				} else {
					msg := "Read bad pevious key data"
					if cached == nil {
						log.Errorw(msg, "offset", indexOffset, "size", indexFileName(idx.basePath, fileNum))
					} else {
						log.Error(msg)
					}
				}
			}
			trimmedIndexKey := indexKey[:keyTrimPos+1]
			var keys []KeyPositionPair

			// Replace the existing previous key (which is too short) with a
			// new one and also insert the new key.
			if bytes.Compare(trimmedPrevKey, trimmedIndexKey) == -1 {
				keys = []KeyPositionPair{
					{trimmedPrevKey, prevRecord.Block},
					{trimmedIndexKey, location},
				}
			} else {
				keys = []KeyPositionPair{
					{trimmedIndexKey, location},
					{trimmedPrevKey, prevRecord.Block},
				}
			}
			newData = records.PutKeys(keys, prevRecord.Pos, pos)
			// There is no need to do anything with the next key as the next key is
			// already guaranteed to be distinguishable from the new key as it was already
			// distinguishable from the previous key.
		} else {
			// The previous key is not fully contained in the key that should get inserted.
			// Hence we only need to trim the new key to the smallest one possible that is
			// still distinguishable from the previous (in case there is one) and next key
			// (in case there is one).
			prevRecordNonCommonBytePos := 0
			if has {
				prevRecordNonCommonBytePos = firstNonCommonByte(indexKey, prevRecord.Key)
			}
			// The new record will not be the last record.
			nextRecordNonCommonBytePos := 0
			if pos < records.Len() {
				// In order to determine the minimal key size, we need to get
				// the next key as well.
				nextRecord := records.ReadRecord(pos)
				nextRecordNonCommonBytePos = firstNonCommonByte(indexKey, nextRecord.Key)
			}

			// Minimum prefix of the key that is different in at least one byte
			// from the previous as well as the next key.
			minPrefix := max(
				prevRecordNonCommonBytePos,
				nextRecordNonCommonBytePos,
			)

			// We cannot trim beyond the key length.
			keyTrimPos := min(minPrefix, len(indexKey)-1)

			trimmedIndexKey := indexKey[:keyTrimPos+1]
			newData = records.PutKeys([]KeyPositionPair{{trimmedIndexKey, location}}, pos, pos)
		}
	}
	idx.outstandingWork += types.Work(len(newData) + BucketPrefixSize + sizePrefixSize)
	idx.nextPool[bucket] = newData
	return nil
}

// Update updates a key together with a file offset into the index.
func (idx *Index) Update(key []byte, location types.Block) error {
	// Get record list and bucket index
	bucket, err := idx.getBucketIndex(key)
	if err != nil {
		return err
	}

	// The key does not need the prefix that was used to find its bucket. For
	// simplicity only full bytes are trimmed off.
	indexKey := stripBucketPrefix(key, idx.sizeBits)

	idx.bucketLk.Lock()
	defer idx.bucketLk.Unlock()
	records, err := idx.getRecordsFromBucket(bucket)
	if err != nil {
		return err
	}

	var newData []byte
	// If no records are stored in that bucket yet, it means there is no key to
	// be updated.
	if records == nil {
		return fmt.Errorf("no records found in index, unable to update key")
	}

	// Read the record list to find the key and position.
	r := records.GetRecord(indexKey)
	if r == nil {
		return fmt.Errorf("key to update not found in index")
	}
	// Update key in position.
	newData = records.PutKeys([]KeyPositionPair{{r.Key, location}}, r.Pos, r.NextPos())

	idx.outstandingWork += types.Work(len(newData) + BucketPrefixSize + sizePrefixSize)
	idx.nextPool[bucket] = newData
	return nil
}

// Remove removes a key from the index.
func (idx *Index) Remove(key []byte) (bool, error) {
	// Get record list and bucket index
	bucket, err := idx.getBucketIndex(key)
	if err != nil {
		return false, err
	}

	// The key does not need the prefix that was used to find its bucket. For
	// simplicity only full bytes are trimmed off.
	indexKey := stripBucketPrefix(key, idx.sizeBits)

	idx.bucketLk.Lock()
	defer idx.bucketLk.Unlock()

	records, err := idx.getRecordsFromBucket(bucket)
	if err != nil {
		return false, err
	}

	// If no records are stored in that bucket yet, it means there is no key to
	// be removed.
	if records == nil {
		// No records in index. Nothing to remove.
		return false, nil
	}

	// Read the record list to find the key and its position.
	r := records.GetRecord(indexKey)
	if r == nil {
		// The record does not exist. Nothing to remove.
		return false, nil
	}

	// Remove key from record.
	newData := records.PutKeys([]KeyPositionPair{}, r.Pos, r.NextPos())
	// NOTE: We are removing the key without changing any keys. If we want
	// to optimize for storage we need to check the keys with the same prefix
	// and see if any of them can be shortened. This process will be similar
	// to finding where to put a new key.

	idx.outstandingWork += types.Work(len(newData) + BucketPrefixSize + sizePrefixSize)
	idx.nextPool[bucket] = newData
	return true, nil
}

func (idx *Index) getBucketIndex(key []byte) (BucketIndex, error) {
	if len(key) < 4 {
		return 0, types.ErrKeyTooShort
	}

	// Determine which bucket a key falls into. Use the first few bytes of they
	// key for it and interpret them as a little-endian integer.
	prefix := BucketIndex(binary.LittleEndian.Uint32(key))
	var leadingBits BucketIndex = (1 << idx.sizeBits) - 1
	return prefix & leadingBits, nil
}

// getRecordsFromBucket returns the recordList and bucket the key belongs to.
func (idx *Index) getRecordsFromBucket(bucket BucketIndex) (RecordList, error) {
	// Get the index file offset of the record list the key is in.
	cached, indexOffset, fileNum, err := idx.readBucketInfo(bucket)
	if err != nil {
		return nil, fmt.Errorf("error reading bucket info: %w", err)
	}
	var records RecordList
	if cached != nil {
		records = NewRecordListRaw(cached)
	} else {
		records, err = idx.readDiskBucket(indexOffset, fileNum)
		if err != nil {
			return nil, fmt.Errorf("error reading index records from disk: %w", err)
		}
	}
	return records, nil
}

func (idx *Index) flushBucket(bucket BucketIndex, newData []byte) (types.Block, types.Work, error) {
	if idx.length >= types.Position(idx.maxFileSize) {
		fileNum := idx.fileNum + 1
		indexPath := indexFileName(idx.basePath, fileNum)
		// If the index file being opened already exists then fileNum has
		// wrapped and there are max uint32 of index files. This means that
		// maxFileSize is set far too small or GC is disabled.
		if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
			log.Warnw("Creating index file overwrites existing. Check that file size limit is not too small resulting in too many files.",
				"maxFileSize", idx.maxFileSize, "indexPath", indexPath)
		}
		file, err := openFileAppend(indexPath)
		if err != nil {
			return types.Block{}, 0, fmt.Errorf("cannot open new index file %s: %w", indexPath, err)
		}
		if err = idx.writer.Flush(); err != nil {
			return types.Block{}, 0, fmt.Errorf("cannot write to index file %s: %w", idx.file.Name(), err)
		}
		idx.file.Close()
		idx.writer.Reset(file)
		idx.file = file
		idx.fileNum = fileNum
		idx.length = 0
	}

	// Write new data to disk. The record list is prefixed with the bucket they
	// are in. This is needed in order to reconstruct the in-memory buckets
	// from the index itself.
	//
	// If the size of new data is too large to fit in 30 bits, then a bigger
	// bit prefix, or s 2nd level index is needed.
	//
	// TODO: If size >= 1<<30 then the size takes up the following 62 bits.
	newDataSize := make([]byte, sizePrefixSize)
	binary.LittleEndian.PutUint32(newDataSize, uint32(len(newData))+uint32(BucketPrefixSize))
	_, err := idx.writer.Write(newDataSize)
	if err != nil {
		return types.Block{}, 0, err
	}

	bucketPrefixBuffer := make([]byte, BucketPrefixSize)
	binary.LittleEndian.PutUint32(bucketPrefixBuffer, uint32(bucket))
	if _, err = idx.writer.Write(bucketPrefixBuffer); err != nil {
		return types.Block{}, 0, err
	}

	if _, err = idx.writer.Write(newData); err != nil {
		return types.Block{}, 0, err
	}
	length := idx.length
	toWrite := types.Position(len(newData) + BucketPrefixSize + sizePrefixSize)
	idx.length += toWrite
	// Fsyncs are expensive, so do not do them here; do in explicit Sync().

	// Keep the reference to the stored data in the bucket.
	return types.Block{
		Offset: localPosToBucketPos(int64(length+sizePrefixSize), idx.fileNum, idx.maxFileSize),
		Size:   types.Size(len(newData) + BucketPrefixSize),
	}, types.Work(toWrite), nil
}

type bucketBlock struct {
	bucket BucketIndex
	blk    types.Block
}

func (idx *Index) readCached(bucket BucketIndex) ([]byte, bool) {
	data, ok := idx.nextPool[bucket]
	if ok {
		return data, true
	}
	data, ok = idx.curPool[bucket]
	if ok {
		return data, true
	}
	return nil, false
}

func (idx *Index) readBucketInfo(bucket BucketIndex) ([]byte, types.Position, uint32, error) {
	data, ok := idx.readCached(bucket)
	if ok {
		return data, 0, 0, nil
	}
	bucketPos, err := idx.buckets.Get(bucket)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("error reading bucket: %w", err)
	}
	localPos, fileNum := localizeBucketPos(bucketPos, idx.maxFileSize)
	return nil, localPos, fileNum, nil
}

func (idx *Index) readDiskBucket(indexOffset types.Position, fileNum uint32) (RecordList, error) {
	// indexOffset should never be 0 if there is a bucket, because it is always
	// at lease sizePrefixSize into the stored data.
	if indexOffset == 0 {
		return nil, nil
	}

	file, err := idx.fileCache.Open(indexFileName(idx.basePath, fileNum))
	if err != nil {
		return nil, err
	}
	defer idx.fileCache.Close(file)

	// Read the record list from disk and get the file offset of that key in
	// the primary storage.
	sizeBuf := make([]byte, sizePrefixSize)
	if _, err = file.ReadAt(sizeBuf, int64(indexOffset-4)); err != nil {
		return nil, err
	}
	data := make([]byte, binary.LittleEndian.Uint32(sizeBuf))
	if _, err = file.ReadAt(data, int64(indexOffset)); err != nil {
		return nil, err
	}
	return NewRecordList(data), nil
}

// Get the file offset in the primary storage of a key.
func (idx *Index) Get(key []byte) (types.Block, bool, error) {
	// Get record list and bucket index.
	bucket, err := idx.getBucketIndex(key)
	if err != nil {
		return types.Block{}, false, err
	}

	// Here we just need an RLock since there will not be changes over buckets.
	// So, do not use getRecordsFromBucket and instead only wrap this line of
	// code in the RLock.
	idx.bucketLk.RLock()
	cached, indexOffset, fileNum, err := idx.readBucketInfo(bucket)
	idx.bucketLk.RUnlock()
	if err != nil {
		return types.Block{}, false, fmt.Errorf("error reading bucket: %w", err)
	}
	var records RecordList
	if cached != nil {
		records = NewRecordListRaw(cached)
	} else {
		records, err = idx.readDiskBucket(indexOffset, fileNum)
		if err != nil {
			return types.Block{}, false, fmt.Errorf("error reading index records from disk: %w", err)
		}
	}
	if records == nil {
		return types.Block{}, false, nil
	}

	// The key does not need the prefix that was used to find its bucket. For
	// simplicity only full bytes are trimmed off.
	indexKey := stripBucketPrefix(key, idx.sizeBits)

	fileOffset, found := records.Get(indexKey)
	return fileOffset, found, nil
}

// Flush writes outstanding work and buffered data to the current index file
// and updates buckets.
func (idx *Index) Flush() (types.Work, error) {
	// Only one Flush at a time, otherwise the 2nd Flush can swap the pools
	// while the 1st Flush is still reading the pool being flushed. That could
	// cause the pool being read by the 1st Flush to be written to
	// concurrently.
	idx.flushLock.Lock()
	defer idx.flushLock.Unlock()

	idx.bucketLk.Lock()
	// If no new data, then nothing to do.
	if len(idx.nextPool) == 0 {
		idx.bucketLk.Unlock()
		return 0, nil
	}
	idx.curPool = idx.nextPool
	idx.nextPool = make(bucketPool, bucketPoolSize)
	idx.outstandingWork = 0
	idx.bucketLk.Unlock()

	blks := make([]bucketBlock, 0, len(idx.curPool))
	var work types.Work
	for bucket, data := range idx.curPool {
		blk, newWork, err := idx.flushBucket(bucket, data)
		if err != nil {
			return 0, err
		}
		blks = append(blks, bucketBlock{bucket, blk})
		work += newWork
	}
	err := idx.writer.Flush()
	if err != nil {
		return 0, fmt.Errorf("cannot flush data to index file %s: %w", idx.file.Name(), err)
	}
	idx.bucketLk.Lock()
	defer idx.bucketLk.Unlock()
	for _, blk := range blks {
		if err = idx.buckets.Put(blk.bucket, blk.blk.Offset); err != nil {
			return 0, fmt.Errorf("error commiting bucket: %w", err)
		}
	}

	return work, nil
}

// Sync commits the contents of the current index file to disk. Flush should be
// called before calling Sync.
func (idx *Index) Sync() error {
	idx.flushLock.Lock()
	defer idx.flushLock.Unlock()
	return idx.file.Sync()
}

// Close calls Flush to write work and data to the current index file, and then
// closes the file.
func (idx *Index) Close() error {
	var err error
	idx.closeOnce.Do(func() {
		idx.fileCache.Clear()
		if idx.gcStop != nil {
			close(idx.gcStop)
			<-idx.gcDone
			idx.gcStop = nil
		}
		_, err = idx.Flush()
		if err != nil {
			idx.file.Close()
			return
		}
		if err = idx.file.Close(); err != nil {
			return
		}
		err = idx.saveBucketState()
	})
	return err
}

func (idx *Index) saveBucketState() error {
	bucketsFileName := savedBucketsName(idx.basePath)
	bucketsFileNameTemp := bucketsFileName + ".tmp"

	file, err := os.Create(bucketsFileNameTemp)
	if err != nil {
		return err
	}
	writer := bufio.NewWriterSize(file, indexBufferSize)
	buf := make([]byte, types.OffBytesLen)

	for _, offset := range idx.buckets {
		binary.LittleEndian.PutUint64(buf, uint64(offset))

		_, err = writer.Write(buf)
		if err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}

	// Only create the file after saving all buckets.
	return os.Rename(bucketsFileNameTemp, bucketsFileName)
}

func loadBucketState(ctx context.Context, basePath string, buckets Buckets, maxFileSize uint32) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	bucketsFileName := savedBucketsName(basePath)
	file, err := os.Open(bucketsFileName)
	if err != nil {
		return err
	}
	defer func() {
		e := file.Close()
		if e != nil {
			log.Error("Error closing saved buckets file", "err", err)
		}
		if e = os.Remove(bucketsFileName); e != nil {
			log.Error("Error removing saved buckets file", "err", err)
		}
	}()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	// If the file is not the expected size then do not use it.
	if fi.Size() != int64(types.OffBytesLen*len(buckets)) {
		return fmt.Errorf("bucket state file is wrong size, expected %d actual %d", types.OffBytesLen*len(buckets), fi.Size())
	}

	reader := bufio.NewReaderSize(file, indexBufferSize)
	buf := make([]byte, types.OffBytesLen)

	for i := 0; i < len(buckets); i++ {
		// Read offset from bucket state.
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return err
		}
		buckets[i] = types.Position(binary.LittleEndian.Uint64(buf))
	}

	return nil
}

func RemoveSavedBuckets(basePath string) error {
	err := os.Remove(savedBucketsName(basePath))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (i *Index) OutstandingWork() types.Work {
	i.bucketLk.RLock()
	defer i.bucketLk.RUnlock()
	return i.outstandingWork
}

// RawIterator iterates raw index file entries, whether or not they are still
// valid. Deleted entries are skipped.
//
// This is primarily to inspect index files for testing.
type RawIterator struct {
	// The index data we are iterating over
	file *os.File
	// The current position within the index
	pos int64
	// The base index file path
	base string
	// The current index file number
	fileNum uint32
}

func NewRawIterator(basePath string, fileNum uint32) *RawIterator {
	return &RawIterator{
		base:    basePath,
		fileNum: fileNum,
	}
}

func (iter *RawIterator) Next() ([]byte, types.Position, bool, error) {
	if iter.file == nil {
		file, err := openFileForScan(indexFileName(iter.base, iter.fileNum))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, 0, true, nil
			}
			return nil, 0, false, err
		}
		iter.file = file
		iter.pos = 0
	}

	var size uint32
	sizeBuf := make([]byte, sizePrefixSize)
	for {
		_, err := iter.file.ReadAt(sizeBuf, iter.pos)
		if err != nil {
			iter.file.Close()
			if errors.Is(err, io.EOF) {
				iter.file = nil
				iter.fileNum++
				return iter.Next()
			}
			return nil, 0, false, err
		}
		size = binary.LittleEndian.Uint32(sizeBuf)
		if size&deletedBit != 0 {
			size ^= deletedBit
			iter.pos += int64(sizePrefixSize + size)
		} else {
			break
		}
	}
	pos := iter.pos + int64(sizePrefixSize)
	data := make([]byte, size)
	_, err := iter.file.ReadAt(data, pos)
	if err != nil {
		iter.file.Close()
		return nil, 0, false, err
	}

	iter.pos += int64(sizePrefixSize + size)
	return data, types.Position(pos), false, nil
}

func (iter *RawIterator) Close() error {
	if iter.file == nil {
		return nil
	}
	return iter.file.Close()
}

// Iterator is an iterator over only valid index entries.
//
// On each iteration it returns the position of the record within the index
// together with the raw record list data.
type Iterator struct {
	// bucketIndex is the next bucket to iterate.
	bucketIndex BucketIndex
	// index is the Index being iterated.
	index  *Index
	rlIter *RecordListIter
}

func (idx *Index) NewIterator() *Iterator {
	return &Iterator{
		index: idx,
	}
}

// Progress returns the percentage of buckets iterated.
func (iter *Iterator) Progress() float64 {
	return 100.0 * float64(iter.bucketIndex) / float64(len(iter.index.buckets))
}

func (iter *Iterator) Next() (Record, bool, error) {
	if iter.rlIter != nil {
		if !iter.rlIter.Done() {
			return iter.rlIter.Next(), false, nil
		}
		iter.rlIter = nil
	}

	iter.index.flushLock.Lock()
	defer iter.index.flushLock.Unlock()

next:
	iter.index.bucketLk.RLock()
	var bucketPos types.Position
	for {
		if int(iter.bucketIndex) >= len(iter.index.buckets) {
			iter.index.bucketLk.RUnlock()
			return Record{}, true, nil
		}
		bucketPos = iter.index.buckets[iter.bucketIndex]
		if bucketPos != 0 {
			// Got non-empty bucket.
			break
		}
		iter.bucketIndex++
	}
	iter.index.bucketLk.RUnlock()

	data, cached := iter.index.readCached(iter.bucketIndex)
	if cached {
		// Add the size prefix to the record data.
		newData := make([]byte, len(data)+sizePrefixSize)
		binary.LittleEndian.PutUint32(newData, uint32(len(data)))
		copy(newData[sizePrefixSize:], data)
		data = newData
	} else {
		localPos, fileNum := localizeBucketPos(bucketPos, iter.index.maxFileSize)
		file, err := iter.index.fileCache.Open(indexFileName(iter.index.basePath, fileNum))
		if err != nil {
			return Record{}, false, err
		}
		defer iter.index.fileCache.Close(file)

		sizeBuf := make([]byte, sizePrefixSize)
		if _, err = file.ReadAt(sizeBuf, int64(localPos-sizePrefixSize)); err != nil {
			return Record{}, false, err
		}
		data = make([]byte, binary.LittleEndian.Uint32(sizeBuf))
		if _, err = file.ReadAt(data, int64(localPos)); err != nil {
			return Record{}, false, err
		}
	}
	iter.bucketIndex++

	rl := NewRecordList(data)
	iter.rlIter = rl.Iter()
	if iter.rlIter.Done() {
		iter.rlIter = nil
		goto next
	}

	return iter.rlIter.Next(), false, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// firstNonCommonByte returns the position of the first character that both
// given slices have not in common.
//
// It might return an index that is bigger than the input strings. If one is
// full prefix of the other, the index will be `shorterSlice.len() + 1`, if
// both slices are equal it will be `slice.len() + 1`
func firstNonCommonByte(aa []byte, bb []byte) int {
	smallerLength := min(len(aa), len(bb))
	index := 0
	for ; index < smallerLength; index++ {
		if aa[index] != bb[index] {
			break
		}
	}
	return index
}

func openFileAppend(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
}

func openFileForScan(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY, 0o644)
}

func bucketPosToFileNum(pos types.Position, maxFileSize uint32) (bool, uint32) {
	// Bucket pos 0 means there is no data in the bucket, so indicate empty bucket.
	if pos == 0 {
		return false, 0
	}
	// The start of the entry, not the position of the record, determines which
	// is file is used.  The record begins sizePrefixSize before pos.  This
	// matters only if pos is slightly after a maxFileSize boundry, but
	// the adjusted position is not.
	return true, uint32((pos - sizePrefixSize) / types.Position(maxFileSize))
}

func localPosToBucketPos(pos int64, fileNum, maxFileSize uint32) types.Position {
	// Valid position must be non-zero, at least sizePrefixSize.
	if pos == 0 {
		panic("invalid local offset")
	}
	// fileNum is a 32bit value and will wrap at 4GiB, So 4294967296 is the
	// maximum number of index files possible.
	return types.Position(fileNum)*types.Position(maxFileSize) + types.Position(pos)
}

// localizeBucketPos decodes a bucketPos into a local pos and file number.
func localizeBucketPos(pos types.Position, maxFileSize uint32) (types.Position, uint32) {
	ok, fileNum := bucketPosToFileNum(pos, maxFileSize)
	if !ok {
		// Return 0 local pos to indicate empty bucket.
		return 0, 0
	}
	// Subtract file offset to get pos within its local file.
	localPos := pos - (types.Position(fileNum) * types.Position(maxFileSize))
	return localPos, fileNum
}

func findLastIndex(basePath string, fileNum uint32) (uint32, error) {
	var lastFound uint32
	for {
		_, err := os.Stat(indexFileName(basePath, fileNum))
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

func copyFile(src, dst string) error {
	fin, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fin.Close()

	fout, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fout.Close()

	_, err = io.Copy(fout, fin)
	return err
}

// remapIndex updates all the primary offset in each record from the location
// in the single primary file to the correct location in the separate primary
// files. Remapping is done on a copy of each original index file so that if
// remapping the index files is not completed, there are no files in a
// partially remapped state. This allows remapping to resume from where it left
// off, without corrupting any files that were already remapped.
func remapIndex(ctx context.Context, mp *gsfaprimary.GsfaPrimary, buckets Buckets, basePath, headerPath string, header Header) (bucketPool, error) {
	remapper, err := mp.NewIndexRemapper()
	if err != nil {
		return nil, err
	}
	if remapper == nil {
		// Update the header to indicate remapping is completed.
		header.PrimaryFileSize = mp.FileSize()
		return nil, writeHeader(headerPath, header)
	}

	log.Infow("Remapping primary offsets in index")

	maxFileSize := header.MaxFileSize
	fileBuckets := make(map[uint32][]int)

	var indexTotal int
	for i, offset := range buckets {
		ok, fileNum := bucketPosToFileNum(offset, maxFileSize)
		if ok {
			fileBuckets[fileNum] = append(fileBuckets[fileNum], i)
			indexTotal++
		}
	}

	var rmPool bucketPool
	var fileCount, indexCount, recordCount int
	var scratch []byte
	sizeBuf := make([]byte, sizePrefixSize)

	for fileNum, bucketPrefixes := range fileBuckets {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fileName := indexFileName(basePath, fileNum)
		tmpName := fileName + ".tmp"
		doneName := fileName + ".remapped"

		// If this file was already remapped, skip it.
		_, err = os.Stat(doneName)
		if !os.IsNotExist(err) {
			log.Infow("index file already remapped", "file", fileName)
			indexCount += len(bucketPrefixes)
			continue
		}

		err = copyFile(fileName, tmpName)
		if err != nil {
			return nil, err
		}

		file, err := os.OpenFile(tmpName, os.O_RDWR, 0o644)
		if err != nil {
			return nil, fmt.Errorf("remap cannot open index file %s: %w", fileName, err)
		}

		for _, pfx := range bucketPrefixes {
			// Read the record list from disk and remap the primary file offset
			// in each record in the record list.
			localPos := buckets[pfx] - (types.Position(fileNum) * types.Position(maxFileSize))
			if _, err = file.ReadAt(sizeBuf, int64(localPos-sizePrefixSize)); err != nil {
				return nil, fmt.Errorf("cannot read record list size from index file %s: %w", file.Name(), err)
			}
			size := binary.LittleEndian.Uint32(sizeBuf)
			if len(scratch) < int(size) {
				scratch = make([]byte, size)
			}
			data := scratch[:size]
			if _, err = file.ReadAt(data, int64(localPos)); err != nil {
				return nil, fmt.Errorf("cannot read record list from index file %s: %w", file.Name(), err)
			}
			records := NewRecordList(data)
			recIter := records.Iter()
			var delPosList []int
			for !recIter.Done() {
				record := recIter.Next()
				offset, err := remapper.RemapOffset(record.Block.Offset)
				if err != nil {
					// This offset does not exist in the primary. The primary
					// was corrupted and this offset is not present in the new
					// primary. Create new record list data, with the bad
					// record deleted, and add it to a work pool for later
					// deletion from the index.
					delPosList = append(delPosList, record.Pos, record.NextPos())
					log.Errorw("Index has unusable primary offset", "err", err)
				}
				binary.LittleEndian.PutUint64(records[record.Pos:], uint64(offset))
				recordCount++
			}
			if _, err = file.WriteAt(data, int64(localPos)); err != nil {
				return nil, fmt.Errorf("failed to remap primary offset in index file %s: %w", fileName, err)
			}
			if len(delPosList) != 0 {
				for i := len(delPosList) - 1; i >= 0; i -= 2 {
					delNext := delPosList[i]
					delPos := delPosList[i-1]
					data = records.PutKeys([]KeyPositionPair{}, delPos, delNext)
					records = NewRecordListRaw(data)
				}
				if rmPool == nil {
					rmPool = make(bucketPool)
				}
				rmPool[BucketIndex(pfx)] = data
			}
			indexCount++
		}

		if err = file.Close(); err != nil {
			log.Errorw("Error closing remapped index file", "err", err, "path", fileName)
		}

		// Create a ".remapped" file to indicate this file was remapped, and
		// rename the temp file to the original index file name.
		doneFile, err := os.Create(doneName)
		if err != nil {
			log.Errorw("Error creating remapped file", "err", err, "file", doneName)
		}
		if err = doneFile.Close(); err != nil {
			log.Errorw("Error closeing remapped file", "err", err, "file", doneName)
		}

		if err = os.Rename(tmpName, fileName); err != nil {
			return nil, fmt.Errorf("error renaming remapped file %s to %s: %w", tmpName, fileName, err)
		}

		fileCount++
		log.Infof("Remapped index file %s: %.1f%% done", filepath.Base(fileName), float64(1000*indexCount/indexTotal)/10)
	}

	// Update the header to indicate remapping is completed.
	header.PrimaryFileSize = mp.FileSize()
	if err = writeHeader(headerPath, header); err != nil {
		return nil, err
	}

	// Remove the completion marker files.
	for fileNum := range fileBuckets {
		doneName := indexFileName(basePath, fileNum) + ".remapped"
		if err = os.Remove(doneName); err != nil {
			log.Errorw("Error removing remapped marker", "file", doneName, "err", err)
		}
	}

	log.Infow("Remapped primary offsets", "fileCount", fileCount, "recordCount", recordCount)
	return rmPool, nil
}

func headerName(basePath string) string {
	return filepath.Clean(basePath) + ".info"
}

type fileIter struct {
	basePath string
	fileNum  uint32
}

func newFileIter(basePath string) (*fileIter, error) {
	header, err := readHeader(headerName(basePath))
	if err != nil {
		return nil, err
	}
	return &fileIter{
		basePath: basePath,
		fileNum:  header.FirstFile,
	}, nil
}

// next returns the name of the next index file. Returns io.EOF if there are no
// more index files.
func (fi *fileIter) next() (string, error) {
	_, err := os.Stat(indexFileName(fi.basePath, fi.fileNum))
	if err != nil {
		if os.IsNotExist(err) {
			err = io.EOF
		}
		return "", err
	}
	fileName := indexFileName(fi.basePath, fi.fileNum)
	fi.fileNum++

	return fileName, nil
}

func MoveFiles(indexPath, newDir string) error {
	err := os.MkdirAll(newDir, 0o755)
	if err != nil {
		return err
	}

	fileIter, err := newFileIter(indexPath)
	if err != nil {
		return err
	}
	for {
		fileName, err := fileIter.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		newPath := filepath.Join(newDir, filepath.Base(fileName))
		if err = os.Rename(fileName, newPath); err != nil {
			return err
		}
	}

	headerPath := headerName(indexPath)
	newPath := filepath.Join(newDir, filepath.Base(headerPath))
	if err = os.Rename(headerPath, newPath); err != nil {
		return err
	}

	bucketsPath := savedBucketsName(indexPath)
	_, err = os.Stat(bucketsPath)
	if !os.IsNotExist(err) {
		newPath = filepath.Join(newDir, filepath.Base(bucketsPath))
		if err = os.Rename(bucketsPath, newPath); err != nil {
			return err
		}
	}

	return nil
}
