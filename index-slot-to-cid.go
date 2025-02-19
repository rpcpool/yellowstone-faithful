package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	"k8s.io/klog/v2"
)

// CreateIndex_slot2cid creates an index file that maps slot numbers to CIDs.
func CreateIndex_slot2cid(
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

	tmpDir = filepath.Join(tmpDir, "index-slot-to-cid-"+time.Now().Format("20060102-150405.000000000"))
	if err = os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	numItems := uint64(slottools.EpochLen)

	klog.Infof("Creating builder with %d items", numItems)
	sl2c, err := indexes.NewWriter_SlotToCid(
		epoch,
		rootCID,
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

	// Iterate over all blocks in the CAR file and put them into the index,
	// using the slot number as the key and the CID as the value.
	err = FindBlocksFromReader(
		ctx,
		rd,
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
func VerifyIndex_slot2cid(ctx context.Context, carPaths []string, indexFilePath string) error {
	err := allFilesExist(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}

	rd, err := readasonecar.NewMultiReader(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	defer rd.Close()

	rootCID, err := rd.FindRoot()
	if err != nil {
		return fmt.Errorf("failed to find root CID: %w", err)
	}
	klog.Infof("Root CID: %s", rootCID)

	// Check if the index file exists:
	exists, err := fileExists(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	c2o, err := indexes.Open_SlotToCid(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}

	numItems := uint64(0)
	// Iterate over all blocks in the CAR file and put them into the index,
	// using the slot number as the key and the CID as the value.
	err = FindBlocksFromReader(
		ctx,
		rd,
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
