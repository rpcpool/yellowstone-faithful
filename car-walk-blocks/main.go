package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/rpcpool/yellowstone-faithful/uri"
)

func main() {
	var carpath string
	flag.StringVar(&carpath, "car", "", "Path to the CAR file")
	flag.Parse()
	if carpath == "" {
		flag.Usage()
		return
	}
	slog.Info("Going to walk each block in the CAR file",
		"car", carpath,
	)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	highestSlotCar := new(atomic.Uint64)
	numBlocksReadCar := new(atomic.Uint64)
	startedAt := time.Now()

	reader, bytecounter, err := openURI(carpath)
	if err != nil {
		slog.Error("Failed to open CAR file", "error", err, "carpath", carpath)
		return
	}
	go func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Printf(
					"Walking '%s': %s blocks, Highest Slot: %s, Elapsed %s%s\r",
					carpath,
					humanize.Comma(int64(numBlocksReadCar.Load())),
					humanize.Comma(int64(highestSlotCar.Load())),
					time.Since(startedAt).Round(time.Second),
					func() string {
						if bytecounter != nil {
							return fmt.Sprintf(", Read: %s", humanize.Bytes(bytecounter.Load()))
						}
						return ""
					}(),
				)
			case <-ctx.Done():
				slog.Info("Stopping progress reporting")
				signal.Reset(os.Interrupt)
				return
			}
		}
	}()

	walker, err := NewBlockWalker(reader, func(dag *nodetools.DataAndCidSlice) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		blocks, err := dag.Blocks()
		if err != nil {
			slog.Error("Failed to get blocks from DataAndCidSlice", "error", err, "carpath", carpath)
			panic(fmt.Sprintf("Fatal: Failed to get blocks from DataAndCidSlice: %v", err))
		}
		if len(blocks) != 1 {
			slog.Error("Expected exactly one block in DataAndCidSlice", "numBlocks", len(blocks), "carpath", carpath)
			panic(fmt.Sprintf("Fatal: Expected exactly one block in DataAndCidSlice, got %d", len(blocks)))
		}
		for _, wrapper := range blocks {
			block := wrapper.Data.(*ipldbindcode.Block)
			highestSlotCar.Store(uint64(block.Slot))
			numBlocksReadCar.Add(1)
		}
		dag.SortByCid()
		parsed, err := dag.ToParsedAndCidSlice()
		if err != nil {
			panic(err)
		}
		_ = parsed
		// dag.Put() // NOTE:
		return nil
	})
	if err != nil {
		slog.Error("Failed to create BlockDAG for CAR file", "error", err, "carpath", carpath)
		panic(fmt.Sprintf("Fatal: Failed to create BlockDAG for CAR file: %v", err))
	}
	spew.Dump(walker.Header())

	if err := walker.Do(); err != nil {
		if errors.Is(err, io.EOF) {
			slog.Info("Reached end of CAR file", "carpath", carpath)
			cancel()
			return
		}
		slog.Error("Error processing CAR file", "error", err, "carpath", carpath, "numBlocksRead", numBlocksReadCar.Load(), "highestSlot", highestSlotCar.Load())
		cancel()
		return
	}
	slog.Info("Finished processing CAR file",
		"car", carpath,
		"numBlocksRead", numBlocksReadCar.Load(),
		"highestSlot", highestSlotCar.Load(),
	)
	cancel()
}

func openURI(pathOrURL string) (io.ReadCloser, *atomic.Uint64, error) {
	uri_ := uri.New(pathOrURL)
	if uri_.IsZero() || !uri_.IsValid() || (!uri_.IsFile() && !uri_.IsWeb()) {
		return nil, nil, fmt.Errorf("invalid path or URL: %s", pathOrURL)
	}
	if uri_.IsFile() {
		rc, err := os.Open(pathOrURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open file %q: %w", pathOrURL, err)
		}
		bytecounter := new(atomic.Uint64)
		countingReader := NewCountingReader(rc, bytecounter)
		return io.NopCloser(bufio.NewReaderSize(countingReader, MiB*50)), bytecounter, nil
	}
	{
		client := NewClient(nil)
		stream, err := NewResilientStream(client, pathOrURL, 3, time.Second*5)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get stream from %q: %w", pathOrURL, err)
		}
		bytecounter := new(atomic.Uint64)
		countingReader := NewCountingReader(stream, bytecounter)
		buf := bufio.NewReaderSize(countingReader, MiB*50)
		return io.NopCloser(buf), bytecounter, nil
		// return stream, nil
	}
	{
		rfspc, byteLen, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
			context.Background(),
			pathOrURL,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", pathOrURL, err)
		}
		sr := io.NewSectionReader(rfspc, 0, byteLen)
		return io.NopCloser(bufio.NewReaderSize(sr, MiB*50)), nil, nil
	}
}

type CountingReader struct {
	reader io.Reader
	count  *atomic.Uint64
}

func (cr *CountingReader) Read(p []byte) (n int, err error) {
	n, err = cr.reader.Read(p)
	if n > 0 {
		cr.count.Add(uint64(n))
	}
	return n, err
}

func (cr *CountingReader) Close() error {
	if closer, ok := cr.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func NewCountingReader(reader io.Reader, count *atomic.Uint64) *CountingReader {
	return &CountingReader{
		reader: reader,
		count:  count,
	}
}

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)

func NewBlockWalker(readCloser io.ReadCloser, callback func(*nodetools.DataAndCidSlice) error) (*nodetools.BlockDAGs, error) {
	rd, err := carreader.NewPrefetching(readCloser)
	if err != nil {
		return nil, fmt.Errorf("failed to create prefetching car reader: %w", err)
	}
	return nodetools.NewBlockDagFromReader(rd, callback), nil
}

// Client is a wrapper around http.Client for streaming large files.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new streaming client.
// If no httpClient is provided, a default one with a 30-second timeout is used.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
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
	}
	return &Client{httpClient: httpClient}
}

// GetStream makes a GET request, returns the body and the response.
func (c *Client) GetStream(url string, offset int64) (io.ReadCloser, *http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	return resp.Body, resp, nil
}

// ResilientStream is an io.ReadCloser that automatically retries on read errors.
type ResilientStream struct {
	client     *Client
	url        string
	maxRetries int
	retryDelay time.Duration

	stream io.ReadCloser
	offset int64 // Total bytes successfully read through this reader
}

// NewResilientStream creates and initializes a stream that will attempt to recover.
func NewResilientStream(c *Client, url string, retries int, delay time.Duration) (*ResilientStream, error) {
	rs := &ResilientStream{
		client:     c,
		url:        url,
		maxRetries: retries,
		retryDelay: delay,
		offset:     0,
	}

	// Make the initial connection.
	if err := rs.reconnect(); err != nil {
		return nil, fmt.Errorf("initial connection failed: %w", err)
	}
	return rs, nil
}

// reconnect handles the logic of establishing or re-establishing the stream.
func (rs *ResilientStream) reconnect() error {
	if rs.stream != nil {
		rs.stream.Close() // Close the old, broken stream.
	}

	stream, resp, err := rs.client.GetStream(rs.url, rs.offset)
	if err != nil {
		return err
	}

	// Verify the server's response.
	// This approach assumes the server correctly supports Range requests.
	if (rs.offset > 0 && resp.StatusCode != http.StatusPartialContent) || (rs.offset == 0 && resp.StatusCode != http.StatusOK) {
		stream.Close()
		return fmt.Errorf("received unexpected status: %s", resp.Status)
	}

	rs.stream = stream
	return nil
}

// Read implements the io.Reader interface. This is where the retry logic lives.
func (rs *ResilientStream) Read(p []byte) (n int, err error) {
	if rs.stream == nil {
		return 0, io.ErrClosedPipe
	}

	// Attempt the initial read.
	n, err = rs.stream.Read(p)
	rs.offset += int64(n)

	// If there's an error (and it's not a clean EOF), start the retry process.
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "\nRead error: %v. Attempting to recover...\n", err)

		for i := 0; i < rs.maxRetries; i++ {
			time.Sleep(rs.retryDelay)
			fmt.Fprintf(os.Stderr, "Retry %d/%d... ", i+1, rs.maxRetries)

			if reconErr := rs.reconnect(); reconErr != nil {
				fmt.Fprintf(os.Stderr, "reconnect failed: %v\n", reconErr)
				continue // Move to the next retry attempt.
			}

			fmt.Fprint(os.Stderr, "reconnected. Retrying read... ")
			n, err = rs.stream.Read(p) // Try reading from the new stream.
			rs.offset += int64(n)

			if err == nil {
				fmt.Fprintln(os.Stderr, "read successful.")
				return n, nil // Success, exit the retry loop and return data.
			}
		}
		// If all retries fail, return the last error.
		return n, fmt.Errorf("read failed after %d retries: %w", rs.maxRetries, err)
	}

	return n, err
}

// Close closes the underlying stream.
func (rs *ResilientStream) Close() error {
	if rs.stream == nil {
		return nil
	}
	return rs.stream.Close()
}
