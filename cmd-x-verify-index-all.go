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
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.StringFlag{
				Name:  "index-cid-to-offset",
				Usage: "Path to the CID-to-offset index file",
			},
			&cli.StringFlag{
				Name:  "index-slot-to-cid",
				Usage: "Path to the slot-to-CID index file",
			},
			&cli.StringFlag{
				Name:  "index-sig-to-cid",
				Usage: "Path to the signature-to-CID index file",
			},
			&cli.StringFlag{
				Name:  "index-sig-exists",
				Usage: "Path to the signature-exists index file",
			},
		},
		Action: func(c *cli.Context) error {
			carPaths := c.StringSlice("car")
			indexFilePathCid2Offset := c.String("index-cid-to-offset")
			indexFilePathSlot2Cid := c.String("index-slot-to-cid")
			indexFilePathSig2Cid := c.String("index-sig-to-cid")
			indexFilePathSigExists := c.String("index-sig-exists")

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Verifying indexes for %v", carPaths)
				err := verifyAllIndexes(
					context.TODO(),
					carPaths,
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
