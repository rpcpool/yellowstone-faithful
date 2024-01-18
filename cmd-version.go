package main

import (
	"fmt"
	"runtime/debug"

	"github.com/urfave/cli/v2"
)

func newCmd_Version() *cli.Command {
	return &cli.Command{
		Name:        "version",
		Usage:       "Print version information of this binary.",
		Description: "Print version information of this binary.",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			fmt.Println("YELLOWSTONE FAITHFUL CLI")
			fmt.Printf("Tag/Branch: %s\n", GitTag)
			fmt.Printf("Commit: %s\n", GitCommit)
			if info, ok := debug.ReadBuildInfo(); ok {
				fmt.Printf("More info:\n")
				for _, setting := range info.Settings {
					if isAnyOf(setting.Key,
						"-compiler",
						"GOARCH",
						"GOOS",
						"GOAMD64",
						"vcs",
						"vcs.revision",
						"vcs.time",
						"vcs.modified",
					) {
						fmt.Printf("  %s: %s\n", setting.Key, setting.Value)
					}
				}
			}
			return nil
		},
	}
}

var (
	GitCommit string
	GitTag    string
)

func isAnyOf(s string, anyOf ...string) bool {
	for _, v := range anyOf {
		if s == v {
			return true
		}
	}
	return false
}
