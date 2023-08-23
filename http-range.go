package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
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

// remoteHTTPFileAsIoReaderAt returns a ReaderAtCloser for a remote file.
// The returned ReaderAtCloser is backed by a http.Client.
func remoteHTTPFileAsIoReaderAt(ctx context.Context, url string) (ReaderAtCloser, error) {
	// send a request to the server to get the file size:
	contentLength, err := getContentSizeWithHeadOrZeroRange(url)
	if err != nil {
		return nil, err
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length/Content-Range header, or file is empty")
	}

	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	rr := &HTTPSingleFileRemoteReaderAt{
		url:           url,
		contentLength: contentLength,
		client:        newHTTPClient(),
	}

	rc := NewRangeCache(contentLength, func(p []byte, off int64) (n int, err error) {
		return remoteReadAt(rr.client, rr.url, p, off)
	})
	rc.StartCacheGC(ctx, 1*time.Minute)
	rr.ca = rc

	return rr, nil
}

type HTTPSingleFileRemoteReaderAt struct {
	url           string
	contentLength int64
	client        *http.Client
	ca            *RangeCache
}

// Close implements io.Closer.
func (r *HTTPSingleFileRemoteReaderAt) Close() error {
	r.client.CloseIdleConnections()
	return r.ca.Close()
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
	v, err := r.ca.GetRange(context.Background(), off, int64(len(p)))
	if err != nil {
		return 0, err
	}
	n = copy(p, v)
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

type readCloserWrapper struct {
	rac  ReaderAtCloser
	name string
}

// when reading print a dot
func (r *readCloserWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	startedAt := time.Now()
	defer func() {
		if DebugMode {
			prefix := "[READ-UNKNOWN]"
			// if has suffix .index, then it's an index file
			if strings.HasSuffix(r.name, ".index") {
				prefix = azureBG("[READ-INDEX]")
			}
			// if has suffix .car, then it's a car file
			if strings.HasSuffix(r.name, ".car") {
				prefix = purpleBG("[READ-CAR]")
			}
			fmt.Fprintf(os.Stderr, prefix+" %s:%d-%d (%s)\n", filepath.Base(r.name), off, len(p), time.Since(startedAt))
		}
	}()
	return r.rac.ReadAt(p, off)
}

func purpleBG(s string) string {
	// blue bg, black fg
	return "\033[48;5;4m\033[38;5;0m" + s + "\033[0m"
}

func azureBG(s string) string {
	// azure bg, black fg
	return "\033[48;5;6m\033[38;5;0m" + s + "\033[0m"
}

// when closing print a newline
func (r *readCloserWrapper) Close() error {
	return r.rac.Close()
}
