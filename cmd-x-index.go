package main

import (
	"github.com/urfave/cli/v2"
)

func newIndexCmd() *cli.Command {
	return &cli.Command{
		Name:        "index",
		Description: "Create various kinds of indexes for CAR files.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			newXIndexCid2OffsetCmd(),
		},
	}
}
