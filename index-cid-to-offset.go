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
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"k8s.io/klog/v2"
)

// CreateIndex_cid2offset creates an index file that maps CIDs to offsets in the CAR file.
func CreateIndex_cid2offset(
	ctx context.Context,
	epoch uint64,
	network indexes.Network,
	tmpDir string,
	carPath string,
	indexDir string,
) (string, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("CAR file %q does not exist", carPath)
	}

	carFile, err := os.Open(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to open car file: %w", err)
	}
	defer carFile.Close()

	rd, err := carreader.New(carFile)
	if err != nil {
		return "", fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.Header.Roots) != 1 {
		return "", fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.Header.Roots))
	}

	klog.Infof("Getting car file size")
	targetFileSize, err := getFileSize(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to get car file size: %w", err)
	}

	klog.Infof("Counting items in car file...")
	numItems, err := carCountItems(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to count items in car file: %w", err)
	}
	klog.Infof("Found %s items in car file", humanize.Comma(int64(numItems)))

	tmpDir = filepath.Join(tmpDir, "index-cid-to-offset-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	rootCid := rd.Header.Roots[0]

	klog.Infof("Creating builder with %d items and target file size %d", numItems, targetFileSize)
	c2o, err := indexes.NewWriter_CidToOffsetAndSize(
		epoch,
		rootCid,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer c2o.Close()
	totalOffset := uint64(0)
	{
		if size, err := rd.HeaderSize(); err != nil {
			return "", err
		} else {
			totalOffset += size
		}
	}
	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")
	for {
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

		totalOffset += sectionLength

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
func VerifyIndex_cid2offset(ctx context.Context, carPath string, indexFilePath string) error {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("CAR file %s does not exist", carPath)
	}

	// Check if the index file exists:
	exists, err = fileExists(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	carFile, err := os.Open(carPath)
	if err != nil {
		return fmt.Errorf("failed to open car file: %w", err)
	}
	defer carFile.Close()

	rd, err := carreader.New(carFile)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.Header.Roots) != 1 {
		return fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.Header.Roots))
	}

	c2o, err := indexes.Open_CidToOffsetAndSize(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	{
		// find root cid
		rootCID := rd.Header.Roots[0]
		offset, err := c2o.Get(rootCID)
		if err != nil {
			return fmt.Errorf("failed to get offset from index: %w", err)
		}
		cr, err := carv2.OpenReader(carPath)
		if err != nil {
			return fmt.Errorf("failed to open CAR file: %w", err)
		}
		defer cr.Close()

		dr, err := cr.DataReader()
		if err != nil {
			return fmt.Errorf("failed to open CAR data reader: %w", err)
		}
		dr.Seek(int64(offset.Offset), io.SeekStart)
		br := bufio.NewReader(dr)

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

	totalOffset := uint64(0)
	{
		if size, err := rd.HeaderSize(); err != nil {
			return err
		} else {
			totalOffset += size
		}
	}
	for {
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

		totalOffset += sectionLen
	}
	return nil
}
