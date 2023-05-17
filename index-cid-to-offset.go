package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ipfs/go-cid"
	carv1 "github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"k8s.io/klog/v2"
)

// CreateIndex_cid2offset creates an index file that maps CIDs to offsets in the CAR file.
func CreateIndex_cid2offset(ctx context.Context, carPath string, indexDir string) (string, error) {
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

	rd, err := newCarReader(carFile)
	if err != nil {
		return "", fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.header.Roots) != 1 {
		return "", fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.header.Roots))
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
	klog.Infof("Found %d items in car file", numItems)

	klog.Infof("Creating builder with %d items and target file size %d", numItems, targetFileSize)
	c2o, err := compactindex.NewBuilder(
		"",
		uint(numItems),
		(targetFileSize),
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer c2o.Close()
	totalOffset := uint64(0)
	{
		var buf bytes.Buffer
		if err = carv1.WriteHeader(rd.header, &buf); err != nil {
			return "", err
		}
		totalOffset = uint64(buf.Len())
	}
	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")
	for {
		c, sectionLength, err := rd.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		err = c2o.Insert(c.Bytes(), uint64(totalOffset))
		if err != nil {
			return "", fmt.Errorf("failed to put cid to offset: %w", err)
		}

		totalOffset += sectionLength

		numItemsIndexed++
		if numItemsIndexed%100_000 == 0 {
			printToStderr(".")
		}
	}

	rootCID := rd.header.Roots[0]

	// Use the car file name and root CID to name the index file:
	indexFilePath := filepath.Join(indexDir, fmt.Sprintf("%s.%s.cid-to-offset.index", filepath.Base(carPath), rootCID.String()))

	klog.Infof("Creating index file at %s", indexFilePath)
	targetFile, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create index file: %w", err)
	}
	defer targetFile.Close()

	klog.Infof("Sealing index...")
	if err = c2o.Seal(ctx, targetFile); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	klog.Infof("Index created; %d items indexed", numItemsIndexed)
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

	rd, err := newCarReader(carFile)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.header.Roots) != 1 {
		return fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.header.Roots))
	}

	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer indexFile.Close()

	c2o, err := compactindex.Open(indexFile)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	{
		// find root cid
		rootCID := rd.header.Roots[0]
		offset, err := findOffsetFromCid(c2o, rootCID)
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
		dr.Seek(int64(offset), io.SeekStart)
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
		var buf bytes.Buffer
		if err = carv1.WriteHeader(rd.header, &buf); err != nil {
			return err
		}
		totalOffset = uint64(buf.Len())
	}
	for {
		c, sectionLen, err := rd.Next()
		if errors.Is(err, io.EOF) {
			klog.Infof("EOF")
			break
		}
		numItems++
		if numItems%100000 == 0 {
			printToStderr(".")
		}
		offset, err := findOffsetFromCid(c2o, c)
		if err != nil {
			return fmt.Errorf("failed to lookup offset for %s: %w", c, err)
		}
		if offset != totalOffset {
			return fmt.Errorf("offset mismatch for %s: %d != %d", c, offset, totalOffset)
		}

		totalOffset += sectionLen
	}
	return nil
}

func findOffsetFromCid(db *compactindex.DB, c cid.Cid) (uint64, error) {
	bucket, err := db.LookupBucket(c.Bytes())
	if err != nil {
		return 0, fmt.Errorf("failed to lookup bucket for %s: %w", c, err)
	}
	offset, err := bucket.Lookup(c.Bytes())
	if err != nil {
		return 0, fmt.Errorf("failed to lookup offset for %s: %w", c, err)
	}
	return offset, nil
}
