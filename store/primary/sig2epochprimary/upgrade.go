package sig2epochprimary

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

	"github.com/rpcpool/yellowstone-faithful/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/store/types"
)

type IndexRemapper struct {
	firstFile   uint32
	maxFileSize uint32
	sizes       []int64
}

func (mp *SigToEpochPrimary) NewIndexRemapper() (*IndexRemapper, error) {
	header, err := readHeader(mp.headerPath)
	if err != nil {
		return nil, err
	}

	var sizes []int64
	for fileNum := header.FirstFile; fileNum <= mp.fileNum; fileNum++ {
		fi, err := os.Stat(primaryFileName(mp.basePath, fileNum))
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		sizes = append(sizes, fi.Size())
	}

	// If there are no primary files, or the only primary file is small enough
	// that no remapping is needed, return a nil remapper.
	if len(sizes) == 0 || (len(sizes) == 1 && sizes[0] < int64(mp.maxFileSize)) {
		return nil, nil
	}

	return &IndexRemapper{
		firstFile:   header.FirstFile,
		maxFileSize: mp.maxFileSize,
		sizes:       sizes,
	}, nil
}

func (ir *IndexRemapper) RemapOffset(pos types.Position) (types.Position, error) {
	fileNum := ir.firstFile
	newPos := int64(pos)
	for _, size := range ir.sizes {
		if newPos < size {
			return absolutePrimaryPos(types.Position(newPos), fileNum, ir.maxFileSize), nil
		}
		newPos -= size
		fileNum++
	}
	return 0, fmt.Errorf("cannot convert out-of-range primary position: %d", pos)
}

func (ir *IndexRemapper) FileSize() uint32 {
	return ir.maxFileSize
}

func upgradePrimary(ctx context.Context, filePath, headerPath string, maxFileSize uint32, freeList *freelist.FreeList) (uint32, error) {
	// If header already exists, or old primary does not exist, then no upgrade.
	_, err := os.Stat(headerPath)
	if !os.IsNotExist(err) {
		// Header already exists, do nothing.
		return 0, nil
	}
	if _, err = os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// No primary to upgrade.
			return 0, nil
		}
		return 0, err
	}

	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	log.Infow("Upgrading primary storage and splitting into separate files", "newVersion", PrimaryVersion, "fileSize", maxFileSize)
	if freeList != nil {
		// Instead of remapping all the primary offsets in the freelist, call
		// the garbage collector function to process the freelist and make the
		// primary records deleted. This is safer because it can be re-applied
		// if there is a failure during this phase.
		err := applyFreeList(ctx, freeList, filePath)
		if err != nil {
			return 0, fmt.Errorf("could not apply freelist to primary: %w", err)
		}
	}

	fileNum, err := chunkOldPrimary(ctx, filePath, int64(maxFileSize))
	if err != nil {
		return 0, fmt.Errorf("error chunking primary: %w", err)
	}

	if err = writeHeader(headerPath, newHeader(maxFileSize)); err != nil {
		return 0, fmt.Errorf("error writing primary info file: %w", err)
	}

	if err = os.Remove(filePath); err != nil {
		return 0, fmt.Errorf("cannot remove old primary: %w", err)
	}

	log.Infow("Replaced old primary with multiple files", "replaced", filePath, "files", fileNum+1)
	log.Infof("Upgraded primary from version 0 to %d", PrimaryVersion)
	return fileNum, nil
}

func chunkOldPrimary(ctx context.Context, name string, fileSizeLimit int64) (uint32, error) {
	file, err := os.Open(name)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if fi.Size() == 0 {
		return 0, nil
	}

	total := fi.Size()
	var fileNum uint32
	outName := primaryFileName(name, fileNum)
	outFile, err := createFileAppend(outName)
	if err != nil {
		return 0, err
	}
	log.Infow("Upgrade created primary file", "file", filepath.Base(outName))
	writer := bufio.NewWriterSize(outFile, blockBufferSize)

	sizeBuf := make([]byte, sizePrefixSize)
	var written int64
	var count int
	var pos int64
	scratch := make([]byte, 1024)

	for {
		_, err = file.ReadAt(sizeBuf, pos)
		if err != nil {
			if err != io.EOF {
				log.Errorw("Error reading primary", "err", err)
			}
			break
		}
		size := binary.LittleEndian.Uint32(sizeBuf)
		if _, err = writer.Write(sizeBuf); err != nil {
			outFile.Close()
			return 0, err
		}
		pos += sizePrefixSize

		del := false
		if size&deletedBit != 0 {
			size ^= deletedBit
			del = true
		}

		if int(size) > len(scratch) {
			scratch = make([]byte, size)
		}
		data := scratch[:size]

		if !del {
			if _, err = file.ReadAt(data, pos); err != nil {
				log.Errorw("Error reading primary", "err", err)
				break
			}
		}
		_, err := writer.Write(data)
		if err != nil {
			outFile.Close()
			return 0, err
		}
		pos += int64(size)

		written += sizePrefixSize + int64(size)
		if written >= fileSizeLimit {
			if err = writer.Flush(); err != nil {
				return 0, err
			}
			outFile.Close()
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			fileNum++
			outName = primaryFileName(name, fileNum)
			outFile, err = createFileAppend(outName)
			if err != nil {
				return 0, err
			}
			log.Infof("Upgrade created primary file %s: %.1f%% done", filepath.Base(outName), float64(1000*pos/total)/10)
			writer.Reset(outFile)
			written = 0
		}
		count++
	}
	if written != 0 {
		if err = writer.Flush(); err != nil {
			return 0, err
		}
	}
	outFile.Close()
	return fileNum, nil
}

func createFileAppend(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0o644)
}

// applyFreeList reads the freelist and marks the locations in the old primary file
// as dead by setting the deleted bit in the record size field.
func applyFreeList(ctx context.Context, freeList *freelist.FreeList, filePath string) error {
	flPath, err := freeList.ToGC()
	if err != nil {
		return fmt.Errorf("cannot get freelist gc file: %w", err)
	}

	fi, err := os.Stat(flPath)
	if err != nil {
		return fmt.Errorf("cannot stat freelist gc file: %w", err)
	}
	flSize := fi.Size()

	// If the freelist size is non-zero, then process its records.
	var count int
	if flSize != 0 {
		log.Infof("Applying freelist to primary storage")

		flFile, err := os.OpenFile(flPath, os.O_RDONLY, 0o644)
		if err != nil {
			return fmt.Errorf("error opening freelist gc file: %w", err)
		}
		defer flFile.Close()

		primaryFile, err := os.OpenFile(filePath, os.O_RDWR, 0o644)
		if err != nil {
			return fmt.Errorf("cannot open primary file %s: %w", filePath, err)
		}
		defer primaryFile.Close()

		fi, err = primaryFile.Stat()
		if err != nil {
			return fmt.Errorf("cannot stat primary file %s: %w", primaryFile.Name(), err)
		}
		primarySize := fi.Size()

		total := int(flSize / (types.OffBytesLen + types.SizeBytesLen))
		flIter := freelist.NewIterator(bufio.NewReader(flFile))
		sizeBuf := make([]byte, sizePrefixSize)
		percentIncr := 1
		nextPercent := percentIncr

		for {
			free, err := flIter.Next()
			if err != nil {
				// Done reading freelist; log if error.
				if err != io.EOF {
					log.Errorw("Error reading freelist", "err", err)
				}
				break
			}

			offset := int64(free.Offset)

			if offset > primarySize {
				log.Errorw("freelist record has out-of-range primary offset", "offset", offset, "fileSize", primarySize)
				continue // skip bad freelist entry
			}

			if _, err = primaryFile.ReadAt(sizeBuf, offset); err != nil {
				return err
			}
			recSize := binary.LittleEndian.Uint32(sizeBuf)
			if recSize&deletedBit != 0 {
				// Already deleted.
				continue
			}
			if recSize != uint32(free.Size) {
				log.Errorw("Record size in primary does not match size in freelist", "primaryRecordSize", recSize, "freelistRecordSize", free.Size, "file", flFile.Name(), "offset", offset)
			}

			// Mark the record as deleted by setting the highest bit in the
			// size. This assumes that the record size is < 2^31.
			binary.LittleEndian.PutUint32(sizeBuf, recSize|deletedBit)
			_, err = primaryFile.WriteAt(sizeBuf, int64(offset))
			if err != nil {
				return fmt.Errorf("cannot write to primary file %s: %w", flFile.Name(), err)
			}

			count++

			// Log at every percent increment.
			percent := 100 * count / total
			if percent >= nextPercent {
				log.Infof("Processed %d of %d freelist records: %d%% done", count, total, percent)
				nextPercent += percentIncr
			}
		}
		log.Infow("Marked primary records from freelist as deleted", "count", count)
		flFile.Close()
	}

	if err = os.Remove(flPath); err != nil {
		return fmt.Errorf("error removing freelist: %w", err)
	}

	return nil
}
