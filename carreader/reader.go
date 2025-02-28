package carreader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-libipfs/blocks"
	carv1 "github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/rpcpool/yellowstone-faithful/readahead"
)

type CarReader struct {
	totalOffset uint64
	headerSize  *uint64
	Header      *carv1.CarHeader
	br          *bufio.Reader
}

func alignValueToPageSize(value int) int {
	pageSize := os.Getpagesize()
	return (value + pageSize - 1) &^ (pageSize - 1)
}

func New(r io.ReadCloser) (*CarReader, error) {
	br := bufio.NewReaderSize(r, alignValueToPageSize(readahead.DefaultChunkSize))
	ch, err := ReadHeader(br)
	if err != nil {
		return nil, err
	}

	if ch.Version != 1 {
		return nil, fmt.Errorf("invalid car version: %d", ch.Version)
	}

	// TODO: ???
	// if len(ch.Roots) == 0 {
	// 	return nil, fmt.Errorf("empty car, no roots")
	// }

	cr := &CarReader{
		br:     br,
		Header: ch,
	}

	headerSize, err := cr.HeaderSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get header size: %w", err)
	}
	cr.totalOffset = headerSize

	return cr, nil
}

func ReadHeader(br io.Reader) (*carv1.CarHeader, error) {
	hb, err := util.LdRead(bufio.NewReader(br))
	if err != nil {
		return nil, err
	}

	var ch carv1.CarHeader
	if err := cbor.DecodeInto(hb, &ch); err != nil {
		return nil, fmt.Errorf("invalid header: %v", err)
	}

	return &ch, nil
}

func (cr *CarReader) ReadAt(p []byte, off int64) (n int, err error) {
	panic("not implemented")
}

func (cr *CarReader) Next() (blocks.Block, error) {
	_, _, block, err := cr.NextNode()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (cr *CarReader) NextInfo() (cid.Cid, uint64, error) {
	c, sectionLen, err := ReadNodeInfoWithoutData(cr.br)
	if err != nil {
		return c, 0, err
	}
	cr.totalOffset += sectionLen
	return c, sectionLen, nil
}

func (cr *CarReader) NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error) {
	c, sectionLen, data, err := ReadNodeInfoWithData(cr.br)
	if err != nil {
		return c, 0, nil, fmt.Errorf("failed to read node info: %w", err)
	}
	bl, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return c, 0, nil, fmt.Errorf("failed to create block: %w", err)
	}
	cr.totalOffset += sectionLen
	return c, sectionLen, bl, nil
}

func (cr *CarReader) NextNodeBytes() (cid.Cid, uint64, []byte, error) {
	c, sectionLen, data, err := ReadNodeInfoWithData(cr.br)
	if err != nil {
		return c, 0, nil, fmt.Errorf("failed to read node info: %w", err)
	}
	cr.totalOffset += sectionLen
	return c, sectionLen, data, nil
}

func (cr *CarReader) HeaderSize() (uint64, error) {
	if cr.headerSize == nil {
		var buf bytes.Buffer
		if err := carv1.WriteHeader(cr.Header, &buf); err != nil {
			return 0, err
		}
		size := uint64(buf.Len())
		cr.headerSize = &size
	}
	return *cr.headerSize, nil
}

func (cr *CarReader) Close() error {
	return nil
}

func (cr *CarReader) GetGlobalOffsetForNextRead() (uint64, bool) {
	// NOTE: this will NOT return false because we don't know the total size of the file.
	return cr.totalOffset, true
}

func ReadNodeInfoWithoutData(br *bufio.Reader) (cid.Cid, uint64, error) {
	sectionLen, ll, err := ReadSectionLength(br)
	if err != nil {
		return cid.Cid{}, 0, err
	}

	cidLen, c, err := cid.CidFromReader(br)
	if err != nil {
		return cid.Cid{}, 0, err
	}

	// Seek to the next section by skipping the block.
	// The section length includes the CID, so subtract it.
	remainingSectionLen := int64(sectionLen) - int64(cidLen)

	_, err = io.CopyN(io.Discard, br, remainingSectionLen)
	if err != nil {
		return cid.Cid{}, 0, err
	}

	return c, sectionLen + ll, nil
}

func ReadNodeInfoWithData(br *bufio.Reader) (cid.Cid, uint64, []byte, error) {
	sectionLen, ll, err := ReadSectionLength(br)
	if err != nil {
		return cid.Cid{}, 0, nil, fmt.Errorf("failed to read section length: %w", err)
	}

	cidLen, c, err := cid.CidFromReader(br)
	if err != nil {
		return cid.Cid{}, 0, nil, fmt.Errorf("failed to read cid: %w", err)
	}

	// Seek to the next section by skipping the block.
	// The section length includes the CID, so subtract it.
	remainingSectionLen := int64(sectionLen) - int64(cidLen)

	buf := make([]byte, remainingSectionLen)
	_, err = io.ReadFull(br, buf)
	if err != nil {
		return cid.Cid{}, 0, nil, fmt.Errorf("failed to read block: %w", err)
	}

	return c, sectionLen + ll, buf, nil
}

func ReadSectionLength(r *bufio.Reader) (uint64, uint64, error) {
	if _, err := r.Peek(1); err != nil { // no more blocks, likely clean io.EOF
		if errors.Is(err, io.ErrNoProgress) {
			return 0, 0, io.EOF
		}
		return 0, 0, fmt.Errorf("failed to peek: %w", err)
	}

	br := byteReaderWithCounter{r, 0}
	l, err := binary.ReadUvarint(&br)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, 0, io.ErrUnexpectedEOF // don't silently pretend this is a clean EOF
		}
		return 0, 0, err
	}

	if l > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return 0, 0, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}

	return l, br.Offset, nil
}

type byteReaderWithCounter struct {
	io.ByteReader
	Offset uint64
}

func (b *byteReaderWithCounter) ReadByte() (byte, error) {
	c, err := b.ByteReader.ReadByte()
	if err == nil {
		b.Offset++
	}
	return c, err
}
