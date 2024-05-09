package bucketteer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"

	bin "github.com/gagliardetto/binary"
	"k8s.io/klog/v2"
)

type Writer struct {
	destination    *os.File
	writer         *bufio.Writer
	prefixToHashes map[[2]byte][]uint64 // prefix -> hashes
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
		return nil, fmt.Errorf("file is not blank")
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Writer{
		writer:         bufio.NewWriterSize(file, writeBufSize),
		destination:    file,
		prefixToHashes: make(map[[2]byte][]uint64),
	}, nil
}

// Put adds the given signature to the Bucketteer.
// Cannot be called concurrently.
func (b *Writer) Put(sig [64]byte) {
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	b.prefixToHashes[prefix] = append(b.prefixToHashes[prefix], Hash(sig))
}

// Has returns true if the Bucketteer has seen the given signature.
func (b *Writer) Has(sig [64]byte) bool {
	var prefix [2]byte
	copy(prefix[:], sig[:2])
	hash := Hash(sig)
	for _, h := range b.prefixToHashes[prefix] {
		if h == hash {
			return true
		}
	}
	return false
}

func (b *Writer) Close() error {
	return b.destination.Close()
}

// Seal writes the Bucketteer's state to the given writer.
func (b *Writer) Seal(meta map[string]string) (int64, error) {
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
	return size, overwriteFileContentAt(b.destination, 0, newHeader)
}

func createHeader(
	magic [8]byte,
	version uint64,
	headerSizeIn uint32,
	meta map[string]string,
	prefixToOffset map[[2]byte]uint64,
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
		// write num meta entries
		if err := headerWriter.WriteUint64(uint64(len(meta)), binary.LittleEndian); err != nil {
			return nil, err
		}
		// write meta entries
		for k, v := range meta {
			if err := headerWriter.WriteString(k); err != nil {
				return nil, err
			}
			if err := headerWriter.WriteString(v); err != nil {
				return nil, err
			}
		}
	}
	// write num buckets
	if err := headerWriter.WriteUint64(uint64(len(prefixToOffset)), binary.LittleEndian); err != nil {
		return nil, err
	}

	prefixes := getSortedPrefixes(prefixToOffset)
	// write prefix+offset pairs
	for _, prefix := range prefixes {
		if _, err := headerWriter.Write(prefix[:]); err != nil {
			return nil, err
		}
		offset := prefixToOffset[prefix]
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

func getSortedPrefixes[K any](prefixToHashes map[[2]byte]K) [][2]byte {
	prefixes := make([][2]byte, 0, len(prefixToHashes))
	for prefix := range prefixToHashes {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return bytes.Compare(prefixes[i][:], prefixes[j][:]) < 0
	})
	return prefixes
}

func seal(
	out *bufio.Writer,
	prefixToHashes map[[2]byte][]uint64,
	meta map[string]string,
) ([]byte, int64, error) {
	prefixes := getSortedPrefixes(prefixToHashes)
	prefixToOffset := make(map[[2]byte]uint64, len(prefixes))
	for _, prefix := range prefixes {
		// initialize all offsets to 0:
		prefixToOffset[prefix] = 0
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
	for _, prefix := range prefixes {
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
		uint32(headerSize-4), // -4 because we don't count the header size itself
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
