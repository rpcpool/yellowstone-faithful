// write.go
package bucketteer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/continuity"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"k8s.io/klog/v2"
)

type Writer struct {
	path           string
	destination    *os.File
	writer         *bufio.Writer
	prefixToHashes *prefixToHashes // prefix -> hashes
}

type prefixToHashes [math.MaxUint16 + 1][]uint64 // prefix -> hashes

func newPrefixToHashes() *prefixToHashes {
	var out prefixToHashes
	// Optimization: Do not pre-allocate huge slices for every bucket.
	// 65536 buckets * 16,000 capacity * 8 bytes = ~8GB RAM overhead.
	// We initialize with nil and allocate only when Put is called.
	return &out
}

const (
	_MiB         = 1024 * 1024
	writeBufSize = _MiB * 10
)

func NewWriter(path string) (*Writer, error) {
	if ok, err := isDir(path); err != nil {
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("path is a directory")
	}
	if ok, err := fileIsBlank(path); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("file already exists and is not empty: %s", path)
	}

	return &Writer{
		path:           path,
		prefixToHashes: newPrefixToHashes(),
	}, nil
}

// Put adds the given signature to the Bucketteer.
// Cannot be called concurrently.
func (b *Writer) Put(sig [64]byte) {
	if b == nil {
		panic("Put called on nil Writer")
	}
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	pU16 := prefixToUint16(prefix)

	// Lazy allocation for specific bucket
	if b.prefixToHashes[pU16] == nil {
		b.prefixToHashes[pU16] = make([]uint64, 0, 1024)
	}
	b.prefixToHashes[pU16] = append(b.prefixToHashes[pU16], Hash(sig))
}

// Has returns true if the Bucketteer has seen the given signature in memory.
func (b *Writer) Has(sig [64]byte) bool {
	if b == nil {
		return false
	}
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	hash := Hash(sig)
	bucket := b.prefixToHashes[prefixToUint16(prefix)]
	for _, h := range bucket {
		if h == hash {
			return true
		}
	}
	return false
}

func (b *Writer) close() error {
	if b == nil {
		return nil
	}
	var errs []error
	if b.writer != nil {
		if err := b.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("failed to flush writer: %w", err))
		}
	}
	if b.destination != nil {
		if err := b.destination.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("failed to sync file: %w", err))
		}
		if err := b.destination.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close destination: %w", err))
		}
		b.destination = nil
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// SealAndClose writes the Bucketteer's state to disk and closes resources.
func (b *Writer) SealAndClose(meta indexmeta.Meta) (int64, error) {
	if b == nil {
		return 0, errors.New("SealAndClose called on nil Writer")
	}

	klog.V(1).Infof("Sealing bucketteer to %s", b.path)

	file, err := os.Create(b.path)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	b.destination = file
	b.writer = bufio.NewWriterSize(file, writeBufSize)

	// In case of any error before the continuity chain finishes, ensure we close.
	success := false
	defer func() {
		if !success {
			_ = b.close()
		}
	}()

	newHeader, size, err := seal(b.writer, b.prefixToHashes, meta)
	if err != nil {
		return 0, fmt.Errorf("failed to seal buckets: %w", err)
	}

	if err := b.writer.Flush(); err != nil {
		return 0, fmt.Errorf("failed to flush final buckets: %w", err)
	}
	if err := b.destination.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync file: %w", err)
	}

	err = continuity.New().
		Thenf("overwriteHeader", func() error {
			klog.V(2).Infof("Overwriting header at offset 0 (header size: %d)", len(newHeader))
			return overwriteFileContentAt(b.destination, 0, newHeader)
		}).
		Thenf("close", func() error {
			success = true // Continuity handles closing now
			return b.close()
		}).
		Err()

	return size, err
}

func createHeader(
	magic [8]byte,
	version uint64,
	headerSizeIn uint32,
	meta indexmeta.Meta,
	prefixToOffset bucketToOffset,
) ([]byte, error) {
	tmpHeaderBuf := new(bytes.Buffer)
	headerWriter := bin.NewBorshEncoder(tmpHeaderBuf)

	// 1. Header Size (4 bytes)
	if err := headerWriter.WriteUint32(headerSizeIn, binary.LittleEndian); err != nil {
		return nil, err
	}
	// 2. Magic (8 bytes)
	if n, err := headerWriter.Write(magic[:]); err != nil {
		return nil, err
	} else if n != 8 {
		return nil, fmt.Errorf("invalid number of bytes written for magic: %d", n)
	}
	// 3. Version (8 bytes)
	if err := headerWriter.WriteUint64(version, binary.LittleEndian); err != nil {
		return nil, err
	}
	// 4. Metadata
	metaBuf, err := meta.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if _, err := headerWriter.Write(metaBuf); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}
	// 5. Num Buckets (8 bytes)
	if err := headerWriter.WriteUint64(uint64(len(prefixToOffset)), binary.LittleEndian); err != nil {
		return nil, err
	}

	// 6. Prefix-Offset Index (65536 * 10 bytes)
	for i := range prefixToOffset {
		prefix := uint16ToPrefix(uint16(i))
		if _, err := headerWriter.Write(prefix[:]); err != nil {
			return nil, err
		}
		if err := headerWriter.WriteUint64(prefixToOffset[i], binary.LittleEndian); err != nil {
			return nil, err
		}
	}
	return tmpHeaderBuf.Bytes(), nil
}

func overwriteFileContentAt(file *os.File, offset int64, data []byte) error {
	wrote, err := file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if wrote != len(data) {
		return fmt.Errorf("wrote %d bytes, expected to write %d bytes", wrote, len(data))
	}
	return nil
}

func seal(
	out *bufio.Writer,
	prefixToHashes *prefixToHashes,
	meta indexmeta.Meta,
) ([]byte, int64, error) {
	prefixToOffset := bucketToOffset{}
	totalWritten := int64(0)

	// Write draft header to reserve space.
	header, err := createHeader(_Magic, Version, 0, meta, prefixToOffset)
	if err != nil {
		return nil, 0, err
	}
	headerSize, err := out.Write(header)
	if err != nil {
		return nil, 0, err
	}
	totalWritten += int64(headerSize)

	klog.V(2).Infof("Header draft written: %d bytes. Starting bucket serialization.", headerSize)

	previousOffset := uint64(0)
	for prefix := range prefixToHashes {
		entries := getCleanSet(prefixToHashes[prefix])
		if len(entries) != len(prefixToHashes[prefix]) {
			// TODO: is this an error? but if this is an existence check, duplicates dont matter?
			// BUT if there are collisions in this index, it means that there are collisions in the OTHER
			// signature indexes as well (sig_to_cid), which is FATAL (index creation FAILS).
			klog.Warningf("duplicate hashes for prefix %v", prefix)
		}
		sortWithCompare(entries, func(i, j int) int {
			if entries[i] < entries[j] {
				return -1
			} else if entries[i] > entries[j] {
				return 1
			}
			return 0
		})

		thisSize := 4 + len(entries)*8

		// Map current prefix to its relative offset within the content section.
		prefixToOffset[prefix] = previousOffset

		// Write bucket metadata (count)
		if err := binary.Write(out, binary.LittleEndian, uint32(len(entries))); err != nil {
			return nil, 0, err
		}
		// Write hashes
		for _, h := range entries {
			if err := binary.Write(out, binary.LittleEndian, h); err != nil {
				return nil, 0, err
			}
		}

		previousOffset += uint64(thisSize)
		totalWritten += int64(thisSize)
	}

	// flush the buckets buffer:
	if err := out.Flush(); err != nil {
		return nil, 0, err
	}
	updatedHeader, err := createHeader(
		_Magic,
		Version,
		uint32(headerSize-4), // Borsh doesn't count the size field itself in some protocols.
		meta,
		prefixToOffset,
	)
	if err != nil {
		return nil, 0, err
	}
	return updatedHeader, totalWritten, nil
}

func getCleanSet(entries []uint64) []uint64 {
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i] < entries[j]
	})
	out := make([]uint64, 0, len(entries))
	for i := 0; i < len(entries); i++ {
		if i > 0 && entries[i] == entries[i-1] {
			continue
		}
		out = append(out, entries[i])
	}
	return out
}

func fileIsBlank(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return info.Size() == 0, nil
}

func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func uint16ToPrefix(num uint16) [2]byte {
	var prefix [2]byte
	binary.LittleEndian.PutUint16(prefix[:], num)
	return prefix
}
