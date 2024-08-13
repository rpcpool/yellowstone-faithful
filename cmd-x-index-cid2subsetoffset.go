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

func newCmd_Index_cid2subsetOffset() *cli.Command {
	var verify bool
	var epoch uint64
	var network indexes.Network
	var indexDir string
	return &cli.Command{
		Name:        "cid-to-offset",
		Description: "Given all split CAR files corresponding to a Solana epoch, create an index of the file that maps CIDs to offsets in the CAR file.",
		ArgsUsage:   "<car-paths> <index-dir>",
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
				Usage:       "the epoch of the CAR files",
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
			&cli.StringFlag{
				Name:        "index-dir",
				Usage:       "directory to store the index",
				Destination: &indexDir,
				Required:    true,
			},
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			carPaths := c.Args().Slice()
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
				klog.Infof("Creating CID-to-offset index")
				indexFilepath, err := CreateIndex_cid2subsetOffset(
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
				// if verify {
				// 	klog.Infof("Verifying index located at %s", indexFilepath)
				// 	startedAt := time.Now()
				// 	defer func() {
				// 		klog.Infof("Finished in %s", time.Since(startedAt))
				// 	}()
				// 	err := VerifyIndex_cid2subsetOffset(context.TODO(), indexFilepath)
				// 	if err != nil {
				// 		return cli.Exit(err, 1)
				// 	}
				// 	klog.Info("Index verified")
				// 	return nil
				// }
			}
			return nil
		},
	}
}
