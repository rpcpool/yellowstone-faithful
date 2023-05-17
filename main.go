package main

import (
	"log"
	"os"
	"sort"

	"github.com/urfave/cli/v2"
)

var gitCommitSHA = ""

func main() {
	app := &cli.App{
		Name:        "faithful CLI",
		Version:     gitCommitSHA,
		Description: "CLI to get, manage and interact with the Solana blockchain data stored in a CAR file or on Filecoin/IPFS.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags:  []cli.Flag{},
		Action: nil,
		Commands: []*cli.Command{
			newCmd_DumpCar(),
			newCmd_Fetch(),
			newCmd_Index(),
			newCmd_VerifyIndex(),
			newCmd_XTraverse(),
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
