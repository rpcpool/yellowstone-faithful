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

// ParseNodeFromSectionBuffer extracts the CID and slices the buffer to the block data.
// Logic Trace:
// 1. Decode uvarint 'gotLen' which represents the total length of [CID + Data].
// 2. Wrap remaining buffer in a bytes.Reader (implements io.ByteReader for efficiency).
// 3. Call CidFromReader to get the CID and its encoded length 'cidLen'.
// 4. Verify CID identity.
// 5. Slice original buffer: dataStart (uvarint_size + cid_size) to dataEnd (uvarint_size + gotLen).
func ParseNodeFromSectionBuffer(section *bytebufferpool.ByteBuffer, wantedCid *cid.Cid) (*bytebufferpool.ByteBuffer, error) {
	if section == nil || len(section.B) == 0 {
		return nil, errors.New("empty section buffer")
	}

	// 1. Decode Uvarint (Length of CID + Data)
	gotLen, usize := binary.Uvarint(section.B)
	if usize <= 0 {
		return nil, errors.New("failed to decode uvarint: malformed header")
	}

	// log.Printf("[DEBUG] ParseNode: section_content_len=%d, uvarint_bytes=%d", gotLen, usize)

	if gotLen > uint64(util.MaxAllowedSectionSize) {
		return nil, fmt.Errorf("section size %d exceeds limit %d", gotLen, util.MaxAllowedSectionSize)
	}

	// 2. Extract CID
	// bytes.Reader implements io.ByteReader, satisfying the efficiency recommendation.
	remainingData := section.B[usize:]
	r := bytes.NewReader(remainingData)

	cidLen, gotCid, err := cid.CidFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CID: %w", err)
	}

	// log.Printf("[DEBUG] ParseNode: CID=%s, cid_encoded_bytes=%d", gotCid, cidLen)

	// 3. Verify CID identity
	if wantedCid != nil && !gotCid.Equals(*wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}

	// 4. Calculate Slices
	// The uvarint 'gotLen' covers [CID + Data].
	// Therefore, the block data ends at 'usize + gotLen'.
	dataStart := usize + cidLen
	dataEnd := usize + int(gotLen)

	if dataEnd > len(section.B) {
		return nil, fmt.Errorf("buffer underrun: section claims %d bytes but buffer has %d", dataEnd, len(section.B))
	}

	// log.Printf("[DEBUG] ParseNode: slicing block data [%d:%d]", dataStart, dataEnd)

	// In-place slice update to isolate block data.
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

// ReadIntoBuffer performs a bounded, full read from a ReaderAt into a pooled buffer.
func ReadIntoBuffer(offset uint64, length uint64, dr io.ReaderAt) (*bytebufferpool.ByteBuffer, error) {
	if dr == nil {
		return nil, errors.New("reader is nil")
	}

	// log.Printf("[DEBUG] ReadIntoBuffer: offset=%d length=%d", offset, length)

	buf := bytebufferpool.Get()

	// Reset slice but maintain underlying capacity.
	buf.B = buf.B[:0]
	if cap(buf.B) < int(length) {
		buf.B = make([]byte, length)
	} else {
		buf.B = buf.B[:length]
	}

	// ReadAt into the pre-allocated slice.
	n, err := dr.ReadAt(buf.B, int64(offset))
	if err != nil && err != io.EOF {
		bytebufferpool.Put(buf)
		return nil, fmt.Errorf("read error at offset %d: %w", offset, err)
	}

	if uint64(n) < length {
		bytebufferpool.Put(buf)
		return nil, fmt.Errorf("short read at offset %d: expected %d, got %d", offset, length, n)
	}

	return buf, nil
}
