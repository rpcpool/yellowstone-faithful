package bucketteer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	bin "github.com/gagliardetto/binary"
)

type Reader struct {
	prefixToOffset map[[2]byte]uint64
	contentReader  io.ReaderAt
}

func NewReader(reader io.ReaderAt) (*Reader, error) {
	r := &Reader{
		prefixToOffset: make(map[[2]byte]uint64),
	}
	headerTotalSize, err := r.readHeader(reader)
	if err != nil {
		return nil, err
	}
	r.contentReader = io.NewSectionReader(reader, headerTotalSize, 1<<63-1)
	return r, nil
}

func (r *Reader) readHeaderSize(reader io.ReaderAt) (int64, error) {
	// read header size:
	headerSizeBuf := make([]byte, 4)
	if _, err := reader.ReadAt(headerSizeBuf, 0); err != nil {
		return 0, err
	}
	headerSize := int64(binary.LittleEndian.Uint32(headerSizeBuf))
	return headerSize, nil
}

func (r *Reader) readHeader(reader io.ReaderAt) (int64, error) {
	// read header size:
	headerSize, err := r.readHeaderSize(reader)
	if err != nil {
		return 0, err
	}
	// read header bytes:
	headerBuf := make([]byte, headerSize)
	if _, err := reader.ReadAt(headerBuf, 4); err != nil {
		return 0, err
	}
	// decode header:
	decoder := bin.NewBorshDecoder(headerBuf)

	// magic:
	{
		magicBuf := make([]byte, len(_Magic[:]))
		_, err := decoder.Read(magicBuf)
		if err != nil {
			return 0, err
		}
		if !bytes.Equal(magicBuf, _Magic[:]) {
			return 0, fmt.Errorf("invalid magic: %x", string(magicBuf))
		}
	}
	// version:
	{
		got, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return 0, err
		}
		if got != Version {
			return 0, fmt.Errorf("expected version %d, got %d", Version, got)
		}
	}
	// numPrefixes:
	numPrefixes, err := decoder.ReadUint64(bin.LE)
	if err != nil {
		return 0, err
	}
	// prefix -> offset:
	r.prefixToOffset = make(map[[2]byte]uint64, numPrefixes)
	for i := uint64(0); i < numPrefixes; i++ {
		var prefix [2]byte
		_, err := decoder.Read(prefix[:])
		if err != nil {
			return 0, err
		}
		offset, err := decoder.ReadUint64(bin.LE)
		if err != nil {
			return 0, err
		}
		r.prefixToOffset[prefix] = offset
	}
	return headerSize + 4, nil
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
