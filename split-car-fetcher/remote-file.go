package splitcarfetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/goware/urlx"
)

// NewRemoteHTTPFileAsIoReaderAt returns a ReaderAtCloser for a remote file.
// The returned ReaderAtCloser is backed by a http.Client.
func NewRemoteHTTPFileAsIoReaderAt(ctx context.Context, url string) (ReaderAtCloserSize, int64, error) {
	// send a request to the server to get the file size:
	contentLength, err := GetContentSizeWithHeadOrZeroRange(url)
	if err != nil {
		return nil, 0, err
	}
	if contentLength == 0 {
		return nil, 0, fmt.Errorf("missing Content-Length/Content-Range header, or file is empty")
	}

	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	rr := &HTTPSingleFileRemoteReaderAt{
		url:           url,
		contentLength: contentLength,
		client:        NewHTTPClient(),
	}
	_, err = urlx.Parse(url)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse URL %q: %w", url, err)
	}

	return rr, contentLength, nil
}

type HTTPSingleFileRemoteReaderAt struct {
	url           string
	contentLength int64
	client        *http.Client
}

// Close implements io.Closer.
func (r *HTTPSingleFileRemoteReaderAt) Close() error {
	r.client.CloseIdleConnections()
	return nil
}

// Size returns the size of the file.
func (r *HTTPSingleFileRemoteReaderAt) Size() int64 {
	return r.contentLength
}

func retryExpotentialBackoff(
	ctx context.Context,
	startDuration time.Duration,
	maxRetries int,
	fn func() error,
) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(startDuration):
			startDuration *= 2
		}
	}
	return fmt.Errorf("failed after %d retries; last error: %w", maxRetries, err)
}

func (r *HTTPSingleFileRemoteReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= r.contentLength {
		return 0, io.EOF
	}
	n, err = remoteReadAt(r.client, r.url, p, off)
	if err != nil {
		return n, fmt.Errorf("failed to read remote file at offset %d: %w", off, err)
	}
	if n == 0 {
		return 0, io.EOF
	}
	if n < 0 {
		return 0, fmt.Errorf("negative read count: %d", n)
	}
	if n < len(p) {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

func remoteReadAt(client *http.Client, url string, p []byte, off int64) (n int, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	{
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Keep-Alive", "timeout=600")
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))))

	var resp *http.Response
	err = retryExpotentialBackoff(
		context.Background(),
		100*time.Millisecond,
		3,
		func() error {
			resp, err = client.Do(req)
			return err
		})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	{
		n, err := io.ReadFull(resp.Body, p)
		if err != nil {
			return 0, err
		}
		return n, nil
	}
}
