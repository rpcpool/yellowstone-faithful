package main

import (
	"context"
	"io/fs"
	"os"

	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_SplitCar() *cli.Command {
	return &cli.Command{
		Name:        "split-car",
		Description: "Splits an epoch car file into smaller chunks. Each chunk corresponds to a subset.",
		ArgsUsage:   "<epoch-car-path>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "size",
				Aliases:  []string{"s"},
				Value:    31 * 1024 * 1024 * 1024, // 31 GiB
				Usage:    "Target size in bytes to chunk CARs to.",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			carPath := c.Args().First()
			var file fs.File
			var err error
			if carPath == "-" {
				file = os.Stdin
			} else {
				file, err = os.Open(carPath)
				if err != nil {
					klog.Exit(err.Error())
				}
				defer file.Close()
			}

			rd, err := carreader.New(file)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			{
				// print roots:
				roots := rd.Header.Roots
				klog.Infof("Roots: %d", len(roots))
				for i, root := range roots {
					if i == 0 && len(roots) == 1 {
						klog.Infof("- %s (Epoch CID)", root.String())
					} else {
						klog.Infof("- %s", root.String())
					}
				}
			}

			maxSize := c.Int("size")
			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(owm1 *accum.ObjectWithMetadata, owm2 []accum.ObjectWithMetadata) error {
					size := 0

					return nil
				},
				iplddecoders.KindEpoch,
				iplddecoders.KindSubset,
			)

			if err := accum.Run((context.Background())); err != nil {
				klog.Exitf("error while accumulating objects: %w", err)
			}

			return nil
		},
	}
}
