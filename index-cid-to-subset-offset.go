package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/indexes"
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

		tmpDir = filepath.Join(tmpDir, "index-cid-to-subset-offset-"+time.Now().Format("20060102-150405.000000000"))
		if err = os.MkdirAll(tmpDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create tmp dir: %w", err)
		}

		rootCid := rd.Header.Roots[0]

		klog.Infof("Creating builder with %d items and target file size %d", numItems, targetFileSize)
		c2so, err := indexes.NewWriter_CidToSubsetOffsetAndSize(
			epoch,
			rootCid,
			network,
			tmpDir,
			numItems,
		)
		if err != nil {
			return "", fmt.Errorf("failed to open index store: %w", err)
		}
		defer c2so.Close()
	}

}
