package bucketteer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"

	bin "github.com/gagliardetto/binary"
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
	for i := range out {
		out[i] = make([]uint64, 0, 16_000)
	}
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
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	pU16 := prefixToUint16(prefix)
	b.prefixToHashes[pU16] = append(b.prefixToHashes[pU16], Hash(sig))
}

// Has returns true if the Bucketteer has seen the given signature.
func (b *Writer) Has(sig [64]byte) bool {
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	hash := Hash(sig)
	for _, h := range b.prefixToHashes[prefixToUint16(prefix)] {
		if h == hash {
			return true
		}
	}
	return false
}

func (b *Writer) Close() error {
	if b.writer != nil {
		if err := b.writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush writer: %w", err)
		}
	}
	if b.destination == nil {
		return nil
	}
	if err := b.destination.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	return b.destination.Close()
}

// Seal writes the Bucketteer's state to the given writer.
func (b *Writer) Seal(meta indexmeta.Meta) (int64, error) {
	file, err := os.Create(b.path)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	b.writer = bufio.NewWriterSize(file, writeBufSize)
	b.destination = file

	// truncate file and seek to beginning:
	if err := b.destination.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := b.destination.Seek(0, 0); err != nil {
		return 0, err
	}
	newHeader, size, err := seal(b.writer, b.prefixToHashes, meta)
	if err != nil {
		return 0, err
	}
	{
		// flush the writer:
		if err := b.writer.Flush(); err != nil {
			return 0, fmt.Errorf("failed to flush writer: %w", err)
		}
		// sync the file:
		if err := b.destination.Sync(); err != nil {
			return 0, fmt.Errorf("failed to sync file: %w", err)
		}
	}
	return size, overwriteFileContentAt(b.destination, 0, newHeader)
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

	// write header size:
	if err := headerWriter.WriteUint32(headerSizeIn, binary.LittleEndian); err != nil {
		return nil, err
	}
	// write magic:
	if n, err := headerWriter.Write(magic[:]); err != nil {
		return nil, err
	} else {
		if n != 8 {
			return nil, fmt.Errorf("invalid number of bytes written for magic: %d", n)
		}
	}
	// write version uint64
	if err := headerWriter.WriteUint64(version, binary.LittleEndian); err != nil {
		return nil, err
	}
	// write meta
	{
		metaBuf, err := meta.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if _, err := headerWriter.Write(metaBuf); err != nil {
			return nil, fmt.Errorf("failed to write metadata: %w", err)
		}
	}
	// write num buckets
	if err := headerWriter.WriteUint64(uint64(len(prefixToOffset)), binary.LittleEndian); err != nil {
		return nil, err
	}

	// write prefix+offset pairs
	for prefixAsUint16 := range prefixToOffset {
		prefix := uint16ToPrefix(uint16(prefixAsUint16))
		if _, err := headerWriter.Write(prefix[:]); err != nil {
			return nil, err
		}
		offset := prefixToOffset[prefixAsUint16]
		if err := headerWriter.WriteUint64(offset, binary.LittleEndian); err != nil {
			return nil, err
		}
	}
	return tmpHeaderBuf.Bytes(), nil
}

func overwriteFileContentAt(
	file *os.File,
	offset int64,
	data []byte,
) error {
	wrote, err := file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if wrote != len(data) {
		return fmt.Errorf("wrote %d bytes, expected to write %d bytes", wrote, len(data))
	}
	return err
}

func seal(
	out *bufio.Writer,
	prefixToHashes *prefixToHashes,
	meta indexmeta.Meta,
) ([]byte, int64, error) {
	prefixToOffset := bucketToOffset{}
	for prefixAsUint16 := range prefixToHashes {
		// initialize all offsets to 0:
		prefixToOffset[prefixAsUint16] = 0
	}

	totalWritten := int64(0)
	// create and write draft header:
	header, err := createHeader(
		_Magic,
		Version,
		0, // header size
		meta,
		prefixToOffset,
	)
	if err != nil {
		return nil, 0, err
	}
	headerSize, err := out.Write(header)
	if err != nil {
		return nil, 0, err
	}
	totalWritten += int64(headerSize)

	previousOffset := uint64(0)
	for prefix := range prefixToHashes {
		entries := getCleanSet(prefixToHashes[prefix])
		if len(entries) != len(prefixToHashes[prefix]) {
			klog.Errorf("duplicate hashes for prefix %v", prefix)
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
		// write the clean set to the buckets buffer
		if err := binary.Write(out, binary.LittleEndian, uint32(len(entries))); err != nil {
			return nil, 0, err
		}
		for _, h := range entries {
			if err := binary.Write(out, binary.LittleEndian, h); err != nil {
				return nil, 0, err
			}
		}

		prefixToOffset[prefix] = previousOffset
		previousOffset = previousOffset + uint64(thisSize)
		totalWritten += int64(thisSize)
	}

	// flush the buckets buffer:
	if err := out.Flush(); err != nil {
		return nil, 0, err
	}

	// write final header by overwriting the draft header:
	updatedHeader, err := createHeader(
		_Magic,
		Version,
		uint32(headerSize-4), // -4 because we don't count the header size itself (it's a uint32, so 4 bytes long)
		meta,
		prefixToOffset,
	)
	if err != nil {
		return nil, 0, err
	}
	return updatedHeader, totalWritten, err
}

// getCleanSet returns a sorted, deduplicated copy of getCleanSet.
func getCleanSet(entries []uint64) []uint64 {
	// sort:
	sort.Slice(entries, func(i, j int) bool {
		return entries[i] < entries[j]
	})
	// dedup:
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
