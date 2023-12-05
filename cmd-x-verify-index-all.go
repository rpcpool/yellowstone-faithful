package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_VerifyIndex_all() *cli.Command {
	return &cli.Command{
		Name:        "all",
		Description: "Verify all indexes.",
		ArgsUsage:   "<car-path> <index-cid-to-offset> <index-slot-to-cid> <index-sig-to-cid> <index-sig-exists>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexFilePathCid2Offset := c.Args().Get(1)
			indexFilePathSlot2Cid := c.Args().Get(2)
			indexFilePathSig2Cid := c.Args().Get(3)
			indexFilePathSigExists := c.Args().Get(4)

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Verifying Slot-to-CID index for %s", carPath)
				err := verifyAllIndexes(
					context.TODO(),
					carPath,
					&IndexPaths{
						CidToOffsetAndSize: indexFilePathCid2Offset,
						SlotToCid:          indexFilePathSlot2Cid,
						SignatureToCid:     indexFilePathSig2Cid,
						SignatureExists:    indexFilePathSigExists,
					},
					0,
				)
				if err != nil {
					return err
				}
				klog.Info("Index verified successfully")
			}
			return nil
		},
	}
}
