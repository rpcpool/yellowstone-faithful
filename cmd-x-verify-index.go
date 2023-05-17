package main

import (
	"github.com/urfave/cli/v2"
)

func newVerifyIndexCmd() *cli.Command {
	return &cli.Command{
		Name:        "verify-index",
		Description: "Verify various kinds of index.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			newVerifyIndexCid2OffsetCmd(),
		},
	}
}
