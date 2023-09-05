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
		ArgsUsage:   "<car-path> <index-file-path>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexFilePath := c.Args().Get(1)
			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Verifying sig-exists index for %s", carPath)
				err := VerifyIndex_sigExists(context.TODO(), carPath, indexFilePath)
				if err != nil {
					return err
				}
				klog.Info("Index verified successfully")
			}
			return nil
		},
	}
}
