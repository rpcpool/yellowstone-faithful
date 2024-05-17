package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

func isDirEmpty(dir string) (bool, error) {
	file, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = file.Readdir(1)
	if errors.Is(err, io.EOF) {
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

	rd, err := carreader.New(file)
	if err != nil {
		return 0, fmt.Errorf("failed to open car file: %w", err)
	}

	var count uint64
	for {
		_, _, err := rd.NextInfo()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

func carCountItemsByFirstByte(carPath string) (map[byte]uint64, *ipldbindcode.Epoch, error) {
	file, err := os.Open(carPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	rd, err := carreader.New(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open car file: %w", err)
	}

	numTotalItems := uint64(0)
	counts := make(map[byte]uint64)
	startedCountAt := time.Now()
	var epochObject *ipldbindcode.Epoch
	for {
		_, _, block, err := rd.NextNodeBytes()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, err
		}
		// the first data byte is the block type (after the CBOR tag)
		firstDataByte := block[1]
		counts[firstDataByte]++
		numTotalItems++

		if numTotalItems%1_000_000 == 0 {
			printToStderr(
				fmt.Sprintf("\rCounted %s items", humanize.Comma(int64(numTotalItems))),
			)
		}

		if iplddecoders.Kind(firstDataByte) == iplddecoders.KindEpoch {
			epochObject, err = iplddecoders.DecodeEpoch(block)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to decode Epoch node: %w", err)
			}
		}
	}

	printToStderr(
		fmt.Sprintf("\rCounted %s items in %s\n", humanize.Comma(int64(numTotalItems)), time.Since(startedCountAt).Truncate(time.Second)),
	)

	return counts, epochObject, err
}

func printToStderr(msg string) {
	fmt.Fprint(os.Stderr, msg)
}
