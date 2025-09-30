package main

import (
	"github.com/urfave/cli/v2"
)

func newCmd_Index() *cli.Command {
	return &cli.Command{
		Name:        "index",
		Usage:       "Create various kinds of indexes for CAR files.",
		Description: "Create various kinds of indexes for CAR files.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			newCmd_Index_cid2offset(),
			newCmd_Index_slot2cid(),
			newCmd_Index_sig2cid(),
			newCmd_Index_all(),  // NOTE: not actually all (excludes gsfa)
			newCmd_Index_gsfa(), // v2
			// newCmd_Index_gsfa_v3(),
			newCmd_Index_sigExists(),
			newCmd_Index_slot2blocktime(),
		},
	}
}
