package main

import (
	"github.com/urfave/cli/v2"
)

func newCmd_VerifyIndex() *cli.Command {
	return &cli.Command{
		Name:        "verify-index",
		Description: "Verify various kinds of index.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			newCmd_VerifyIndex_cid2offset(),
			newCmd_VerifyIndex_slot2cid(),
			newCmd_VerifyIndex_sig2cid(),
			newCmd_VerifyIndex_sigExists(),
			newCmd_VerifyIndex_all(),
		},
	}
}
