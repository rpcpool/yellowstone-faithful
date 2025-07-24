package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	highestSlotCar := new(atomic.Uint64)
	numBlocksReadCar := new(atomic.Uint64)
	go func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Printf("Blocks read from CAR file '%s': %d blocks, Highest Slot: %d\r", carpath, numBlocksReadCar.Load(), highestSlotCar.Load())
			case <-ctx.Done():
				slog.Info("Stopping progress reporting")
				return
			}
		}
	}()
	reader, err := openURI(carpath)
	if err != nil {
		slog.Error("Failed to open CAR file", "error", err, "carpath", carpath)
		return
	}
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

func openURI(pathOrURL string) (io.ReadCloser, error) {
	uri_ := uri.New(pathOrURL)
	if uri_.IsZero() || !uri_.IsValid() || (!uri_.IsFile() && !uri_.IsWeb()) {
		return nil, fmt.Errorf("invalid path or URL: %s", pathOrURL)
	}
	if uri_.IsFile() {
		return os.Open(pathOrURL)
	}
	{
		rfspc, byteLen, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
			context.Background(),
			pathOrURL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", pathOrURL, err)
		}
		sr := io.NewSectionReader(rfspc, 0, byteLen)
		return io.NopCloser(bufio.NewReaderSize(sr, MiB*50)), nil
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
