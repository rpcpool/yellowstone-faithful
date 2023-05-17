package main

import (
	"bufio"
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
)

func readHeader(br io.Reader) (*carv1.CarHeader, error) {
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

type carReader struct {
	br     *bufio.Reader
	header *carv1.CarHeader
}

func newCarReader(r io.Reader) (*carReader, error) {
	br := bufio.NewReader(r)
	ch, err := readHeader(br)
	if err != nil {
		return nil, err
	}

	if ch.Version != 1 {
		return nil, fmt.Errorf("invalid car version: %d", ch.Version)
	}

	if len(ch.Roots) == 0 {
		return nil, fmt.Errorf("empty car, no roots")
	}

	return &carReader{
		br:     br,
		header: ch,
	}, nil
}

func readNodeInfoWithoutData(br *bufio.Reader) (cid.Cid, uint64, error) {
	sectionLen, ll, err := readSectionLength(br)
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

func readNodeInfoWithData(br *bufio.Reader) (cid.Cid, uint64, []byte, error) {
	sectionLen, ll, err := readSectionLength(br)
	if err != nil {
		return cid.Cid{}, 0, nil, err
	}

	cidLen, c, err := cid.CidFromReader(br)
	if err != nil {
		return cid.Cid{}, 0, nil, err
	}

	// Seek to the next section by skipping the block.
	// The section length includes the CID, so subtract it.
	remainingSectionLen := int64(sectionLen) - int64(cidLen)

	buf := make([]byte, remainingSectionLen)
	_, err = io.ReadFull(br, buf)
	if err != nil {
		return cid.Cid{}, 0, nil, err
	}

	return c, sectionLen + ll, buf, nil
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

func readSectionLength(r *bufio.Reader) (uint64, uint64, error) {
	if _, err := r.Peek(1); err != nil { // no more blocks, likely clean io.EOF
		return 0, 0, err
	}

	br := byteReaderWithCounter{r, 0}
	l, err := binary.ReadUvarint(&br)
	if err != nil {
		if err == io.EOF {
			return 0, 0, io.ErrUnexpectedEOF // don't silently pretend this is a clean EOF
		}
		return 0, 0, err
	}

	if l > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return 0, 0, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}

	return l, br.Offset, nil
}

func (cr *carReader) NextInfo() (cid.Cid, uint64, error) {
	c, sectionLen, err := readNodeInfoWithoutData(cr.br)
	if err != nil {
		return c, 0, err
	}
	return c, sectionLen, nil
}

func (cr *carReader) NextNode() (cid.Cid, uint64, *blocks.BasicBlock, error) {
	c, sectionLen, data, err := readNodeInfoWithData(cr.br)
	if err != nil {
		return c, 0, nil, err
	}
	bl, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return c, 0, nil, err
	}
	return c, sectionLen, bl, nil
}

func isDirEmpty(dir string) (bool, error) {
	file, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = file.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func getFileSize(path string) (uint64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return uint64(st.Size()), nil
}

func carCountItems(carPath string) (uint64, error) {
	file, err := os.Open(carPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	rd, err := newCarReader(file)
	if err != nil {
		return 0, fmt.Errorf("failed to open car file: %w", err)
	}

	var count uint64
	for {
		_, _, err := rd.NextInfo()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

func printToStderr(msg string) {
	fmt.Fprint(os.Stderr, msg)
}
