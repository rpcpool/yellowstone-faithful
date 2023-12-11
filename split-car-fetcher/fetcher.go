package splitcarfetcher

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"

	"github.com/anjor/carlet"
)

type SplitCarReader struct {
	files       *carlet.CarPiecesAndMetadata
	multireader io.ReaderAt
	closers     []io.Closer
}

type ReaderAtCloserSize interface {
	io.ReaderAt
	io.Closer
	Size() int64
}

type SplitCarFileReaderCreator func(carFile carlet.CarFile) (ReaderAtCloserSize, error)

type FileSplitCarReader struct {
	filepath string
	file     *os.File
	size     int64
}

func NewFileSplitCarReader(filepath string) (*FileSplitCarReader, error) {
	fi, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %s", filepath, err)
	}
	stat, err := fi.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %q: %s", filepath, err)
	}
	size := stat.Size()
	return &FileSplitCarReader{
		filepath: filepath,
		file:     fi,
		size:     size,
	}, nil
}

func (fscr *FileSplitCarReader) ReadAt(p []byte, off int64) (n int, err error) {
	return fscr.file.ReadAt(p, off)
}

func (fscr *FileSplitCarReader) Close() error {
	return fscr.file.Close()
}

func (fscr *FileSplitCarReader) Size() int64 {
	return fscr.size
}

func getContentSizeWithHeadOrZeroRange(url string) (int64, error) {
	// try sending a HEAD request to the server to get the file size:
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		// try sending a GET request with a zero range to the server to get the file size:
		req := &http.Request{
			Method: "GET",
			URL:    resp.Request.URL,
			Header: make(http.Header),
		}
		req.Header.Set("Range", "bytes=0-0")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusPartialContent {
			return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		// now find the content length:
		contentRange := resp.Header.Get("Content-Range")
		if contentRange == "" {
			return 0, fmt.Errorf("missing Content-Range header")
		}
		var contentLength int64
		_, err := fmt.Sscanf(contentRange, "bytes 0-0/%d", &contentLength)
		if err != nil {
			return 0, err
		}
		return contentLength, nil
	}
	return resp.ContentLength, nil
}

type RemoteFileSplitCarReader struct {
	commP      string
	url        string
	size       int64
	httpClient *http.Client
}

func NewRemoteFileSplitCarReader(commP string, url string) (*RemoteFileSplitCarReader, error) {
	size, err := getContentSizeWithHeadOrZeroRange(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get content size from %q: %s", url, err)
	}
	return &RemoteFileSplitCarReader{
		commP:      commP,
		url:        url,
		size:       size,
		httpClient: http.DefaultClient,
	}, nil
}

func (fscr *RemoteFileSplitCarReader) ReadAt(p []byte, off int64) (n int, err error) {
	req, err := http.NewRequest("GET", fscr.url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))-1))
	{
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Keep-Alive", "timeout=600")
	}
	resp, err := fscr.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("GET %q: unexpected status code: %d", fscr.url, resp.StatusCode)
	}
	n, err = io.ReadFull(resp.Body, p)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (fscr *RemoteFileSplitCarReader) Close() error {
	fscr.httpClient.CloseIdleConnections()
	return nil
}

func (fscr *RemoteFileSplitCarReader) Size() int64 {
	return fscr.size
}

func NewSplitCarReader(
	files *carlet.CarPiecesAndMetadata,
	readerCreator SplitCarFileReaderCreator,
) (*SplitCarReader, error) {
	scr := &SplitCarReader{
		files:   files,
		closers: make([]io.Closer, 0),
	}
	readers := make([]io.ReaderAt, 0)
	sizes := make([]int64, 0)

	{
		// add the original car header
		originalCarHeaderReaderAt, originalCarHeaderSize, err := scr.getOriginalCarHeaderReaderAt()
		if err != nil {
			return nil, fmt.Errorf("failed to get original car header reader: %s", err)
		}
		readers = append(readers, originalCarHeaderReaderAt)
		sizes = append(sizes, int64(originalCarHeaderSize))
	}
	for _, cf := range files.CarPieces {
		fi, err := readerCreator(cf)
		if err != nil {
			return nil, fmt.Errorf("failed to open remote file %q: %s", cf.CommP, err)
		}

		size := int(fi.Size())

		// if local file, check the size:
		if _, ok := fi.(*FileSplitCarReader); ok {
			expectedSize := int(cf.HeaderSize) + int(cf.ContentSize) // NOTE: valid only for pre-upload split CARs. They get padded after upload.
			if size != expectedSize {
				return nil, fmt.Errorf(
					"remote file %q has unexpected size: saved=%d actual=%d (diff=%d)",
					cf.Name,
					expectedSize,
					size,
					expectedSize-size,
				)
			}
		}

		// if remote, then the file must be at least as header size + content size:
		if _, ok := fi.(*RemoteFileSplitCarReader); ok {
			expectedMinSize := int(cf.HeaderSize) + int(cf.ContentSize)
			if size < expectedMinSize {
				return nil, fmt.Errorf(
					"remote file %q has unexpected size: expected min size=%d actual=%d (diff=%d)",
					cf.CommP.String(),
					expectedMinSize,
					size,
					expectedMinSize-size,
				)
			}
		}

		scr.closers = append(scr.closers, fi)
		sectionReader := io.NewSectionReader(fi, int64(cf.HeaderSize), int64(cf.ContentSize))

		readers = append(readers, sectionReader)
		sizes = append(sizes, int64(cf.ContentSize))
	}
	scr.multireader = NewMultiReaderAt(readers, sizes)
	return scr, nil
}

func (scr *SplitCarReader) Close() error {
	for _, closer := range scr.closers {
		closer.Close()
	}
	return nil
}

func (scr *SplitCarReader) ReadAt(p []byte, off int64) (n int, err error) {
	return scr.multireader.ReadAt(p, off)
}

func (scr *SplitCarReader) getOriginalCarHeaderReaderAt() (io.ReaderAt, int, error) {
	originalWholeCarHeader, originalWholeCarHeaderSize, err := scr.originalCarHeader()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get original car header: %s", err)
	}
	originalWholeCarHeaderReader := bytes.NewReader(originalWholeCarHeader)
	return originalWholeCarHeaderReader, int(originalWholeCarHeaderSize), nil
}

func (scr *SplitCarReader) originalCarHeader() ([]byte, int64, error) {
	accu := int64(0)

	// now add the size of the actual header
	headerBytes, err := base64.StdEncoding.DecodeString(scr.files.OriginalCarHeader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode original car header: %s", err)
	}
	headerSizePrefix := make([]byte, 0)
	headerSizePrefix = binary.AppendUvarint(headerSizePrefix, uint64(len(headerBytes)))
	accu += int64(len(headerSizePrefix))

	totalSize := int(len(headerBytes)) + int(len(headerSizePrefix))
	if totalSize != int(scr.files.OriginalCarHeaderSize) {
		return nil, 0, fmt.Errorf("unexpected header size: saved=%d actual=%d", scr.files.OriginalCarHeaderSize, totalSize)
	}
	accu += int64(len(headerBytes))
	totalHeader := make([]byte, 0)
	totalHeader = append(totalHeader, headerSizePrefix...)
	totalHeader = append(totalHeader, headerBytes...)
	return totalHeader, accu, nil
}

type MultiReaderAt struct {
	readers []io.ReaderAt
	offsets []int64
}

func NewMultiReaderAt(readers []io.ReaderAt, sizes []int64) *MultiReaderAt {
	offsets := make([]int64, len(sizes))
	var total int64 = 0
	for i, size := range sizes {
		offsets[i] = total
		total += size
	}
	return &MultiReaderAt{
		readers: readers,
		offsets: offsets,
	}
}

func (m *MultiReaderAt) ReadAt(p []byte, off int64) (totalN int, err error) {
	remaining := len(p)
	bufOffset := 0
	reachedEnd := false

	for i, offset := range m.offsets {
		if off < offset {
			continue
		}

		nextOffset := int64(math.MaxInt64)
		if i < len(m.offsets)-1 {
			nextOffset = m.offsets[i+1]
		}

		toRead := int(min(max(0, nextOffset-off), int64(remaining)))

		n, err := m.readers[i].ReadAt(p[bufOffset:bufOffset+toRead], off-offset)
		totalN += n
		bufOffset += n
		remaining -= n

		if err != nil {
			if err == io.EOF && i == len(m.readers)-1 {
				reachedEnd = true
			} else if err != io.EOF {
				return totalN, err
			}
		}

		if n == toRead {
			off += int64(n)
		}

		if remaining == 0 {
			break
		}
	}

	if remaining > 0 && reachedEnd {
		return totalN, io.EOF
	}

	return totalN, nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func getFileSize(path string) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := fileInfo.Size()
	return int(fileSize)
}
