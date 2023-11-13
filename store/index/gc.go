package index

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/rpcpool/yellowstone-faithful/store/types"
)

var log = logging.Logger("storethehash/index")

// maxFreeSkip is the maximum number of gc cycled to skip looking for free
// index files to truncate.
const maxFreeSkip = 8

// garbageCollector is a goroutine that runs periodically to search for and
// remove stale index files. It runs every gcInterval, if there have been any
// index updates.
func (index *Index) garbageCollector(interval, timeLimit time.Duration) {
	defer close(index.gcDone)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var gcDone chan struct{}
	var freeSkip, freeSkipIncr int

	t := time.NewTimer(interval)

	for {
		select {
		case <-index.gcStop:
			cancel()
			if gcDone != nil {
				<-gcDone
			}
			return
		case <-t.C:
			gcDone = make(chan struct{})
			go func(ctx context.Context) {
				defer close(gcDone)
				// Log cache stats.
				hits, misses, cacheLen, cacheCap := index.fileCache.Stats()
				log.Infow("File cache stats", "hits", hits, "misses", misses, "len", cacheLen, "cap", cacheCap)

				if timeLimit != 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, timeLimit)
					defer cancel()
				}

				log.Infow("GC started")
				reclaimed, emptied, err := index.gc(ctx, freeSkip == 0)
				switch err {
				case nil:
				case context.DeadlineExceeded:
					log.Infow("GC stopped at time limit", "limit", timeLimit)
				case context.Canceled:
					log.Info("GC canceled")
					return
				default:
					log.Errorw("GC failed", "err", err)
					return
				}

				log.Infof("GC finished, reclaimed %d bytes", reclaimed)
				if !index.gcResume {
					// GC had time to reap records from all files, so running
					// truncateFreeFiles will not be helpful over just reaping
					// records on next GC run.
					emptied = 0
				}

				if freeSkip == 0 {
					if emptied == 0 {
						if freeSkipIncr < maxFreeSkip {
							// No files truncated, skip scan for more gc cycles.
							freeSkipIncr++
						}
					} else if freeSkipIncr > 0 {
						// Files were truncated, skip scan for fewer gc cycles.
						freeSkipIncr--
					}
					freeSkip = freeSkipIncr
				} else {
					// One less cycle until truncateFreeFiles tried again.
					freeSkip--
				}
			}(ctx)
		case <-gcDone:
			gcDone = nil
			t.Reset(interval)
		}
	}
}

// gc searches for and removes stale index files. Returns the number of unused
// index files that were removed and the number of freeFiles that were found.
func (index *Index) gc(ctx context.Context, scanFree bool) (int64, int, error) {
	var emptied int
	var reclaimed int64
	var err error

	if scanFree {
		reclaimed, emptied, err = index.truncateFreeFiles(ctx)
		if err != nil {
			if err == context.DeadlineExceeded {
				return reclaimed, emptied, err
			}
			return 0, 0, err
		}
		log.Debugf("Emptied %d unused index files", emptied)
	}

	header, err := readHeader(index.headerPath)
	if err != nil {
		return 0, 0, err
	}

	index.flushLock.Lock()
	lastFileNum := index.fileNum
	index.flushLock.Unlock()

	if header.FirstFile == lastFileNum {
		return reclaimed, emptied, nil
	}

	var firstFileNum uint32
	if index.gcResume {
		firstFileNum = index.gcResumeAt
		index.gcResume = false
		log.Debugw("Resuming GC", "file", filepath.Base(indexFileName(index.basePath, firstFileNum)))
	} else {
		firstFileNum = header.FirstFile
	}

	var seenFirst bool
	for fileNum := firstFileNum; fileNum != lastFileNum; {
		indexPath := indexFileName(index.basePath, fileNum)

		stale, err := index.reapIndexRecords(ctx, fileNum, indexPath)
		if err != nil {
			if err == context.DeadlineExceeded {
				index.gcResumeAt = fileNum
				index.gcResume = true
				return reclaimed, emptied, err
			}
			return 0, 0, err
		}
		if stale {
			index.fileCache.Remove(indexPath)

			// If this is first index file, then update header and remove file.
			if header.FirstFile == fileNum {
				header.FirstFile++
				err = writeHeader(index.headerPath, header)
				if err != nil {
					return 0, 0, err
				}
				err = os.Remove(indexPath)
				if err != nil {
					return 0, 0, err
				}
				seenFirst = true
			}
		}

		fileNum++
		if fileNum == lastFileNum {
			if seenFirst {
				break
			}
			fileNum = header.FirstFile
		}
		if fileNum == firstFileNum {
			// Back to where gc started, all done.
			break
		}
	}
	return reclaimed, emptied, nil
}

func (index *Index) truncateFreeFiles(ctx context.Context) (int64, int, error) {
	header, err := readHeader(index.headerPath)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read index header: %w", err)
	}
	index.flushLock.Lock()
	lastFileNum := index.fileNum
	index.flushLock.Unlock()

	fileCount := lastFileNum - header.FirstFile
	if fileCount == 0 {
		return 0, 0, nil
	}

	busySet := make(map[uint32]struct{}, fileCount)
	maxFileSize := index.maxFileSize
	end := 1 << index.sizeBits
	tmpBuckets := make([]types.Position, 4096)
	for i := 0; i < end; {
		index.bucketLk.RLock()
		i += copy(tmpBuckets, index.buckets[i:])
		index.bucketLk.RUnlock()
		for _, offset := range tmpBuckets {
			ok, fileNum := bucketPosToFileNum(offset, maxFileSize)
			if ok {
				busySet[fileNum] = struct{}{}
			}
		}
	}

	var emptied int
	var reclaimed int64
	basePath := index.basePath

	for fileNum := header.FirstFile; fileNum != lastFileNum; fileNum++ {
		if _, busy := busySet[fileNum]; busy {
			continue
		}

		if ctx.Err() != nil {
			return reclaimed, emptied, ctx.Err()
		}

		indexPath := indexFileName(basePath, fileNum)

		index.fileCache.Remove(indexPath)

		fi, err := os.Stat(indexPath)
		if err != nil {
			log.Errorw("Cannot stat index file", "err", err, "file", indexPath)
			continue
		}
		reclaimed += fi.Size()

		// If this is first index file, then update header and remove file.
		if header.FirstFile == fileNum {
			header.FirstFile++
			if err = writeHeader(index.headerPath, header); err != nil {
				return 0, 0, err
			}
			if err = os.Remove(indexPath); err != nil {
				return 0, 0, err
			}
			emptied++
			log.Debugw("Removed unused index file", "file", indexPath)
			continue
		}

		if fi.Size() == 0 {
			continue
		}

		err = os.Truncate(indexPath, 0)
		if err != nil {
			log.Errorw("Error truncating index file", "err", err, "file", indexPath)
			continue
		}
		emptied++
		log.Debugw("Emptied unused index file", "file", indexPath)
	}

	return reclaimed, emptied, nil
}

// reapIndexRecords scans a single index file, logically deleting records that
// are not referenced by a bucket, merging spans of deleted records, and
// truncating deleted records from the end of the file.
func (index *Index) reapIndexRecords(ctx context.Context, fileNum uint32, indexPath string) (bool, error) {
	fi, err := os.Stat(indexPath)
	if err != nil {
		return false, fmt.Errorf("cannot stat index file: %w", err)
	}
	if fi.Size() == 0 {
		// File is empty, so OK to delete if it is first file.
		return true, nil
	}

	file, err := os.OpenFile(indexPath, os.O_RDWR, 0o644)
	if err != nil {
		return false, err
	}
	defer file.Close()

	var freedCount, mergedCount int
	var freeAtSize uint32
	var busyAt, freeAt int64
	freeAt = -1
	busyAt = -1

	sizeBuf := make([]byte, sizePrefixSize)
	scratch := make([]byte, 256)
	var pos int64
	for {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if _, err = file.ReadAt(sizeBuf, pos); err != nil {
			if errors.Is(err, io.EOF) {
				// Finished reading entire index.
				break
			}
			return false, err
		}

		size := binary.LittleEndian.Uint32(sizeBuf)
		if size&deletedBit != 0 {
			// Record is already deleted.
			size ^= deletedBit
			if freeAt > busyAt {
				// Previous record free, so merge this record into the last.
				freeAtSize += sizePrefixSize + size
				if freeAtSize >= deletedBit {
					log.Warnf("Records are too large to merge %d >= %d", freeAtSize, deletedBit)
					freeAt = pos
					freeAtSize = size
				} else {
					binary.LittleEndian.PutUint32(sizeBuf, freeAtSize|deletedBit)
					_, err = file.WriteAt(sizeBuf, freeAt)
					if err != nil {
						return false, fmt.Errorf("cannot write to index file %s: %w", file.Name(), err)
					}
					mergedCount++
				}
			} else {
				// Previous record was not free, so mark new free position.
				freeAt = pos
				freeAtSize = size
			}
			pos += sizePrefixSize + int64(size)
			continue
		}

		if int(size) > len(scratch) {
			scratch = make([]byte, size)
		}
		data := scratch[:size]
		if _, err = file.ReadAt(data, pos+sizePrefixSize); err != nil {
			if errors.Is(err, io.EOF) {
				// The data has not been written yet, or the file is corrupt.
				// Take the data we are able to use and move on.
				break
			}
			return false, fmt.Errorf("error reading data from index: %w", err)
		}

		bucketPrefix := BucketIndex(binary.LittleEndian.Uint32(data))
		inUse, err := index.busy(bucketPrefix, pos+sizePrefixSize, fileNum)
		if err != nil {
			return false, err
		}
		if inUse {
			// Record is in use.
			busyAt = pos
		} else {
			// Record is free.
			if freeAt > busyAt {
				// Merge this free record into the last
				freeAtSize += sizePrefixSize + size
				if freeAtSize >= deletedBit {
					log.Warn("Records are too large to merge")
					freeAt = pos
					freeAtSize = size
				} else {
					mergedCount++
				}
			} else {
				freeAt = pos
				freeAtSize = size
			}

			// Mark the record as deleted by setting the highest bit in the
			// size. This assumes that the size of an individual index record
			// will always be less than 2^30.
			binary.LittleEndian.PutUint32(sizeBuf, freeAtSize|deletedBit)
			if _, err = file.WriteAt(sizeBuf, freeAt); err != nil {
				return false, fmt.Errorf("cannot write to index file %s: %w", file.Name(), err)
			}
			freedCount++
		}
		pos += sizePrefixSize + int64(size)
	}

	fileName := filepath.Base(file.Name())
	log.Debugw("Marked index records as free", "freed", freedCount, "merged", mergedCount, "file", fileName)

	// If there is a span of free records at end of file, truncate file.
	if freeAt > busyAt {
		// End of primary is free.
		if err = file.Truncate(freeAt); err != nil {
			return false, fmt.Errorf("failed to truncate index file: %w", err)
		}
		log.Debugw("Removed free records from end of index file", "file", fileName, "at", freeAt, "bytes", freeAtSize)
		if freeAt == 0 {
			return true, nil
		}
	}

	return false, nil
}

func (index *Index) busy(bucketPrefix BucketIndex, localPos int64, fileNum uint32) (bool, error) {
	index.bucketLk.RLock()
	bucketPos, err := index.buckets.Get(bucketPrefix)
	index.bucketLk.RUnlock()
	if err != nil {
		return false, err
	}
	localPosInBucket, fileNumInBucket := localizeBucketPos(bucketPos, index.maxFileSize)
	if fileNum == fileNumInBucket && localPos == int64(localPosInBucket) {
		return true, nil
	}
	return false, nil
}
