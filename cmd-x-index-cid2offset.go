package main

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_cid2offset() *cli.Command {
	var verify bool
	return &cli.Command{
		Name:        "cid-to-offset",
		Description: "Given a CAR file containing a Solana epoch, create an index of the file that maps CIDs to offsets in the CAR file.",
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
				klog.Infof("Creating CID-to-offset index for %s", carPath)
				indexFilepath, err := CreateIndex_cid2offset(
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
					err := VerifyIndex_cid2offset(context.TODO(), carPath, indexFilepath)
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
