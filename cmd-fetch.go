package main

import (
	"github.com/urfave/cli/v2"
)

func newFetchCmd() *cli.Command {
	return &cli.Command{
		Name:        "fetch",
		Description: "Fetch Solana data from Filecoin/IPFS",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			return nil
		},
	}
}
