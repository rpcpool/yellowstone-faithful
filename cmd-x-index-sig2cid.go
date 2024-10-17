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

func newCmd_Index_sig2cid() *cli.Command {
	var verify bool
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "sig-to-cid",
		Description: "Given one or more CAR files containing a Solana epoch, create an index of the file that maps transaction signatures to CIDs. If multiple CAR files are provided, each of them is expected to correspond to a single Subset.",
		ArgsUsage:   "<car-path>... <index-dir>",
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
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			args := c.Args()
			if args.Len() < 2 {
				return fmt.Errorf("at least one CAR file and an index directory are required")
			}

			carFiles := args.Slice()[:args.Len()-1]
			indexDir := args.Get(args.Len() - 1)
			tmpDir := c.String("tmp-dir")

			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			// Sort CAR files
			sortedCarFiles, err := SortCarFiles(carFiles)
			if err != nil {
				return fmt.Errorf("failed to sort CAR files: %w", err)
			}

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()

				for _, carPath := range sortedCarFiles {
					klog.Infof("Creating Sig-to-CID index for %s", carPath)
					indexFilepath, err := CreateIndex_sig2cid(
						context.TODO(),
						epoch,
						network,
						tmpDir,
						carPath,
						indexDir,
					)
					if err != nil {
						panic(err)
					}
					klog.Info("Index created for %s", carPath)
					if verify {
						klog.Infof("Verifying index for %s located at %s", carPath, indexFilepath)
						startedAt := time.Now()
						defer func() {
							klog.Infof("Finished in %s", time.Since(startedAt))
						}()
						err := VerifyIndex_sig2cid(context.TODO(), carPath, indexFilepath)
						if err != nil {
							return cli.Exit(err, 1)
						}
						klog.Info("Index verified")
						return nil
					}
				}
			}
			klog.Info("Index created for all CAR files")
			return nil
		},
	}
}
