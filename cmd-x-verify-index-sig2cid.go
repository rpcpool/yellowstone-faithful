package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_VerifyIndex_sig2cid() *cli.Command {
	return &cli.Command{
		Name:        "sig-to-cid",
		Description: "Verify the index of the CAR file that maps transaction signatures to CIDs.",
		ArgsUsage:   "--car=<car-path> --index=<index-file>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.StringFlag{
				Name:  "index-dir",
				Usage: "Destination directory for the output files",
			},
		},
		Action: func(c *cli.Context) error {
			carPaths := c.StringSlice("car")
			indexFilePath := c.String("index-dir")
			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Verifying Sig-to-CID index for %v", carPaths)
				err := VerifyIndex_sig2cid(context.TODO(), carPaths, indexFilePath)
				if err != nil {
					return err
				}
				klog.Info("Index verified successfully")
			}
			return nil
		},
	}
}
