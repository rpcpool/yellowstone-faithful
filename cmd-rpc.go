package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/ryanuber/go-glob"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_rpc() *cli.Command {
	var listenOn string
	var gsfaOnlySignatures bool
	return &cli.Command{
		Name:        "rpc",
		Description: "Provide multiple epoch config files, and start a Solana JSON RPC that exposes getTransaction, getBlock, and (optionally) getSignaturesForAddress",
		ArgsUsage:   "<one or more config files or directories containing config files (nested is fine)>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: append(lassieFetchFlags,
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Listen address",
				Value:       ":8899",
				Destination: &listenOn,
			},
			&cli.BoolFlag{
				Name:        "gsfa-only-signatures",
				Usage:       "gSFA: only return signatures",
				Value:       false,
				Destination: &gsfaOnlySignatures,
			},
		),
		Action: func(c *cli.Context) error {
			src := c.Args().Slice()
			configFiles, err := getListOfConfigFiles(src)
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			klog.Infof("Found %d config files", len(configFiles))
			spew.Dump(configFiles)

			// Load configs:
			configs := make(ConfigSlice, 0)
			for _, configFile := range configFiles {
				config, err := loadConfig(configFile)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to load config file %q: %s", configFile, err.Error()), 1)
				}
				configs = append(configs, config)
			}
			// Validate configs:
			if err := configs.Validate(); err != nil {
				return cli.Exit(fmt.Sprintf("error validating configs: %s", err.Error()), 1)
			}
			configs.SortByEpoch()
			klog.Infof("Loaded %d epoch configs", len(configs))
			klog.Info("Initializing epochs...")

			epochs := make([]*Epoch, 0)
			for _, config := range configs {
				epoch, err := NewEpochFromConfig(config, c)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to create epoch from config %q: %s", config.ConfigFilepath(), err.Error()), 1)
				}
				epochs = append(epochs, epoch)
			}

			multi := NewMultiEpoch(&Options{
				GsfaOnlySignatures: gsfaOnlySignatures,
			})

			for _, epoch := range epochs {
				if err := multi.AddEpoch(epoch.Epoch(), epoch); err != nil {
					return cli.Exit(fmt.Sprintf("failed to add epoch %d: %s", epoch.Epoch(), err.Error()), 1)
				}
			}

			return multi.ListenAndServe(listenOn)
		},
	}
}

func getListOfConfigFiles(src []string) ([]string, error) {
	var allFiles []string

	for _, srcItem := range src {
		isDir, err := isDirectory(srcItem)
		if err != nil {
			return nil, err
		}
		if isDir {
			files, err := getFilesFromDirectory(srcItem)
			if err != nil {
				return nil, err
			}
			allFiles = append(allFiles, files...)
		} else {
			allFiles = append(allFiles, srcItem)
		}
	}

	allFiles = selectMatching(allFiles, "*.yaml", "*.yml", "*.json")

	// deduplicate
	seen := make(map[string]struct{})
	var deduped []string
	for _, file := range allFiles {
		if _, ok := seen[file]; !ok {
			seen[file] = struct{}{}
			deduped = append(deduped, file)
		}
	}
	allFiles = deduped

	return allFiles, nil
}

func getFilesFromDirectory(dir string) ([]string, error) {
	ok, err := exists(dir)
	if err != nil {
		return nil, fmt.Errorf("error checking if path %q exists: %w", dir, err)
	}
	if !ok {
		return nil, fmt.Errorf("path %q does not exist", dir)
	}

	isDir, err := isDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("error checking if path %q is a directory: %w", dir, err)
	}
	if !isDir {
		return nil, fmt.Errorf("path %q is not a directory", dir)
	}

	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %q: %w", dir, err)
	}

	var files []string
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			files = append(files, filepath.Join(dir, fileInfo.Name()))
		}
	}

	return files, nil
}

func selectMatching(items []string, patterns ...string) []string {
	var matching []string
	for _, item := range items {
		matches := itemMatchesAnyPattern(item, patterns...)
		if matches {
			matching = append(matching, item)
		}
	}
	return matching
}

func itemMatchesAnyPattern(item string, patterns ...string) bool {
	for _, pattern := range patterns {
		matches := glob.Glob(pattern, item)
		if matches {
			return true
		}
	}
	return false
}
