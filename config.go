package main

import (
	"errors"
	"fmt"
	"sort"

	"github.com/ipfs/go-cid"
)

type URI string

// IsZero returns true if the URI is empty.
func (u URI) IsZero() bool {
	return u == ""
}

// IsValid returns true if the URI is not empty and is a valid URI.
func (u URI) IsValid() bool {
	if u.IsZero() {
		return false
	}
	return u.IsLocal() || u.IsRemoteWeb() || u.IsCID() || u.IsIPFS() || u.IsFilecoin()
}

// IsLocal returns true if the URI is a local file or directory.
func (u URI) IsLocal() bool {
	return (len(u) > 7 && u[:7] == "file://") || (len(u) > 1 && u[0] == '/')
}

// IsRemoteWeb returns true if the URI is a remote web URI (HTTP or HTTPS).
func (u URI) IsRemoteWeb() bool {
	// http:// or https://
	return len(u) > 7 && u[:7] == "http://" || len(u) > 8 && u[:8] == "https://"
}

// IsCID returns true if the URI is a CID.
func (u URI) IsCID() bool {
	if u.IsZero() {
		return false
	}
	parsed, err := cid.Parse(string(u))
	return err == nil && parsed.Defined()
}

// IsIPFS returns true if the URI is an IPFS URI.
func (u URI) IsIPFS() bool {
	return len(u) > 6 && u[:6] == "ipfs://"
}

// IsFilecoin returns true if the URI is a Filecoin URI.
func (u URI) IsFilecoin() bool {
	return len(u) > 10 && u[:10] == "filecoin://"
}

func loadConfig(configFilepath string) (*Config, error) {
	var config Config
	if isJSONFile(configFilepath) {
		if err := loadFromJSON(configFilepath, &config); err != nil {
			return nil, err
		}
	} else if isYAMLFile(configFilepath) {
		if err := loadFromYAML(configFilepath, &config); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("config file %q must be JSON or YAML", configFilepath)
	}
	config.originalFilepath = configFilepath
	return &config, nil
}

type Config struct {
	originalFilepath string
	Epoch            *uint64 `json:"epoch" yaml:"epoch"`
	Data             struct {
		FilecoinMode bool `json:"filecoin_mode" yaml:"filecoin_mode"`
		URI          URI  `json:"uri" yaml:"uri"`
	} `json:"data" yaml:"data"`
	Indexes struct {
		CidToOffset struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"cid_to_offset" yaml:"cid_to_offset"`
		SlotToCid struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"slot_to_cid" yaml:"slot_to_cid"`
		SigToCid struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"sig_to_cid" yaml:"sig_to_cid"`
		Gsfa struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"gsfa" yaml:"gsfa"`
	} `json:"indexes" yaml:"indexes"`
}

func (c *Config) ConfigFilepath() string {
	return c.originalFilepath
}

// IsFilecoinMode returns true if the config is in Filecoin mode.
// This means that the data is going to be fetched from Filecoin directly (by CID).
func (c *Config) IsFilecoinMode() bool {
	return c.Data.FilecoinMode
}

type ConfigSlice []*Config

func (c ConfigSlice) Validate() error {
	for _, config := range c {
		if err := config.Validate(); err != nil {
			return err
		}
	}
	{
		// Check that all epochs are unique.
		epochs := make(map[uint64][]string)
		for _, config := range c {
			epochs[*config.Epoch] = append(epochs[*config.Epoch], config.originalFilepath)
		}
		multiErrors := make([]error, 0)
		for epoch, configFiles := range epochs {
			if len(configFiles) > 1 {
				multiErrors = append(multiErrors, fmt.Errorf("epoch %d is defined in multiple config files: %v", epoch, configFiles))
			}
		}
		if len(multiErrors) > 0 {
			return errors.Join(multiErrors...)
		}
	}
	return nil
}

func (c ConfigSlice) SortByEpoch() {
	sort.Slice(c, func(i, j int) bool {
		return *c[i].Epoch < *c[j].Epoch
	})
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	if c.Epoch == nil {
		return fmt.Errorf("epoch must be set")
	}
	// Distinguish between CAR-mode and Filecoin-mode.
	// In CAR-mode, the data is fetched from a CAR file (local or remote).
	// In Filecoin-mode, the data is fetched from Filecoin directly (by CID via Lassie).
	isFilecoinMode := c.Data.FilecoinMode
	isCarMode := !isFilecoinMode
	if isCarMode {
		if c.Data.URI.IsZero() {
			return fmt.Errorf("data.uri must be set")
		}
		if c.Indexes.CidToOffset.URI.IsZero() {
			return fmt.Errorf("indexes.cid_to_offset.uri must be set")
		}
	}

	if c.Indexes.SlotToCid.URI.IsZero() {
		return fmt.Errorf("indexes.slot_to_cid.uri must be set")
	}
	if c.Indexes.SigToCid.URI.IsZero() {
		return fmt.Errorf("indexes.sig_to_cid.uri must be set")
	}
	// The GSFA index is optional.
	// if c.Indexes.Gsfa.URI.IsZero() {
	// 	return fmt.Errorf("indexes.gsfa.uri must be set")
	// }
	{
		// check that the URIs are valid
		if isCarMode {
			if !c.Data.URI.IsValid() {
				return fmt.Errorf("data.uri is invalid")
			}
			if !c.Indexes.CidToOffset.URI.IsValid() {
				return fmt.Errorf("indexes.cid_to_offset.uri is invalid")
			}
		}
		if !c.Indexes.SlotToCid.URI.IsValid() {
			return fmt.Errorf("indexes.slot_to_cid.uri is invalid")
		}
		if !c.Indexes.SigToCid.URI.IsValid() {
			return fmt.Errorf("indexes.sig_to_cid.uri is invalid")
		}
		if !c.Indexes.Gsfa.URI.IsZero() && !c.Indexes.Gsfa.URI.IsValid() {
			return fmt.Errorf("indexes.gsfa.uri is invalid")
		}
		// gsfa, if set, must be a local directory:
		if !c.Indexes.Gsfa.URI.IsZero() && !c.Indexes.Gsfa.URI.IsLocal() {
			return fmt.Errorf("indexes.gsfa.uri must be a local directory")
		}
	}
	return nil
}
