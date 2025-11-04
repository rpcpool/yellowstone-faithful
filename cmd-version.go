package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"time"

	"github.com/google/uuid"
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
			printVersion()
			return nil
		},
	}
}

func printVersion() {
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
	fmt.Println("Date: ", time.Now().Format(time.RFC3339))
	fmt.Println("Go version:", runtime.Version())
	fmt.Println("Num CPU:", runtime.NumCPU())
}

func printVersionAsJson() {
	info := map[string]string{
		"tag":        GitTag,
		"commit":     GitCommit,
		"date":       time.Now().Format(time.RFC3339),
		"go_version": runtime.Version(),
		"num_cpu":    fmt.Sprintf("%d", runtime.NumCPU()),
		"session_id": SessionID,
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
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
				info[setting.Key] = setting.Value
			}
		}
	}
	asJson, err := json.Marshal(info)
	if err != nil {
		panic(fmt.Errorf("error while marshaling version info to JSON: %w", err))
	}
	fmt.Println(":FAITHFUL_VERSION_BEGIN:" + string(asJson) + ":FAITHFUL_VERSION_END:")
}

var (
	GitCommit string
	GitTag    string
	SessionID string
)

func init() {
	SessionID = uuid.New().String() + ":" + time.Now().Format("20060102T150405")
}

func GetSessionID() string {
	return SessionID
}

func isAnyOf(s string, anyOf ...string) bool {
	return slices.Contains(anyOf, s)
}
