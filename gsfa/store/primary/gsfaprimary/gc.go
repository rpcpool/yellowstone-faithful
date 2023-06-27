package gsfaprimary

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
)

var log = logging.Logger("storethehash/mhprimary")

// defaultLowUsePercent is the default percentage of the file that must be
// composed of free records to be considered low-use. Records will be relocated
// from low-use files over time.
const defaultLowUsePercent = 85

type primaryGC struct {
	freeList    *freelist.FreeList
	primary     *GsfaPrimary
	done        chan struct{}
	stop        chan struct{}
	updateIndex UpdateIndexFunc
	visited     map[uint32]struct{}
	reclaimed   int64
}

type UpdateIndexFunc func([]byte, types.Block) error

func newGC(primary *GsfaPrimary, freeList *freelist.FreeList, interval, timeLimit time.Duration, updateIndex UpdateIndexFunc) *primaryGC {
	gc := &primaryGC{
		freeList:    freeList,
		primary:     primary,
		done:        make(chan struct{}),
		stop:        make(chan struct{}),
		updateIndex: updateIndex,
		visited:     make(map[uint32]struct{}),
	}

	go gc.run(interval, timeLimit)

	return gc
}

func (gc *primaryGC) close() {
	close(gc.stop)
	<-gc.done
}

// run is a goroutine that runs periodically to search for and remove primary
// files that contain only deleted records. It runs every interval and operates
// on files that have not been visited before or that are affected by deleted
// records from the freelist.
func (gc *primaryGC) run(interval, timeLimit time.Duration) {
	defer close(gc.done)

	// Start after half the interval to offset from index GC.
	t := time.NewTimer(interval / 2)

	var gcDone chan struct{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-gc.stop:
			cancel()
			if gcDone != nil {
				<-gcDone
			}
			return
		case <-t.C:
			gcDone = make(chan struct{})
			go func(ctx context.Context) {
				defer close(gcDone)

				log.Infow("GC started")

				reclaimed, err := gc.gc(ctx, defaultLowUsePercent, timeLimit)
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
				log.Infof("GC reclaimed %d bytes", reclaimed)
			}(ctx)
		case <-gcDone:
			gcDone = nil
			t.Reset(interval)
		}
	}
}

// gc searches for and removes stale primary files. Returns the number of bytes
// of storage reclaimed.
func (gc *primaryGC) gc(ctx context.Context, lowUsePercent int64, timeLimit time.Duration) (int64, error) {
	gc.reclaimed = 0
	affectedSet, err := processFreeList(ctx, gc.freeList, gc.primary.basePath, gc.primary.maxFileSize)
	if err != nil {
		if err == context.DeadlineExceeded {
			return gc.reclaimed, err
		}
		return 0, fmt.Errorf("cannot process freelist: %w", err)
	}

	// Remove all files in the affected set from the visited set.
	for fileNum := range affectedSet {
		delete(gc.visited, fileNum)
	}

	header, err := readHeader(gc.primary.headerPath)
	if err != nil {
		return 0, fmt.Errorf("cannot read primary header: %w", err)
	}

	// Start the timer only after processing the free list to allow some time
	// to reclaim storage. Otherwise, GC may only keep trying to keep up with
	// freelist.
	if timeLimit != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeLimit)
		defer cancel()
	}

	// GC each unvisited file in order.
	for fileNum := header.FirstFile; fileNum != gc.primary.fileNum; fileNum++ {
		if _, ok := gc.visited[fileNum]; ok {
			continue
		}

		filePath := primaryFileName(gc.primary.basePath, fileNum)

		dead, err := gc.reapRecords(fileNum, lowUsePercent)
		if err != nil {
			return gc.reclaimed, err
		}

		if dead && fileNum == header.FirstFile {
			header.FirstFile++
			if err = writeHeader(gc.primary.headerPath, header); err != nil {
				return 0, fmt.Errorf("cannot write header: %w", err)
			}
			if err = os.Remove(filePath); err != nil {
				return 0, fmt.Errorf("cannot remove primary file %s: %w", filePath, err)
			}
			log.Debugw("Removed empty primary file", "file", filepath.Base(filePath))
		}

		gc.visited[fileNum] = struct{}{}

		if ctx.Err() != nil {
			if err == context.DeadlineExceeded {
				return gc.reclaimed, err
			}
			return 0, ctx.Err()
		}
	}

	return gc.reclaimed, nil
}

// reapRecords removes empty records from the end of the file. If the file is
// empty, then returns true to indicate the file can be deleted.
func (gc *primaryGC) reapRecords(fileNum uint32, lowUsePercent int64) (bool, error) {
	file, err := os.OpenFile(primaryFileName(gc.primary.basePath, fileNum), os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("cannot open primary file: %w", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("cannot stat primary file: %w", err)
	}
	fileName := filepath.Base(file.Name())
	if fi.Size() == 0 {
		// File was already truncated to 0 size, but was not yet removed.
		log.Debugw("Primary file is already empty", "file", fileName)
		return true, nil
	}

	var mergedCount int
	var busyAt, freeAt, prevBusyAt int64
	var busySize, prevBusySize, totalBusy, totalFree int64
	var freeAtSize uint32
	freeAt = -1
	busyAt = -1
	prevBusyAt = -1

	// See if any entries can be merged.
	sizeBuf := make([]byte, sizePrefixSize)
	var pos int64
	for {
		if _, err = file.ReadAt(sizeBuf, pos); err != nil {
			if err == io.EOF {
				// Finished reading entire primary.
				break
			}
			return false, err
		}
		size := binary.LittleEndian.Uint32(sizeBuf)

		if size&deletedBit != 0 {
			size ^= deletedBit
			// If previous record is free.
			if freeAt > busyAt {
				// Merge this free record into the last
				freeAtSize += sizePrefixSize + size
				if freeAtSize >= deletedBit {
					log.Warn("Records are too large to merge")
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
			totalFree += int64(size)
		} else {
			// Record is in use.
			prevBusyAt = busyAt
			prevBusySize = busySize
			busyAt = pos
			busySize = int64(size)
			totalBusy += busySize
		}

		pos += sizePrefixSize + int64(size)
	}

	log.Debugw("Merged free primary records", "merged", mergedCount, "file", fileName)

	// If updateIndex is not set, then do not truncate files because index
	// remapping may not have completed yet.
	//
	// No ability to move primary records without being able to update index.
	if gc.updateIndex == nil {
		return false, nil
	}

	// If there is a span of free records at end of file, truncate file.
	if freeAt > busyAt {
		// End of primary is free.
		if err = file.Truncate(freeAt); err != nil {
			return false, err
		}
		gc.reclaimed += int64(freeAtSize)
		log.Debugw("Removed free records from end of primary file", "file", fileName, "at", freeAt, "bytes", freeAtSize)

		if freeAt == 0 {
			// Entire primary is free.
			return true, nil
		}
	}

	// If only known busy location was freed, but file is not empty, then start
	// over next gc cycle.
	if busyAt == -1 {
		return false, nil
	}

	// If a sufficient percent of the records in the file are free, rewrite the
	// last 2 records, that are still in use, into a later primary. This will
	// allow low-use primary files to evaporate over time.
	if 100*totalFree >= lowUsePercent*(totalFree+totalBusy) {
		scratch := make([]byte, 1024)

		for busyAt >= 0 {
			// Read the record data.
			if _, err = file.ReadAt(sizeBuf, busyAt); err != nil {
				return false, fmt.Errorf("cannot read record size: %w", err)
			}
			size := binary.LittleEndian.Uint32(sizeBuf)
			if int(size) > len(scratch) {
				scratch = make([]byte, size)
			}
			data := scratch[:size]
			if _, err = file.ReadAt(data, busyAt+sizePrefixSize); err != nil {
				return false, fmt.Errorf("cannot read record data: %w", err)
			}
			// Extract key and value from record data.
			key, val, err := readNode(data)
			if err != nil {
				return false, fmt.Errorf("cannot extract key and value from record: %w", err)
			}
			// Get the index key for the record key.
			indexKey, err := gc.primary.IndexKey(key)
			if err != nil {
				return false, fmt.Errorf("cannot get index key for record key: %w", err)
			}
			// Store the key and value in the primary.
			fileOffset, err := gc.primary.Put(key, val)
			if err != nil {
				return false, fmt.Errorf("cannot put new primary record: %w", err)
			}
			// Update the index with the new primary location.
			if err = gc.updateIndex(indexKey, fileOffset); err != nil {
				log.Errorw("Cannot update index with new record location", "err", err)
				// Failed to index the moved record, most likely because the
				// key was not found in the index. The moved record is
				// unreachable so it must be removed.
				if err = gc.freeList.Put(fileOffset); err != nil {
					log.Errorw("Cannot put failed index record location into freelist", "err", err)
				}
			} else {
				log.Debugw("Moved record from end of low-use file", "from", fileName, "free", totalFree, "busy", totalBusy)
			}
			// Do not truncate file here, because moved record may not be
			// written yet. Instead put moved record onto freelist and let next
			// GC cycle process freelist and delete this record. This also
			// keeps low-use files getting processed each GC cycle.

			// Add outdated data in primary storage to freelist
			offset := absolutePrimaryPos(types.Position(busyAt), fileNum, gc.primary.maxFileSize)
			blk := types.Block{Size: types.Size(busySize), Offset: types.Position(offset)}
			if err = gc.freeList.Put(blk); err != nil {
				return false, fmt.Errorf("cannot put old record location into freelist: %w", err)
			}

			busyAt = prevBusyAt
			busySize = prevBusySize
			prevBusyAt = -1
		}
	}

	return false, nil
}

// processFreeList reads the freelist and marks the locations in primary files
// as dead by setting the deleted bit in the record size field.
func processFreeList(ctx context.Context, freeList *freelist.FreeList, basePath string, maxFileSize uint32) (map[uint32]struct{}, error) {
	const freeBatchSize = 1024 * 512

	flPath, err := freeList.ToGC()
	if err != nil {
		return nil, fmt.Errorf("cannot get freelist gc file: %w", err)
	}

	fi, err := os.Stat(flPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat freelist gc file: %w", err)
	}

	var affectedSet map[uint32]struct{}
	var freeBatch []*types.Block

	// If the freelist size is non-zero, then process its records.
	if fi.Size() != 0 {
		log.Debugf("Applying freelist to primary storage")
		affectedSet = make(map[uint32]struct{})
		startTime := time.Now()

		flFile, err := os.OpenFile(flPath, os.O_RDONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error opening freelist gc file: %w", err)
		}
		defer flFile.Close()

		var count int
		flIter := freelist.NewIterator(bufio.NewReader(flFile))

		batchSize := fi.Size() / (types.OffBytesLen + types.SizeBytesLen)
		if batchSize > freeBatchSize {
			batchSize = freeBatchSize
		}
		freeBatch = make([]*types.Block, 0, batchSize)

		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			free, err := flIter.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("error reading freelist: %w", err)
			}
			freeBatch = append(freeBatch, free)
			if len(freeBatch) == cap(freeBatch) {
				// Mark dead location with tombstone bit in the record's size data.
				count += deleteRecords(freeBatch, maxFileSize, basePath, affectedSet)
				freeBatch = freeBatch[:0]
			}
		}
		flFile.Close()

		if len(freeBatch) != 0 {
			count += deleteRecords(freeBatch, maxFileSize, basePath, affectedSet)
		}

		log.Debugw("Marked primary records from freelist as deleted", "count", count, "elapsed", time.Since(startTime).String())
	}

	if err = os.Remove(flPath); err != nil {
		return nil, fmt.Errorf("error removing freelist: %w", err)
	}

	return affectedSet, nil
}

func deleteRecords(freeBatch []*types.Block, maxFileSize uint32, basePath string, affected map[uint32]struct{}) int {
	sort.Slice(freeBatch, func(i, j int) bool { return freeBatch[i].Offset < freeBatch[j].Offset })

	var file *os.File
	var primarySize int64
	var curFileNum uint32
	var count, prevCount int

	for _, freeRec := range freeBatch {
		var err error
		localPos, fileNum := localizePrimaryPos(freeRec.Offset, maxFileSize)
		if file == nil || fileNum != curFileNum {
			if file != nil {
				// Close the previous file.
				file.Close()
				file = nil
				if count > prevCount {
					affected[curFileNum] = struct{}{}
					prevCount = count
				}
			}

			file, err = os.OpenFile(primaryFileName(basePath, fileNum), os.O_RDWR, 0o644)
			if err != nil {
				log.Errorw("Cannot open primary file", "file", file.Name(), "err", err)
				continue
			}
			fi, err := file.Stat()
			if err != nil {
				log.Errorw("Cannot stat primary file", "file", file.Name(), "err", err)
				continue
			}
			primarySize = fi.Size()
			curFileNum = fileNum
		}

		if int64(localPos) > primarySize {
			log.Errorw("freelist record has out-of-range primary offset", "offset", localPos, "fileSize", primarySize)
			continue
		}

		sizeBuf := make([]byte, sizePrefixSize)
		if _, err = file.ReadAt(sizeBuf, int64(localPos)); err != nil {
			log.Errorw("Cannot read primary record", "err", err)
			continue
		}

		recSize := binary.LittleEndian.Uint32(sizeBuf)
		if recSize&deletedBit != 0 {
			// Already deleted
			continue
		}

		if types.Size(recSize) != freeRec.Size {
			log.Errorw("Record size in primary does not match size in freelist",
				"recordSize", recSize, "file", file.Name(), "freeSize", freeRec.Size, "pos", localPos)
			continue
		}

		// Mark the record as deleted by setting the highest bit in the size. This
		// assumes that the record size is < 2^31.
		binary.LittleEndian.PutUint32(sizeBuf, recSize|deletedBit)
		_, err = file.WriteAt(sizeBuf, int64(localPos))
		if err != nil {
			log.Errorw("Cannot write to primary file", "file", file.Name(), "err", err)
			continue
		}
		count++
	}

	if file != nil {
		// Close the previous file.
		file.Close()
		if count > prevCount {
			affected[curFileNum] = struct{}{}
		}
	}

	return count
}
