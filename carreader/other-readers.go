package carreader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	"github.com/valyala/bytebufferpool"
)

func ReadNodeSizeFromReaderAtWithOffset(reader io.ReaderAt, offset uint64) (uint64, error) {
	// read MaxVarintLen64 bytes
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := reader.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return 0, err
	}
	// read uvarint
	dataLen, n := binary.Uvarint(lenBuf)
	dataLen += uint64(n)
	if dataLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return 0, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	return dataLen, nil
}

func ReadNodeWithKnownSize(br *bufio.Reader, wantedCid *cid.Cid, length uint64) ([]byte, error) {
	section := make([]byte, length)
	_, err := io.ReadFull(br, section)
	if err != nil {
		return nil, fmt.Errorf("failed to read section from CAR with length %d: %w", length, err)
	}
	return ParseNodeFromSection(section, wantedCid)
}

// ParseNodeFromSection parses a section and returns the data (omitting the CID)
func ParseNodeFromSection(section []byte, wantedCid *cid.Cid) ([]byte, error) {
	// read an uvarint from the buffer
	gotLen, usize := binary.Uvarint(section)
	if usize <= 0 {
		return nil, fmt.Errorf("failed to decode uvarint")
	}
	if gotLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	data := section[usize:]
	cidLen, gotCid, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read cid: %w", err)
	}
	// verify that the CID we read matches the one we expected.
	if wantedCid != nil && !gotCid.Equals(*wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data[cidLen:], nil
}

func ParseNodeFromSectionBuffer(section *bytebufferpool.ByteBuffer, wantedCid *cid.Cid) (*bytebufferpool.ByteBuffer, error) {
	// read an uvarint from the buffer
	gotLen, usize := binary.Uvarint(section.B)
	if usize <= 0 {
		return nil, fmt.Errorf("failed to decode uvarint")
	}
	if gotLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	cidLen, gotCid, err := cid.CidFromReader(bytes.NewReader(section.B[usize:]))
	if err != nil {
		return nil, fmt.Errorf("failed to read cid: %w", err)
	}
	// verify that the CID we read matches the one we expected.
	if wantedCid != nil && !gotCid.Equals(*wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	dataStart := usize + cidLen
	dataEnd := int(gotLen) + usize

	section.B = section.B[dataStart:dataEnd]
	return section, nil
}

func ReadAllFromReaderAt(reader io.ReaderAt, size uint64) ([]byte, error) {
	buf := make([]byte, size)
	n, err := reader.ReadAt(buf, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}
	if uint64(n) != size {
		return nil, fmt.Errorf("failed to read all bytes: expected %d, got %d", size, n)
	}
	return buf, nil
}

func ReadSectionFromReaderAt(reader ReaderAtCloser, offset uint64, length uint64) ([]byte, error) {
	data := make([]byte, length)
	_, err := reader.ReadAt(data, int64(offset))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func ReadNodeFromReaderAtWithOffsetAndSize(reader ReaderAtCloser, wantedCid *cid.Cid, offset uint64, length uint64) ([]byte, error) {
	// read MaxVarintLen64 bytes
	section := make([]byte, length)
	_, err := reader.ReadAt(section, int64(offset))
	if err != nil {
		return nil, err
	}
	return ParseNodeFromSection(section, wantedCid)
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

func ReadIntoBuffer(offset uint64, length uint64, dr io.ReaderAt) (*bytebufferpool.ByteBuffer, error) {
	if dr == nil {
		return nil, fmt.Errorf("reader is nil")
	}
	buf := bytebufferpool.Get()
	buf.B = make([]byte, length)
	_, err := dr.ReadAt(buf.B, int64(offset))
	if err != nil {
		bytebufferpool.Put(buf)
		return nil, fmt.Errorf("failed to read from reader at offset %d: %w", offset, err)
	}
	return buf, nil
}
