package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/carreader"
)

func main() {
	var carPath string
	flag.StringVar(&carPath, "car", "", "Path to the CAR file")
	flag.Parse()

	fmt.Println("Reading CAR file:", carPath)

	file, err := os.Open(carPath)
	if err != nil {
		panic(fmt.Errorf("failed to open file %q: %w", carPath, err))
	}
	defer file.Close()
	reader, err := carreader.NewPrefetching(file)
	if err != nil {
		panic(fmt.Errorf("failed to create car reader for file %q: %w", carPath, err))
	}
	size, err := sizeOfFile(carPath)
	if err != nil {
		panic(fmt.Errorf("failed to get size of file %q: %w", carPath, err))
	}
	startedAt := time.Now()
	defer func() {
		fmt.Printf("Finished reading CAR file in %s\n", time.Since(startedAt))
		fmt.Printf("Average read speed: %s/s\n", humanize.Bytes(uint64(float64(size)/time.Since(startedAt).Seconds())))
	}()
	numRead := 0
	lastCheckpoint := time.Now()
	for {
		numRead++
		cid, offset, data, err := reader.NextNodeBytes()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break // No more objects to read
			}
			panic(fmt.Errorf("error reading next object: %w", err))
		}
		if numRead%1_000_000 == 0 {
			fmt.Printf("Read %d objects in %s\n", numRead, time.Since(lastCheckpoint))
			lastCheckpoint = time.Now()
		}
		if numRead%100_000_000 == 0 {
			fmt.Printf("=== Read %d objects in %s ===\n", numRead, time.Since(startedAt))
		}

		_ = cid                // Use cid if needed
		_ = offset             // Use offset if needed
		reader.PutBuffer(data) // Return the buffer to the pool
	}
}

func sizeOfFile(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info for %q: %w", path, err)
	}
	return info.Size(), nil
}
