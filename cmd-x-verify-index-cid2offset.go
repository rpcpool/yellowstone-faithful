package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_VerifyIndex_cid2offset() *cli.Command {
	return &cli.Command{
		Name:        "cid-to-offset",
		Description: "Verify the index of the CAR file that maps CIDs to offsets in the CAR file.",
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
				klog.Infof("Verifying CID-to-offset index for %v", carPaths)
				err := VerifyIndex_cid2offset(context.TODO(), carPaths, indexFilePath)
				if err != nil {
					return err
				}
				klog.Info("Index verified successfully")
			}
			return nil
		},
	}
}
