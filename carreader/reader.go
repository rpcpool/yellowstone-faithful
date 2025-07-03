package carreader

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/go-libipfs/blocks"
	carv1 "github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	"github.com/rpcpool/yellowstone-faithful/readahead"
)

// prefetchedBlock holds the data for a block that has been read from disk
// and is ready for processing.
type prefetchedBlock struct {
	cid        cid.Cid
	data       []byte
	sectionLen uint64
	err        error
}

// PrefetchingCarReader provides a high-performance, concurrent reader for CARv1 files
// that uses prefetching to maximize I/O throughput.
type PrefetchingCarReader struct {
	// Header contains the parsed CARv1 header.
	Header *carv1.CarHeader
	// totalOffset is the current position in the CAR file, pointing to the start of the next block.
	totalOffset uint64
	// headerSize is the size of the CAR header in bytes.
	headerSize uint64

	// br is the buffered reader for the underlying CAR file stream.
	br *bufio.Reader
	// r is the original reader, kept for closing.
	r io.ReadCloser

	// prefetchChan is the channel through which prefetched blocks are delivered.
	prefetchChan chan prefetchedBlock
	// closeOnce ensures the closing logic is executed only once.
	closeOnce sync.Once
	// wg waits for the prefetching goroutine to finish.
	wg sync.WaitGroup
	// bufferPool manages reusable byte buffers for block data to reduce allocations.
	bufferPool sync.Pool
}

// alignValueToPageSize rounds the given value up to the nearest system page size.
func alignValueToPageSize(value int) int {
	pageSize := os.Getpagesize()
	return (value + pageSize - 1) &^ (pageSize - 1)
}

// NewPrefetching creates a new PrefetchingCarReader from an io.ReadCloser.
// It starts a background goroutine to prefetch blocks from the CAR file, which can
// significantly improve performance on fast storage like SSDs.
// `prefetchingSize` controls the size of the prefetch buffer.
func NewPrefetching(r io.ReadCloser, prefetchingSize int) (*PrefetchingCarReader, error) {
	br := bufio.NewReaderSize(r, alignValueToPageSize(readahead.DefaultChunkSize))

	header, headerSize, err := readHeader(br)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty car file")
		}
		return nil, fmt.Errorf("failed to read car header: %w", err)
	}

	if header.Version != 1 {
		return nil, fmt.Errorf("invalid car version: %d", header.Version)
	}

	cr := &PrefetchingCarReader{
		r:            r,
		br:           br,
		Header:       header,
		headerSize:   headerSize,
		totalOffset:  headerSize,
		prefetchChan: make(chan prefetchedBlock, prefetchingSize),
		bufferPool:   sync.Pool{New: func() interface{} { return new([]byte) }},
	}

	cr.wg.Add(1)
	go cr.prefetch()

	return cr, nil
}

// prefetch is the background worker that reads blocks from disk and sends them to the prefetch channel.
func (cr *PrefetchingCarReader) prefetch() {
	defer cr.wg.Done()
	defer close(cr.prefetchChan)

	for {
		c, data, sectionLen, err := cr.readNode(true)
		if err != nil {
			// Send the error and stop prefetching. EOF is the normal exit condition.
			cr.prefetchChan <- prefetchedBlock{err: err}
			return
		}

		// Send the successfully read block.
		cr.prefetchChan <- prefetchedBlock{cid: c, data: data, sectionLen: sectionLen}
	}
}

// readNode is the internal workhorse for reading the next section (CID + data).
// It's optimized to either read the data into a pooled buffer or discard it efficiently.
func (cr *PrefetchingCarReader) readNode(withData bool) (cid.Cid, []byte, uint64, error) {
	sectionLen, uvarintLen, err := ReadSectionLength(cr.br)
	if err != nil {
		return cid.Undef, nil, 0, err
	}
	totalSectionSize := sectionLen + uvarintLen

	cidLen, c, err := cid.CidFromReader(cr.br)
	if err != nil {
		return cid.Undef, nil, 0, fmt.Errorf("failed to read cid: %w", err)
	}

	dataLen := int(sectionLen - uint64(cidLen))
	if dataLen < 0 {
		return c, nil, 0, errors.New("malformed car; section length is smaller than CID length")
	}

	var data []byte
	if withData {
		// Get a buffer from the pool.
		bufPtr := cr.bufferPool.Get().(*[]byte)
		if cap(*bufPtr) < dataLen {
			*bufPtr = make([]byte, dataLen)
		}
		data = (*bufPtr)[:dataLen]

		if _, err := io.ReadFull(cr.br, data); err != nil {
			// Return the buffer to the pool on error.
			cr.bufferPool.Put(bufPtr)
			return c, nil, 0, fmt.Errorf("failed to read block data: %w", err)
		}
	} else {
		if _, err := cr.br.Discard(dataLen); err != nil {
			return c, nil, 0, fmt.Errorf("failed to discard block data: %w", err)
		}
	}

	// The global offset must be updated sequentially in this method, not in the consumer.
	cr.totalOffset += totalSectionSize
	return c, data, totalSectionSize, nil
}

// Next reads the next block from the prefetch buffer.
// IMPORTANT: The returned blocks.Block contains a byte slice from a buffer pool.
// You MUST call PutBuffer on the reader to return the buffer after you are done with the data.
func (cr *PrefetchingCarReader) Next() (blocks.Block, error) {
	block, ok := <-cr.prefetchChan
	if !ok || block.err != nil {
		return nil, block.err
	}
	return blocks.NewBlockWithCid(block.data, block.cid)
}

// NextNodeBytes reads the next block and returns its raw components from the prefetch buffer.
// IMPORTANT: The returned byte slice is from a buffer pool.
// You MUST call PutBuffer to return it once you are done processing it.
func (cr *PrefetchingCarReader) NextNodeBytes() (cid.Cid, uint64, []byte, error) {
	block, ok := <-cr.prefetchChan
	if !ok || block.err != nil {
		return cid.Undef, 0, nil, block.err
	}
	return block.cid, block.sectionLen, block.data, nil
}

// PutBuffer returns a data buffer used by a block back to the internal pool.
// This MUST be called after you are finished with the `[]byte` from NextNodeBytes or the block from Next.
func (cr *PrefetchingCarReader) PutBuffer(data []byte) {
	cr.bufferPool.Put(&data)
}

// Close gracefully shuts down the prefetching goroutine and closes the underlying reader.
func (cr *PrefetchingCarReader) Close() error {
	var err error
	cr.closeOnce.Do(func() {
		// Close the underlying reader. This will cause the prefetcher to error out and stop.
		err = cr.r.Close()
		// Wait for the prefetcher goroutine to exit cleanly.
		cr.wg.Wait()
	})
	return err
}

// HeaderSize returns the total size of the CAR header in bytes.
func (cr *PrefetchingCarReader) HeaderSize() (uint64, error) {
	return cr.headerSize, nil
}

// GetGlobalOffsetForNextRead is not supported in the prefetching reader as the offset
// is managed by the background goroutine.
func (cr *PrefetchingCarReader) GetGlobalOffsetForNextRead() (uint64, bool) {
	return 0, false
}

// readHeader remains the same as it's part of the initial setup.
func readHeader(br *bufio.Reader) (*carv1.CarHeader, uint64, error) {
	headerPayloadLen, uvarintLen, err := ReadSectionLength(br)
	if err != nil {
		return nil, 0, err
	}
	totalHeaderSize := uvarintLen + headerPayloadLen

	headerBytes := make([]byte, headerPayloadLen)
	if _, err := io.ReadFull(br, headerBytes); err != nil {
		return nil, 0, fmt.Errorf("failed to read header bytes: %w", err)
	}

	var ch carv1.CarHeader
	if err := cbor.DecodeInto(headerBytes, &ch); err != nil {
		return nil, 0, fmt.Errorf("invalid cbor in header: %w", err)
	}

	return &ch, totalHeaderSize, nil
}

// ReadSectionLength remains a utility function.
func ReadSectionLength(r *bufio.Reader) (uint64, uint64, error) {
	if _, err := r.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, 0, io.EOF
		}
		return 0, 0, err
	}

	brc := &byteReaderWithCounter{r, 0}
	l, err := binary.ReadUvarint(brc)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, err
	}

	if l > uint64(util.MaxAllowedSectionSize) {
		return 0, 0, fmt.Errorf("section size %d is larger than max allowed %d", l, util.MaxAllowedSectionSize)
	}

	return l, brc.Offset, nil
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
