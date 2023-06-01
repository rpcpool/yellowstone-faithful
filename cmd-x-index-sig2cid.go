package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_sig2cid() *cli.Command {
	var verify bool
	return &cli.Command{
		Name:        "sig-to-cid",
		Description: "Given a CAR file containing a Solana epoch, create an index of the file that maps transaction signatures to CIDs.",
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
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: "",
			},
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexDir := c.Args().Get(1)
			tmpDir := c.String("tmp-dir")

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Creating Sig-to-CID index for %s", carPath)
				indexFilepath, err := CreateIndex_sig2cid(
					context.TODO(),
					tmpDir,
					carPath,
					indexDir,
				)
				if err != nil {
					panic(err)
				}
				klog.Info("Index created")
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
			return nil
		},
	}
}
