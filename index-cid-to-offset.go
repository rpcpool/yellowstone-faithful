package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/ipld/go-car/util"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"k8s.io/klog/v2"
)

// CreateIndex_cid2offset creates an index file that maps CIDs to offsets in the CAR file.
func CreateIndex_cid2offset(
	ctx context.Context,
	epoch uint64,
	network indexes.Network,
	tmpDir string,
	carPaths []string,
	indexDir string,
) (string, error) {
	err := allFilesExist(carPaths...)
	if err != nil {
		return "", fmt.Errorf("failed to check if CAR file exists: %w", err)
	}

	rd, err := readasonecar.NewMultiReader(carPaths...)
	if err != nil {
		return "", fmt.Errorf("failed to create car reader: %w", err)
	}
	defer rd.Close()

	rootCID, err := rd.FindRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find root CID: %w", err)
	}
	klog.Infof("Root CID: %s", rootCID)

	klog.Infof("Counting items in car file...")
	numItems, err := carCountItems(carPaths...)
	if err != nil {
		return "", fmt.Errorf("failed to count items in car file: %w", err)
	}
	klog.Infof("Found %s items in car file", humanize.Comma(int64(numItems)))

	tmpDir = filepath.Join(tmpDir, "index-cid-to-offset-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	klog.Infof("Creating builder with %d items", numItems)
	c2o, err := indexes.NewWriter_CidToOffsetAndSize(
		epoch,
		rootCID,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer c2o.Close()

	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")
	for {
		totalOffset, ok := rd.GetGlobalOffsetForNextRead()
		if !ok {
			break
		}
		c, sectionLength, err := rd.NextInfo()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		err = c2o.Put(c, totalOffset, sectionLength)
		if err != nil {
			return "", fmt.Errorf("failed to put cid to offset: %w", err)
		}

		numItemsIndexed++
		if numItemsIndexed%100_000 == 0 {
			printToStderr(".")
		}
	}

	klog.Infof("Sealing index...")
	if err = c2o.Seal(ctx, indexDir); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	indexFilePath := c2o.GetFilepath()
	klog.Infof("Index created at %s; %d items indexed", indexFilePath, numItemsIndexed)
	return indexFilePath, nil
}

// VerifyIndex_cid2offset verifies that the index file is correct for the given car file.
// It does this by reading the car file and comparing the offsets in the index
// file to the offsets in the car file.
func VerifyIndex_cid2offset(ctx context.Context, carPaths []string, indexFilePath string) error {
	// Check if the index file exists:
	exists, err := fileExists(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	c2o, err := indexes.Open_CidToOffsetAndSize(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	err = allFilesExist(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}

	rd, err := readasonecar.NewMultiReader(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	defer rd.Close()
	{

		rootCID, err := rd.FindRoot()
		if err != nil {
			return fmt.Errorf("failed to find root CID: %w", err)
		}
		klog.Infof("Root CID: %s", rootCID)

		offset, err := c2o.Get(rootCID)
		if err != nil {
			return fmt.Errorf("failed to get offset from index: %w", err)
		}

		br := bufio.NewReader(io.NewSectionReader(rd, int64(offset.Offset), int64(offset.Size)))

		gotCid, data, err := util.ReadNode(br)
		if err != nil {
			return err
		}
		// verify that the CID we read matches the one we expected.
		if !gotCid.Equals(rootCID) {
			return fmt.Errorf("CID mismatch: expected %s, got %s", rootCID, gotCid)
		}
		// try parsing the data as an Epoch node.
		decoded, err := iplddecoders.DecodeEpoch(data)
		if err != nil {
			return fmt.Errorf("failed to decode root node: %w", err)
		}
		spew.Dump(decoded)
	}

	startedAt := time.Now()
	numItems := 0
	defer func() {
		klog.Infof("Finished in %s", time.Since(startedAt))
		klog.Infof("Read %d nodes", numItems)
	}()

	for {
		totalOffset, ok := rd.GetGlobalOffsetForNextRead()
		if !ok {
			break
		}
		c, sectionLen, err := rd.NextInfo()
		if errors.Is(err, io.EOF) {
			klog.Infof("EOF")
			break
		}
		numItems++
		if numItems%100000 == 0 {
			printToStderr(".")
		}
		offset, err := c2o.Get(c)
		if err != nil {
			return fmt.Errorf("failed to lookup offset for %s: %w", c, err)
		}
		if offset.Offset != totalOffset {
			return fmt.Errorf("offset mismatch for %s: %d != %d", c, offset, totalOffset)
		}
		if offset.Size != sectionLen {
			return fmt.Errorf("length mismatch for %s: %d != %d", c, offset, sectionLen)
		}
	}
	return nil
}
