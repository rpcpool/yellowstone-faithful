package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_VerifyIndex_sigExists() *cli.Command {
	return &cli.Command{
		Name:        "sig-exists",
		Description: "Verify the index that tells whether a signature exists in it",
		ArgsUsage:   "--index-dir=<index-dir> --car=<car-path>",
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
				Usage: "Directory to store the index",
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
				klog.Infof("Verifying sig-exists index for %v", carPaths)
				err := VerifyIndex_sigExists(context.TODO(), carPaths, indexFilePath)
				if err != nil {
					return err
				}
				klog.Info("Index verified successfully")
			}
			return nil
		},
	}
}
