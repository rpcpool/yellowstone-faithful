package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"k8s.io/klog/v2"
)

// CreateIndex_slot2cid creates an index file that maps slot numbers to CIDs.
func CreateIndex_slot2cid(
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

	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to open CAR file: %w", err)
	}

	// check it has 1 root
	roots, err := cr.Roots()
	if err != nil {
		return "", fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return "", fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}
	rootCid := roots[0]

	// TODO: use another way to precisely count the number of solana Blocks in the CAR file.
	klog.Infof("Counting items in car file...")
	numItems, err := carCountItems(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to count items in car file: %w", err)
	}
	klog.Infof("Found %s items in car file", humanize.Comma(int64(numItems)))

	tmpDir = filepath.Join(tmpDir, "index-slot-to-cid-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	klog.Infof("Creating builder with %d items", numItems)
	sl2c, err := indexes.NewWriter_SlotToCid(
		epoch,
		rootCid,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer sl2c.Close()

	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")

	dr, err := cr.DataReader()
	if err != nil {
		return "", fmt.Errorf("failed to get data reader: %w", err)
	}

	// Iterate over all blocks in the CAR file and put them into the index,
	// using the slot number as the key and the CID as the value.
	err = FindBlocks(
		ctx,
		dr,
		func(c cid.Cid, block *ipldbindcode.Block) error {
			slotNum := block.Slot

			err = sl2c.Put(uint64(slotNum), c)
			if err != nil {
				return fmt.Errorf("failed to put cid to offset: %w", err)
			}

			numItemsIndexed++
			if numItemsIndexed%1_000 == 0 {
				printToStderr(".")
			}
			return nil
		})
	if err != nil {
		return "", fmt.Errorf("failed to index; error while iterating over blocks: %w", err)
	}

	// Use the car file name and root CID to name the index file:

	klog.Infof("Sealing index...")
	if err = sl2c.Seal(ctx, indexDir); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	indexFilePath := sl2c.GetFilepath()
	klog.Infof("Index created at %s; %d items indexed", indexFilePath, numItemsIndexed)
	return indexFilePath, nil
}

// VerifyIndex_slot2cid verifies that the index file is correct for the given car file.
// It does this by reading the car file and comparing the offsets in the index
// file to the offsets in the car file.
func VerifyIndex_slot2cid(ctx context.Context, carPath string, indexFilePath string) error {
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

	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return fmt.Errorf("failed to open CAR file: %w", err)
	}

	// check it has 1 root
	roots, err := cr.Roots()
	if err != nil {
		return fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}

	c2o, err := indexes.Open_SlotToCid(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}

	dr, err := cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}

	numItems := uint64(0)
	// Iterate over all blocks in the CAR file and put them into the index,
	// using the slot number as the key and the CID as the value.
	err = FindBlocks(
		ctx,
		dr,
		func(c cid.Cid, block *ipldbindcode.Block) error {
			slotNum := uint64(block.Slot)

			got, err := c2o.Get(slotNum)
			if err != nil {
				return fmt.Errorf("failed to put cid to offset: %w", err)
			}

			if !got.Equals(c) {
				return fmt.Errorf("slot %d: expected cid %s, got %s", slotNum, c, got)
			}

			numItems++
			if numItems%1_000 == 0 {
				printToStderr(".")
			}

			return nil
		})
	if err != nil {
		return fmt.Errorf("failed to verify index; error while iterating over blocks: %w", err)
	}
	return nil
}
