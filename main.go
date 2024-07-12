package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

var gitCommitSHA = ""

func main() {
	defer klog.Flush()

	// set up a context that is canceled when a command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up a signal handler to cancel the context
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)

		select {
		case <-interrupt:
			fmt.Println()
			klog.Info("received interrupt signal")
			cancel()
		case <-ctx.Done():
		}

		// Allow any further SIGTERM or SIGINT to kill process
		signal.Stop(interrupt)
	}()

	app := &cli.App{
		Name:        "faithful CLI",
		Version:     gitCommitSHA,
		Description: "CLI to get, manage and interact with the Solana blockchain data stored in a CAR file or on Filecoin/IPFS.",
		Flags:       NewKlogFlagSet(),
		Before: func(cctx *cli.Context) error {
			return nil
		},
		Action: nil,
		Commands: []*cli.Command{
			newCmd_DumpCar(),
			fetchCmd,
			newCmd_Index(),
			newCmd_VerifyIndex(),
			newCmd_XTraverse(),
			newCmd_Version(),
			newCmd_rpc(),
			newCmd_check_deals(),
			newCmd_SplitCar(),
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.RunContext(ctx, os.Args); err != nil {
		klog.Fatal(err)
	}
}

// DummyCID is the "zero-length "identity" multihash with "raw" codec".
//
// This is the best-practices placeholder value to refer to a non-existent or unknown object.
var DummyCID = cid.MustParse("bafkqaaa")
