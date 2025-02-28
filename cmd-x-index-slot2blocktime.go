package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/blocktimeindex"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_slot2blocktime() *cli.Command {
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "slot-to-blocktime",
		Description: "Given a CAR file containing a Solana epoch, create an index of the file that maps slots to blocktimes.",
		ArgsUsage:   "--index-dir=<index-dir> --car=<car-path>",
		Before: func(c *cli.Context) error {
			if network == "" {
				network = indexes.NetworkMainnet
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:        "epoch",
				Usage:       "the epoch of the CAR file",
				Destination: &epoch,
				Required:    true,
			},
			&cli.StringFlag{
				Name:  "network",
				Usage: "the cluster of the epoch; one of: mainnet, testnet, devnet",
				Action: func(c *cli.Context, s string) error {
					network = indexes.Network(s)
					if !indexes.IsValidNetwork(network) {
						return fmt.Errorf("invalid network: %q", network)
					}
					return nil
				},
			},
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.StringFlag{
				Name:  "index-dir",
				Usage: "Destination directory for the output files",
			},
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			carPaths := c.StringSlice("car")
			indexDir := c.String("index-dir")

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Creating slot-to-blocktime index for %v", carPaths)
				indexFilepath, err := CreateIndex_slot2blocktime(
					context.TODO(),
					epoch,
					network,
					carPaths,
					indexDir,
				)
				if err != nil {
					panic(err)
				}
				klog.Info("slot-to-blocktime index created at", indexFilepath)
			}
			return nil
		},
	}
}

// CreateIndex_slot2blocktime creates an index file that maps slot numbers to blocktimes.
func CreateIndex_slot2blocktime(
	ctx context.Context,
	epoch uint64,
	network indexes.Network,
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

	slot_to_blocktime := blocktimeindex.NewForEpoch(epoch)

	numBlocksIndexed := uint64(0)
	klog.Infof("Indexing...")

	// Iterate over all blocks in the CAR file and put them into the index,
	// using the slot number as the key and the blocktime as the value.
	err = FindBlocksFromReader(
		ctx,
		rd,
		func(c cid.Cid, block *ipldbindcode.Block) error {
			slotNum := uint64(block.Slot)

			err = slot_to_blocktime.Set(slotNum, int64(block.Meta.Blocktime))
			if err != nil {
				return fmt.Errorf("failed to put cid to offset: %w", err)
			}

			numBlocksIndexed++
			if numBlocksIndexed%1_000 == 0 {
				printToStderr(".")
			}
			return nil
		})
	if err != nil {
		return "", fmt.Errorf("failed to index; error while iterating over blocks: %w", err)
	}

	// Use the car file name and root CID to name the index file:

	klog.Infof("Sealing index...")

	indexFilePath := filepath.Join(indexDir, blocktimeindex.FormatFilename(epoch, rootCID, network))

	file, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create slot_to_blocktime index file: %w", err)
	}
	defer file.Close()

	if _, err := slot_to_blocktime.WriteTo(file); err != nil {
		return "", fmt.Errorf("failed to write slot_to_blocktime index: %w", err)
	}
	klog.Infof("Successfully sealed slot_to_blocktime index")
	klog.Infof("Index created at %s; %d items indexed", indexFilePath, numBlocksIndexed)
	return indexFilePath, nil
}
