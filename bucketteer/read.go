package bucketteer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	bin "github.com/gagliardetto/binary"
	"golang.org/x/exp/mmap"
)

type Reader struct {
	contentReader  io.ReaderAt
	meta           map[string]string
	prefixToOffset map[[2]byte]uint64
}

// Open opens a Bucketteer file in read-only mode,
// using memory-mapped IO.
func Open(path string) (*Reader, error) {
	file, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}
	return NewReader(file)
}

func NewReader(reader io.ReaderAt) (*Reader, error) {
	r := &Reader{
		prefixToOffset: make(map[[2]byte]uint64),
	}
	prefixToOffset, meta, headerTotalSize, err := readHeader(reader)
	if err != nil {
		return nil, err
	}
	r.meta = meta
	r.prefixToOffset = prefixToOffset
	r.contentReader = io.NewSectionReader(reader, headerTotalSize, 1<<63-1)
	return r, nil
}

func (r *Reader) Close() error {
	if closer, ok := r.contentReader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (r *Reader) Meta() map[string]string {
	return r.meta
}

// GetMeta returns the value of the given key.
// Returns an empty string if the key does not exist.
func (r *Reader) GetMeta(key string) string {
	return r.meta[key]
}

func readHeaderSize(reader io.ReaderAt) (int64, error) {
	// read header size:
	headerSizeBuf := make([]byte, 4)
	if _, err := reader.ReadAt(headerSizeBuf, 0); err != nil {
		return 0, err
	}
	headerSize := int64(binary.LittleEndian.Uint32(headerSizeBuf))
	return headerSize, nil
}

func readHeader(reader io.ReaderAt) (map[[2]byte]uint64, map[string]string, int64, error) {
	// read header size:
	headerSize, err := readHeaderSize(reader)
	if err != nil {
		return nil, nil, 0, err
	}
	// read header bytes:
	headerBuf := make([]byte, headerSize)
	if _, err := reader.ReadAt(headerBuf, 4); err != nil {
		return nil, nil, 0, err
	}
	// decode header:
	decoder := bin.NewBorshDecoder(headerBuf)

	// magic:
	{
		magicBuf := make([]byte, len(_Magic[:]))
		_, err := decoder.Read(magicBuf)
		if err != nil {
			return nil, nil, 0, err
		}
		if !bytes.Equal(magicBuf, _Magic[:]) {
			return nil, nil, 0, fmt.Errorf("invalid magic: %x", string(magicBuf))
		}
	}
	// version:
	{
		got, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, err
		}
		if got != Version {
			return nil, nil, 0, fmt.Errorf("expected version %d, got %d", Version, got)
		}
	}
	{
		// read meta:
		numMeta, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, err
		}
		meta := make(map[string]string, numMeta)
		for i := uint64(0); i < numMeta; i++ {
			key, err := decoder.ReadString()
			if err != nil {
				return nil, nil, 0, err
			}
			value, err := decoder.ReadString()
			if err != nil {
				return nil, nil, 0, err
			}
			meta[key] = value
		}
	}
	// numPrefixes:
	numPrefixes, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return nil, nil, 0, err
	}
	// prefix -> offset:
	prefixToOffset := make(map[[2]byte]uint64, numPrefixes)
	for i := uint64(0); i < numPrefixes; i++ {
		var prefix [2]byte
		_, err := decoder.Read(prefix[:])
		if err != nil {
			return nil, nil, 0, err
		}
		offset, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return nil, nil, 0, err
		}
		prefixToOffset[prefix] = offset
	}
	return prefixToOffset, nil, headerSize + 4, err
}

func (r *Reader) Has(sig [64]byte) (bool, error) {
	prefix := [2]byte{sig[0], sig[1]}
	offset, ok := r.prefixToOffset[prefix]
	if !ok {
		return false, nil
	}
	// numHashes:
	numHashesBuf := make([]byte, 4)
	_, err := r.contentReader.ReadAt(numHashesBuf, int64(offset))
	if err != nil {
		return false, err
	}
	numHashes := binary.LittleEndian.Uint32(numHashesBuf)
	bucketReader := io.NewSectionReader(r.contentReader, int64(offset)+4, int64(numHashes*8))

	// hashes:
	wantedHash := Hash(sig)
	got, err := searchEytzinger(0, int(numHashes), wantedHash, func(index int) (uint64, error) {
		pos := int64(index * 8)
		return readUint64Le(bucketReader, pos)
	})
	if err != nil {
		if err == ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return got == wantedHash, nil
}

func searchEytzinger(min int, max int, x uint64, getter func(int) (uint64, error)) (uint64, error) {
	var index int
	for index < max {
		k, err := getter(index)
		if err != nil {
			return 0, err
		}
		if k == x {
			return k, nil
		}
		index = index<<1 | 1
		if k < x {
			index++
		}
	}
	return 0, ErrNotFound
}

var ErrNotFound = fmt.Errorf("not found")

func readUint64Le(reader io.ReaderAt, pos int64) (uint64, error) {
	buf := make([]byte, 8)
	_, err := reader.ReadAt(buf, pos)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf), nil
}
