package main

import (
	"github.com/urfave/cli/v2"
)

func newCmd_Index() *cli.Command {
	return &cli.Command{
		Name:        "index",
		Description: "Create various kinds of indexes for CAR files.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			newCmd_Index_Cid2Offset(),
		},
	}
}
