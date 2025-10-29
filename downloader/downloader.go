package downloader

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Downloader constants
const (
	defaultChunkSize   = 1024 * 1024 * 4 // 4 MB
	defaultConcurrency = 10
	maxRetries         = 5
	maxInMemoryChunks  = 20 // Controls memory usage: maxInMemoryChunks * chunkSize
	baseBackoff        = 1 * time.Second
)

// downloadedChunk holds the data from a completed download job.
type downloadedChunk struct {
	index int
	data  []byte
	err   error
}

// chunkJob defines a byte range for a download worker.
type chunkJob struct {
	index int
	start int64
	end   int64
}

// Downloader manages the concurrent download.
type Downloader struct {
	httpClient  *http.Client
	url         string
	fileSize    int64
	chunkSize   int64
	concurrency int
	client      *http.Client
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	jobs        chan chunkJob
	results     chan downloadedChunk
	errs        chan error
}

// DownloaderReader implements io.ReadCloser for the downloader.
type DownloaderReader struct {
	d          *Downloader
	pipeReader *io.PipeReader
}

// SetHTTPClient allows setting a custom HTTP client.
func (r *Downloader) SetHTTPClient(client *http.Client) {
	r.httpClient = client
}

func NewDownloader(url string, concurrency int, chunkSize int64) (*Downloader, error) {
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	client := &http.Client{
		// do dual stack, try http2 first, then http1
		Transport: &http.Transport{
			ForceAttemptHTTP2:     true,
			DisableKeepAlives:     false,
			IdleConnTimeout:       30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			DisableCompression:    false,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-200 status code: %s", resp.Status)
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, fmt.Errorf("server does not support range requests")
	}
	fileSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Content-Length header: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Downloader{
		httpClient:  client,
		url:         url,
		fileSize:    fileSize,
		chunkSize:   chunkSize,
		concurrency: concurrency,
		client:      client,
		ctx:         ctx,
		cancel:      cancel,
		jobs:        make(chan chunkJob),
		results:     make(chan downloadedChunk, maxInMemoryChunks),
		errs:        make(chan error, 1),
	}, nil
}

func (d *Downloader) Download() (io.ReadCloser, error) {
	log.Printf("Starting download for %s", d.url)
	log.Printf("File size: %s", formatBytes(d.fileSize))
	log.Printf("Chunk size: %s", formatBytes(d.chunkSize))
	log.Printf("Concurrency: %d workers", d.concurrency)

	pipeReader, pipeWriter := io.Pipe()

	d.wg.Add(2) // For generateJobs and reorder
	go d.generateJobs()
	go d.reorder(pipeWriter)

	// Start worker pool
	var workerWg sync.WaitGroup
	for i := 0; i < d.concurrency; i++ {
		workerWg.Add(1)
		go d.worker(&workerWg, i+1)
	}
	go func() {
		workerWg.Wait()
		close(d.results)
	}()

	return &DownloaderReader{d: d, pipeReader: pipeReader}, nil
}

func (d *Downloader) generateJobs() {
	defer d.wg.Done()
	defer close(d.jobs)
	for offset := int64(0); offset < d.fileSize; offset += d.chunkSize {
		end := offset + d.chunkSize - 1
		if end >= d.fileSize {
			end = d.fileSize - 1
		}
		select {
		case d.jobs <- chunkJob{index: int(offset / d.chunkSize), start: offset, end: end}:
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Downloader) worker(wg *sync.WaitGroup, id int) {
	defer wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case job, ok := <-d.jobs:
			if !ok {
				return
			}
			data, err := d.downloadChunk(job)
			select {
			case d.results <- downloadedChunk{index: job.index, data: data, err: err}:
			case <-d.ctx.Done():
				return
			}
		}
	}
}

func (d *Downloader) downloadChunk(job chunkJob) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseBackoff * time.Duration(math.Pow(2, float64(attempt-1)))
			select {
			case <-time.After(delay):
			case <-d.ctx.Done():
				return nil, d.ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(d.ctx, "GET", d.url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", job.start, job.end))

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			lastErr = fmt.Errorf("unexpected status code: %s", resp.Status)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (d *Downloader) reorder(pipeWriter *io.PipeWriter) {
	defer d.wg.Done()
	defer pipeWriter.Close()

	buffer := make(map[int]downloadedChunk)
	nextChunkIndex := 0
	totalChunks := int((d.fileSize + d.chunkSize - 1) / d.chunkSize)

	for receivedCount := 0; receivedCount < totalChunks; {
		select {
		case result, ok := <-d.results:
			if !ok {
				d.reportError(fmt.Errorf("download incomplete: results channel closed prematurely"))
				return
			}
			if result.err != nil {
				d.reportError(result.err)
				return
			}
			buffer[result.index] = result
			receivedCount++
		case <-d.ctx.Done():
			return
		}

		for {
			chunk, ok := buffer[nextChunkIndex]
			if !ok {
				break
			}
			if _, err := pipeWriter.Write(chunk.data); err != nil {
				d.reportError(err)
				return
			}
			delete(buffer, nextChunkIndex)
			nextChunkIndex++
		}
	}
}

func (d *Downloader) reportError(err error) {
	select {
	case d.errs <- err:
		d.cancel()
	default:
	}
}

func (r *DownloaderReader) Read(p []byte) (n int, err error) {
	return r.pipeReader.Read(p)
}

func (r *DownloaderReader) Close() error {
	r.d.cancel()
	r.d.wg.Wait()
	return r.pipeReader.Close()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
