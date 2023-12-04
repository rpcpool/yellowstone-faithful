package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

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
	return loadFromJSON(configFilepath, cfg)
}

func (cfg *RpcServerFilecoinConfig) loadFromYAML(configFilepath string) error {
	return loadFromYAML(configFilepath, cfg)
}

type RpcServerFilecoinConfig struct {
	Indexes struct {
		SlotToCid string `json:"slot_to_cid" yaml:"slot_to_cid"`
		SigToCid  string `json:"sig_to_cid" yaml:"sig_to_cid"`
		Gsfa      string `json:"gsfa" yaml:"gsfa"`
	} `json:"indexes" yaml:"indexes"`
}
