package bucketteer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	bin "github.com/gagliardetto/binary"
)

type Writer struct {
	prefixToHashes map[[2]byte][]uint64 // prefix -> hashes
}

func NewWriter() *Writer {
	return &Writer{
		prefixToHashes: make(map[[2]byte][]uint64),
	}
}

// Push adds the given signature to the Bucketteer.
// Cannot be called concurrently.
func (b *Writer) Push(sig [64]byte) {
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

type WriterWriterAt interface {
	io.Writer
	io.WriterAt
}

func writeHeader(
	out WriterWriterAt,
	magic [8]byte,
	version uint64,
	headerSizeIn uint32,
	numBuckets uint64,
	prefixToOffset map[[2]byte]uint64,
) (int64, error) {
	tmpHeaderBuf := new(bytes.Buffer)
	headerWriter := bin.NewBorshEncoder(tmpHeaderBuf)
	// write header size
	{
		// write empty header size
		if err := headerWriter.WriteUint32(headerSizeIn, binary.LittleEndian); err != nil {
			return 0, err
		}
	}
	headerSize := 0
	// write magic
	if n, err := headerWriter.Write(magic[:]); err != nil {
		return 0, err
	} else {
		headerSize += n
	}
	// write version uint64
	if err := headerWriter.WriteUint64(version, binary.LittleEndian); err != nil {
		return 0, err
	} else {
		headerSize += 8
	}
	// write num buckets
	if err := headerWriter.WriteUint64(numBuckets, binary.LittleEndian); err != nil {
		return 0, err
	} else {
		headerSize += 8
	}

	prefixes := make([][2]byte, 0, len(prefixToOffset))
	for prefix := range prefixToOffset {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return bytes.Compare(prefixes[i][:], prefixes[j][:]) < 0
	})
	// write prefix+offset pairs
	for _, prefix := range prefixes {
		if _, err := headerWriter.Write(prefix[:]); err != nil {
			return 0, err
		}
		offset := prefixToOffset[prefix]
		if err := headerWriter.WriteUint64(offset, binary.LittleEndian); err != nil {
			return 0, err
		}
	}
	// write all the data to the writer
	n, err := out.WriteAt(tmpHeaderBuf.Bytes(), 0)
	// Return the header size without the header size itself.
	return int64(n) - 4, err
}

// WriteTo writes the Bucketteer's state to the given writer.
func (b *Writer) WriteTo(out WriterWriterAt) (int64, error) {
	prefixes := make([][2]byte, 0, len(b.prefixToHashes))
	for prefix := range b.prefixToHashes {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return bytes.Compare(prefixes[i][:], prefixes[j][:]) < 0
	})
	prefixToOffset := make(map[[2]byte]uint64)

	totalWritten := int64(0)
	// write draft header:
	headerSize, err := writeHeader(
		out,
		_Magic,
		Version,
		0, // header size
		uint64(len(prefixes)),
		prefixToOffset,
	)
	if err != nil {
		return 0, err
	}
	totalWritten += headerSize + 4 // +4 because of the header size itself

	previousOffset := uint64(0)
	for _, prefix := range prefixes {
		entries := getCleanSet(b.prefixToHashes[prefix])
		if len(entries) != len(b.prefixToHashes[prefix]) {
			panic(fmt.Sprintf("duplicate hashes for prefix %v", prefix))
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
			return 0, err
		}
		for _, h := range entries {
			if err := binary.Write(out, binary.LittleEndian, h); err != nil {
				return 0, err
			}
		}

		prefixToOffset[prefix] = previousOffset
		previousOffset = previousOffset + uint64(thisSize)
		totalWritten += int64(thisSize)
	}

	// write final header by overwriting the draft header:
	_, err = writeHeader(
		out,
		_Magic,
		Version,
		uint32(headerSize),
		uint64(len(prefixes)),
		prefixToOffset,
	)
	if err != nil {
		return 0, err
	}
	return totalWritten, nil
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
