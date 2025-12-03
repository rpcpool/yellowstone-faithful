package splitcarfetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/goware/urlx"
	"github.com/rpcpool/yellowstone-faithful/metrics"
)

// NewRemoteHTTPFileAsIoReaderAt returns a ReaderAtCloser for a remote file.
// The returned ReaderAtCloser is backed by a http.Client.
func NewRemoteHTTPFileAsIoReaderAt(ctx context.Context, url string) (ReaderAtCloserSize, int64, error) {
	// 1. Validate URL before making requests
	if _, err := urlx.Parse(url); err != nil {
		return nil, 0, fmt.Errorf("failed to parse URL %q: %w", url, err)
	}

	// 2. Send a request to the server to get the file size, respecting the context
	contentLength, err := getContentSize(ctx, globalHTTPClient, url)
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
		client:        globalHTTPClient,
	}

	return rr, contentLength, nil
}

var globalHTTPClient *http.Client

func init() {
	globalHTTPClient = NewHTTPClient()
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
	if len(p) == 0 {
		return 0, nil
	}
	if off >= r.contentLength {
		return 0, io.EOF
	}
	n, err = remoteReadAt(r.client, r.url, p, off)
	if err != nil {
		return n, fmt.Errorf("failed to read remote file %q at offset %d: %w", r.url, off, err)
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

	// Range is inclusive. To read len(p) bytes starting at off, we request off to off+len(p)-1.
	end := off + int64(len(p)) - 1
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))

	var resp *http.Response
	err = retryExpotentialBackoff(
		context.Background(),
		100*time.Millisecond,
		3,
		func() error {
			resp, err = client.Do(req)
			code := "error"
			if err == nil {
				code = strconv.Itoa(resp.StatusCode)
			}
			metrics.RemoteFileHttpRequestsTotal.WithLabelValues("GET", code).Inc()

			if err != nil {
				return err
			}
			// Check for valid status codes for a Range request
			if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("unexpected status code for %q: %d", url, resp.StatusCode)
			}
			return nil
		})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	n, err = io.ReadFull(resp.Body, p)
	if err != nil {
		return n, err
	}
	return n, nil
}

// getContentSize determines the size of the remote file using HEAD or a zero-byte Range GET.
// It uses the provided context for cancellation.
func getContentSize(ctx context.Context, client *http.Client, url string) (int64, error) {
	// 1. Try HEAD
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	code := "error"
	if err == nil {
		code = strconv.Itoa(resp.StatusCode)
	}
	metrics.RemoteFileHttpRequestsTotal.WithLabelValues("HEAD", code).Inc()

	if err == nil && resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		resp.Body.Close()
		return resp.ContentLength, nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// 2. Fallback: GET with Range: bytes=0-0
	req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err = client.Do(req)
	code = "error"
	if err == nil {
		code = strconv.Itoa(resp.StatusCode)
	}
	metrics.RemoteFileHttpRequestsTotal.WithLabelValues("GET", code).Inc()

	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		// If the server ignored the range and sent 200 OK, use the full ContentLength
		if resp.StatusCode == http.StatusOK {
			return resp.ContentLength, nil
		}
		return 0, fmt.Errorf("unexpected status code during size check for %q: %d", url, resp.StatusCode)
	}

	// Parse Content-Range: bytes 0-0/1234
	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return 0, fmt.Errorf("missing Content-Range header for %q", url)
	}

	var totalSize int64
	// Format is usually "bytes <start>-<end>/<total>"
	// We only care about <total>
	parts := -1
	_, err = fmt.Sscanf(contentRange, "bytes 0-0/%d", &parts)
	if err == nil {
		totalSize = int64(parts)
	} else {
		// Try manual parsing if Sscanf is too strict
		slashIdx := 0
		for i := len(contentRange) - 1; i >= 0; i-- {
			if contentRange[i] == '/' {
				slashIdx = i
				break
			}
		}
		if slashIdx > 0 && slashIdx < len(contentRange)-1 {
			if val, err := fmt.Sscanf(contentRange[slashIdx+1:], "%d", &totalSize); err != nil || val != 1 {
				return 0, fmt.Errorf("failed to parse total size from Content-Range for %q: %q", url, contentRange)
			}
		} else {
			return 0, fmt.Errorf("invalid Content-Range format for %q: %q", url, contentRange)
		}
	}

	return totalSize, nil
}
