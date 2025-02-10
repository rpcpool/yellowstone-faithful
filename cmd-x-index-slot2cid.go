package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_slot2cid() *cli.Command {
	var verify bool
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "slot-to-cid",
		Description: "Given a CAR file containing a Solana epoch, create an index of the file that maps slot numbers to CIDs.",
		ArgsUsage:   "--index-dir=<index-dir> --car=<car-path>",
		Before: func(c *cli.Context) error {
			if network == "" {
				network = indexes.NetworkMainnet
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the index after creating it",
				Destination: &verify,
			},
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: os.TempDir(),
			},
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
			tmpDir := c.String("tmp-dir")

			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Creating Slot-to-CID index for %v", carPaths)
				indexFilepath, err := CreateIndex_slot2cid(
					context.TODO(),
					epoch,
					network,
					tmpDir,
					carPaths,
					indexDir,
				)
				if err != nil {
					panic(err)
				}
				klog.Info("Index created")
				if verify {
					klog.Infof("Verifying index for %s located at %v", carPaths, indexFilepath)
					startedAt := time.Now()
					defer func() {
						klog.Infof("Finished in %s", time.Since(startedAt))
					}()
					err := VerifyIndex_slot2cid(context.TODO(), carPaths, indexFilepath)
					if err != nil {
						return cli.Exit(err, 1)
					}
					klog.Info("Index verified")
					return nil
				}
			}
			return nil
		},
	}
}
