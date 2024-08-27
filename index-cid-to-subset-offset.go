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
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"k8s.io/klog/v2"
)

func CreateIndex_cid2subsetOffset(
	ctx context.Context,
	epoch uint64,
	network indexes.Network,
	tmpDir string,
	carPaths []string,
	indexDir string,
) (string, error) {
	var numItems uint64
	var orderedCids []cid.Cid
	carPathMap := make(map[cid.Cid]string)
	for _, carPath := range carPaths {
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

		carPathMap[rd.Header.Roots[0]] = carPath

		klog.Infof("Counting items in car file: %s", carPath)
		ni, epochObject, err := carCountItemsAndFindEpoch(carPath)
		if err != nil {
			return "", fmt.Errorf("failed to count items in CAR: %w", err)
		}
		if epochObject != nil {
			for _, subset := range epochObject.Subsets {
				orderedCids = append(orderedCids, subset.(cidlink.Link).Cid)
			}
		}
		klog.Infof("Found %s items in car file", humanize.Comma(int64(ni)))
		numItems += ni
	}

	klog.Infof("Found a total of %d items in car files", numItems)

	tmpDir = filepath.Join(tmpDir, "index-cid-to-subset-offset-"+time.Now().Format("20060102-150405.000000000"))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	klog.Infof("Creating builder with %d items", numItems)
	c2so, err := indexes.NewWriter_CidToSubsetOffsetAndSize(
		epoch,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer c2so.Close()

	// To do: how to get the subset index?
	subset := uint64(0)
	numItemsIndexed := uint64(0)
	for _, cid := range orderedCids {
		carPath := carPathMap[cid]
		carFile, err := os.Open(carPath)
		if err != nil {
			return "", fmt.Errorf("failed to open car file: %w", err)
		}
		defer carFile.Close()

		rd, err := carreader.New(carFile)
		if err != nil {
			return "", fmt.Errorf("failed to create car reader: %w", err)
		}

		totalOffset := uint64(0)
		{
			if size, err := rd.HeaderSize(); err != nil {
				return "", fmt.Errorf("failed to get car header size: %w", err)
			} else {
				totalOffset += size
			}
		}
		for {
			c, sectionLength, err := rd.NextInfo()
			if err != nil {
				if errors.Is(err, io.EOF) {
					subset++
					break
				}
				return "", fmt.Errorf("encountered an error while indexing: %w", err)
			}

			err = c2so.Put(c, subset, totalOffset, sectionLength)
			if err != nil {
				return "", fmt.Errorf("failed to put cid to subset, offset: %w", err)
			}

			totalOffset += sectionLength

			numItemsIndexed++
			if numItemsIndexed%100_000 == 0 {
				printToStderr(".")
			}
		}

	}

	klog.Infof("Sealing index...")
	if err = c2so.Seal(ctx, indexDir); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}

	indexFilePath := c2so.GetFilePath()
	klog.Infof("Index created at %s, %d items indexed", indexFilePath, numItemsIndexed)
	return indexFilePath, nil
}

func VerifyIndex_cid2subsetOffset(ctx context.Context, carPaths []string, indexFilePath string) error {
	// Check if the index file exists:
	exists, err := fileExists(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	c2so, err := indexes.Open_CidToSubsetOffsetAndSize(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}

	startedAt := time.Now()
	numItems := 0
	defer func() {
		klog.Infof("Finished in %s", time.Since(startedAt))
		klog.Infof("Read %d nodes", numItems)
	}()

	for _, carPath := range carPaths {
		// Check if the CAR file exists:
		exists, err := fileExists(carPath)
		if err != nil {
			return fmt.Errorf("failed to check if CAR file exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("CAR file %s does not exist", carPath)
		}

		carFile, err := os.Open(carPath)
		if err != nil {
			return fmt.Errorf("failed to open car file: %w", err)
		}

		rd, err := carreader.New(carFile)
		if err != nil {
			return fmt.Errorf("failed to create car reader: %w", err)
		}
		// check it has 1 root
		if len(rd.Header.Roots) != 1 {
			return fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.Header.Roots))
		}

		{
			// find root cid
			rootCID := rd.Header.Roots[0]
			subsetAndOffset, err := c2so.Get(rootCID)
			if err != nil {
				return fmt.Errorf("failed to get subset and offset from index: %w", err)
			}
			cr, err := carv2.OpenReader(carPath)
			if err != nil {
				return fmt.Errorf("failed to open CAR file: %w", err)
			}

			dr, err := cr.DataReader()
			if err != nil {
				return fmt.Errorf("failed to open CAR data reader: %w", err)
			}
			dr.Seek(int64(subsetAndOffset.Offset), io.SeekStart)
			br := bufio.NewReader(dr)

			gotCid, data, err := util.ReadNode(br)
			if err != nil {
				return err
			}
			// verify that the CID we read matches the one we expected.
			if !gotCid.Equals(rootCID) {
				return fmt.Errorf("CID mismatch: expected %s, got %s", rootCID, gotCid)
			}
			// try parsing the data as a Subset node.
			decoded, err := iplddecoders.DecodeSubset(data)
			if err != nil {
				return fmt.Errorf("failed to decode root node: %w", err)
			}
			spew.Dump(decoded)
			cr.Close()
		}

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
			offset, err := c2so.Get(c)
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
		carFile.Close()

	}
	return nil
}
