package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func newCmd_rpcServerFilecoin() *cli.Command {
	var listenOn string
	var gsfaOnlySignatures bool
	return &cli.Command{
		Name:        "rpc-server-filecoin",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<slot-to-cid-index-filepath-or-url> <sig-to-cid-index-filepath-or-url> <gsfa-index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Listen address",
				Value:       ":8899",
				Destination: &listenOn,
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "Load config from file instead of arguments",
				Value: "",
			},
			&cli.BoolFlag{
				Name:        "gsfa-only-signatures",
				Usage:       "gSFA: only return signatures",
				Value:       false,
				Destination: &gsfaOnlySignatures,
			},
		},
		Action: func(c *cli.Context) error {
			config, err := rpcServerFilecoinLoadConfig(c)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			spew.Dump(config)
			if config.Indexes.SlotToCid == "" {
				return cli.Exit("Must provide a slot-to-CID index filepath/url", 1)
			}
			if config.Indexes.SigToCid == "" {
				return cli.Exit("Must provide a signature-to-CID index filepath/url", 1)
			}

			slotToCidIndexFile, err := openIndexStorage(config.Indexes.SlotToCid)
			if err != nil {
				return fmt.Errorf("failed to open slot-to-cid index file: %w", err)
			}
			defer slotToCidIndexFile.Close()

			slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open slot-to-cid index: %w", err)
			}

			sigToCidIndexFile, err := openIndexStorage(config.Indexes.SigToCid)
			if err != nil {
				return fmt.Errorf("failed to open sig-to-cid index file: %w", err)
			}
			defer sigToCidIndexFile.Close()

			sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open sig-to-cid index: %w", err)
			}

			ls, err := newLassieWrapper(c)
			if err != nil {
				return fmt.Errorf("newLassieWrapper: %w", err)
			}

			var gsfaIndex *gsfa.GsfaReader
			if config.Indexes.Gsfa != "" {
				gsfaIndex, err = gsfa.NewGsfaReader(config.Indexes.Gsfa)
				if err != nil {
					return fmt.Errorf("failed to open gsfa index: %w", err)
				}
				defer gsfaIndex.Close()
			}

			options := &RpcServerOptions{
				ListenOn:           listenOn,
				GsfaOnlySignatures: gsfaOnlySignatures,
			}

			return createAndStartRPCServer_lassie(
				c.Context,
				options,
				ls,
				slotToCidIndex,
				sigToCidIndex,
				gsfaIndex,
			)
		},
	}
}

func rpcServerFilecoinLoadConfig(c *cli.Context) (*RpcServerFilecoinConfig, error) {
	// Either load from config file or from args:
	cfg := &RpcServerFilecoinConfig{}
	if slotToCidIndexFilepath := c.Args().Get(0); slotToCidIndexFilepath != "" {
		cfg.Indexes.SlotToCid = slotToCidIndexFilepath
	}
	if sigToCidIndexFilepath := c.Args().Get(1); sigToCidIndexFilepath != "" {
		cfg.Indexes.SigToCid = sigToCidIndexFilepath
	}
	if gsfaIndexDir := c.Args().Get(2); gsfaIndexDir != "" {
		cfg.Indexes.Gsfa = gsfaIndexDir
	}

	// if a file is specified, load it:
	if configFilepath := c.String("config"); configFilepath != "" {
		configFromFile, err := loadRpcServerFilecoinConfig(configFilepath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		if cfg.Indexes.SlotToCid == "" {
			cfg.Indexes.SlotToCid = configFromFile.Indexes.SlotToCid
		}
		if cfg.Indexes.SigToCid == "" {
			cfg.Indexes.SigToCid = configFromFile.Indexes.SigToCid
		}
		if cfg.Indexes.Gsfa == "" {
			cfg.Indexes.Gsfa = configFromFile.Indexes.Gsfa
		}
	}

	return cfg, nil
}

func loadRpcServerFilecoinConfig(configFilepath string) (*RpcServerFilecoinConfig, error) {
	cfg := &RpcServerFilecoinConfig{}
	err := cfg.load(configFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func (cfg *RpcServerFilecoinConfig) load(configFilepath string) error {
	// if is json, load from json:
	if isJSONFile(configFilepath) {
		return cfg.loadFromJSON(configFilepath)
	}
	if isYAMLFile(configFilepath) {
		return cfg.loadFromYAML(configFilepath)
	}
	return fmt.Errorf("unknown file type for config: %s", configFilepath)
}

func (cfg *RpcServerFilecoinConfig) loadFromJSON(configFilepath string) error {
	file, err := os.Open(configFilepath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(cfg)
}

func (cfg *RpcServerFilecoinConfig) loadFromYAML(configFilepath string) error {
	file, err := os.Open(configFilepath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return yaml.NewDecoder(file).Decode(cfg)
}

func isJSONFile(filepath string) bool {
	return filepath[len(filepath)-5:] == ".json"
}

func isYAMLFile(filepath string) bool {
	return filepath[len(filepath)-5:] == ".yaml" || filepath[len(filepath)-4:] == ".yml"
}

type RpcServerFilecoinConfig struct {
	Indexes struct {
		SlotToCid string `json:"slot_to_cid" yaml:"slot_to_cid"`
		SigToCid  string `json:"sig_to_cid" yaml:"sig_to_cid"`
		Gsfa      string `json:"gsfa" yaml:"gsfa"`
	} `json:"indexes" yaml:"indexes"`
}
