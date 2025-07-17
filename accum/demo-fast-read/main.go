package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/gsfa-v3/3.3/tooling"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

func main() {
	var carPath string
	var offsetParent uint64
	var offsetChild uint64
	var sizeChild uint64
	flag.Uint64Var(&offsetParent, "offset-parent", 0, "Offset of the parent object")
	flag.Uint64Var(&offsetChild, "offset-child", 0, "Offset of the child object")
	flag.Uint64Var(&sizeChild, "size-child", 0, "Size of the child object")
	flag.StringVar(&carPath, "car", "", "Path to the CAR file")
	flag.Parse()

	if carPath == "" {
		panic("Please provide a CAR file path using the -car flag")
	}
	if offsetParent == 0 || offsetChild == 0 || sizeChild == 0 {
		panic("Please provide valid offsets and size using the -offset-parent, -offset-child, and -size-child flags")
	}
	carFile, err := os.Open(carPath)
	if err != nil {
		panic(err)
	}
	defer carFile.Close()
	// parent must come before child in the CAR file
	if offsetParent >= offsetChild {
		panic("Parent offset must be less than child offset")
	}

	totalSize := offsetChild + sizeChild - offsetParent
	if totalSize <= 0 {
		panic("Invalid offsets or size: total size must be greater than zero")
	}
	slog.Info("Reading CAR file",
		"carPath", carPath,
		"offsetParent", offsetParent,
		"offsetChild", offsetChild,
		"sizeChild", sizeChild,
		"totalSize", totalSize,
		"totalSizeHuman", humanize.Bytes(totalSize),
	)
	fmt.Printf("Reading %d bytes from CAR file starting at offset %d\n", totalSize, offsetParent)
	startedReadAt := time.Now()
	buffer := make([]byte, totalSize)
	_, err = carFile.ReadAt(buffer, int64(offsetParent))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Read %d bytes in %s\n", len(buffer), time.Since(startedReadAt))

	startSplitAt := time.Now()
	{
		numSections := 0
		cloned := clone(buffer)
		node, remaining, err := ParseNodeFromSection(cloned, nil)
		if err != nil {
			panic(err)
		}
		numSections++
		{
			parsed, err := iplddecoders.DecodeAny(node)
			if err != nil {
				panic(err)
			}
			// spew.Dump(parsed)
			_ = parsed
		}
		for len(remaining) > 0 {
			node, remaining, err = ParseNodeFromSection(remaining, nil)
			if err != nil {
				panic(err)
			}
			numSections++
			{
				parsed, err := iplddecoders.DecodeAny(node)
				if err != nil {
					panic(err)
				}
				_ = parsed
				// spew.Dump(parsed)
			}
		}
		fmt.Printf("Parsed %d sections in %s\n", numSections, time.Since(startSplitAt))
	}
	startSplitAt = time.Now()
	nodes, err := tooling.SplitIntoDataAndCids(buffer)
	if err != nil {
		panic(err)
	}
	nodes.SortByCid()
	fmt.Printf("Split into %d nodes in %s\n", len(nodes), time.Since(startSplitAt))
	{
		startParseEachAt := time.Now()
		err := nodes.ReadEachConcurrent(func(d *tooling.DataAndCid) error {
			parsed, err := iplddecoders.DecodeAny(d.Data.Bytes())
			if err != nil {
				return fmt.Errorf("failed to decode node with CID %s: %w", d.Cid, err)
			}
			// spew.Dump(parsed)
			_ = parsed
			return nil
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("Parsed each node concurrently in %s\n", time.Since(startParseEachAt))
	}
	{
		startParseEachAt := time.Now()
		parsedNodes := make([]ipldbindcode.Node, len(nodes))
		i := 0
		err := nodes.ReadEach(func(d *tooling.DataAndCid) error {
			parsed, err := iplddecoders.DecodeAny(d.Data.Bytes())
			if err != nil {
				return fmt.Errorf("failed to decode node with CID %s: %w", d.Cid, err)
			}
			// spew.Dump(parsed)
			_ = parsed
			parsedNodes[i] = parsed
			i++
			return nil
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("Parsed each node serially in %s\n", time.Since(startParseEachAt))
	}
	{
		startConversionAt := time.Now()
		parsedNodes, err := nodes.ToParsedAndCidSlice()
		if err != nil {
			panic(fmt.Errorf("failed to convert nodes to parsed nodes: %w", err))
		}
		fmt.Printf("Converted nodes to parsed nodes in %s\n", time.Since(startConversionAt))
		_ = parsedNodes // Use parsedNodes to avoid unused variable error
		parsedNodes.SortByCid()
		{
			// Epoch
			// Subset
			// Block
			// Rewards
			// Entry
			// Transaction
			// DataFrame
			{
				startedAt := time.Now()
				for v := range parsedNodes.Epoch() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Epoch nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Subset() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Subset nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Block() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Block nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Rewards() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Rewards nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Entry() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Entry nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Transaction() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Transaction nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.DataFrame() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over DataFrame nodes in %s\n", time.Since(startedAt))
			}
			{
				startedAt := time.Now()
				for v := range parsedNodes.Any() {
					_ = v // Use v to avoid unused variable error
				}
				fmt.Printf("Iterated over Any nodes in %s\n", time.Since(startedAt))
			}
		}
		parsedNodes.Put() // Recycle the parsed nodes
	}
	nodes.Put()
}

func clone[T any](s []T) []T {
	v := make([]T, len(s))
	copy(v, s)
	return v
}

func ParseNodeFromSection(section []byte, wantedCid *cid.Cid) ([]byte, []byte, error) {
	// read an uvarint from the buffer
	gotLen, usize := binary.Uvarint(section)
	if usize <= 0 {
		return nil, nil, fmt.Errorf("failed to decode uvarint")
	}
	if gotLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	data := section[usize:]
	cidLen, gotCid, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read cid: %w", err)
	}
	// verify that the CID we read matches the one we expected.
	if wantedCid != nil && !gotCid.Equals(*wantedCid) {
		return nil, nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	dataStart := usize + cidLen
	dataEnd := int(gotLen) + usize
	actualData := section[dataStart:dataEnd]
	// slog.Info(
	// 	"ParseNodeFromSection",
	// 	"gotLen", gotLen,
	// 	"usize", usize,
	// 	"cidLen", cidLen,
	// 	"dataStart", dataStart,
	// 	"dataEnd", dataEnd,
	// 	"dataLen", len(actualData),
	// 	"len(section)", len(section),
	// )
	if len(actualData) != dataEnd-dataStart {
		return nil, nil, fmt.Errorf("data length mismatch: expected %d, got %d", dataEnd-dataStart, len(actualData))
	}
	remainingSection := section[dataEnd:]
	return actualData, remainingSection, nil
}
