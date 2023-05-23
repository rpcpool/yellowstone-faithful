package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/urfave/cli/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/ipld/ipldbindcode"
	"k8s.io/klog/v2"
)

func newCmd_Index_all() *cli.Command {
	var verify bool
	return &cli.Command{
		Name:        "all",
		Description: "Given a CAR file containing a Solana epoch, create all the necessary indexes and save them in the specified index dir.",
		ArgsUsage:   "<car-path> <index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the index after creating it",
				Destination: &verify,
			},
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			panic("not implemented")
			carPath := c.Args().Get(0)
			indexDir := c.Args().Get(1)

			_ = indexDir

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Creating all indexes for %s", carPath)
				// TODO: Create all indexes
				klog.Info("Index created")
				if verify {
				}
			}
			return nil
		},
	}
}

type IndexPaths struct{}

func createAllIndexes(ctx context.Context, carPath string, indexDir string) (*IndexPaths, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("CAR file %q does not exist", carPath)
	}

	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}

	// check it has 1 root
	roots, err := cr.Roots()
	if err != nil {
		return nil, fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return nil, fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}

	// TODO: use another way to precisely count the number of solana Blocks in the CAR file.
	klog.Infof("Counting items in car file...")
	numItems, err := carCountItemsByFirstByte(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to count items in car file: %w", err)
	}
	klog.Infof("Found %v items in car file", numItems)

	// TODO:
	// - initialize the indexes
	// - iterate over all items in the CAR file and put them into the indexes
	// - seal the indexes

	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")

	dr, err := cr.DataReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get data reader: %w", err)
	}

	// Iterate over all Transactions in the CAR file and put them into the index,
	// using the transaction signature as the key and the CID as the value.
	err = FindAny(
		ctx,
		dr,
		func(c cid.Cid, txNode any) error {
			defer func() {
				numItemsIndexed++
				if numItemsIndexed%100_000 == 0 {
					printToStderr(".")
				}
			}()
			switch txNode := txNode.(type) {
			case *ipldbindcode.Epoch:
			case *ipldbindcode.Subset:
			case *ipldbindcode.Block:
			case *ipldbindcode.Entry:
			case *ipldbindcode.Transaction:
			default:
				return fmt.Errorf("unexpected node type: %T", txNode)
			}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to index; error while iterating over blocks: %w", err)
	}

	paths := &IndexPaths{}

	rootCID := roots[0]
	klog.Infof("Root CID: %s", rootCID)

	return paths, nil
}
