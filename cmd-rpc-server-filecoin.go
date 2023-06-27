package main

import (
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/urfave/cli/v2"
)

func newCmd_rpcServerFilecoin() *cli.Command {
	var listenOn string
	return &cli.Command{
		Name:        "rpc-server-filecoin",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<slot-to-cid-index-filepath-or-url> <sig-to-cid-index-filepath-or-url> <gsfa-index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Listen address",
				Value:       ":8899",
				Destination: &listenOn,
			},
		},
		Action: func(c *cli.Context) error {
			slotToCidIndexFilepath := c.Args().Get(0)
			if slotToCidIndexFilepath == "" {
				return cli.Exit("Must provide a slot-to-CID index filepath/url", 1)
			}
			sigToCidIndexFilepath := c.Args().Get(1)
			if sigToCidIndexFilepath == "" {
				return cli.Exit("Must provide a signature-to-CID index filepath/url", 1)
			}

			slotToCidIndexFile, err := openIndexStorage(slotToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer slotToCidIndexFile.Close()

			slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			sigToCidIndexFile, err := openIndexStorage(sigToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer sigToCidIndexFile.Close()

			sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			ls, err := newLassieWrapper(c)
			if err != nil {
				return fmt.Errorf("newLassieWrapper: %w", err)
			}

			var gsfaIndex *gsfa.GsfaReader
			gsfaIndexDir := c.Args().Get(4)
			if gsfaIndexDir != "" {
				gsfaIndex, err = gsfa.NewGsfaReader(gsfaIndexDir)
				if err != nil {
					return fmt.Errorf("failed to open gsfa index: %w", err)
				}
				defer gsfaIndex.Close()
			}

			return createAndStartRPCServer_lassie(
				c.Context,
				listenOn,
				ls,
				slotToCidIndex,
				sigToCidIndex,
				gsfaIndex,
			)
		},
	}
}
