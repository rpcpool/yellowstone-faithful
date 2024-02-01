package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

const ConfigVersion = 1

type URI string

// String() returns the URI as a string.
func (u URI) String() string {
	return string(u)
}

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

func LoadConfig(configFilepath string) (*Config, error) {
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
	sum, err := hashFileSha256(configFilepath)
	if err != nil {
		return nil, fmt.Errorf("config file %q: %s", configFilepath, err.Error())
	}
	config.hashOfConfigFile = sum
	return &config, nil
}

func hashFileSha256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type PieceURLInfo struct {
	URI URI `json:"uri" yaml:"uri"` // URL to the piece.
}

type Config struct {
	originalFilepath string
	hashOfConfigFile string
	Epoch            *uint64 `json:"epoch" yaml:"epoch"`
	Version          *uint64 `json:"version" yaml:"version"`
	Data             struct {
		Car *struct {
			URI        URI `json:"uri" yaml:"uri"`
			FromPieces *struct {
				Metadata struct {
					URI URI `json:"uri" yaml:"uri"` // Local path to the metadata file.
				} `json:"metadata" yaml:"metadata"`
				Deals struct {
					URI URI `json:"uri" yaml:"uri"` // Local path to the deals file.
				} `json:"deals" yaml:"deals"`
				PieceToURI map[cid.Cid]PieceURLInfo `json:"piece_to_uri" yaml:"piece_to_uri"` // Map of piece CID to URL.
			} `json:"from_pieces" yaml:"from_pieces"`
		} `json:"car" yaml:"car"`
		Filecoin *struct {
			// Enable enables Filecoin mode. If false, or if this section is not present, CAR mode is used.
			Enable    bool     `json:"enable" yaml:"enable"`
			RootCID   cid.Cid  `json:"root_cid" yaml:"root_cid"`
			Providers []string `json:"providers" yaml:"providers"`
		} `json:"filecoin" yaml:"filecoin"`
	} `json:"data" yaml:"data"`
	Indexes struct {
		CidToOffsetAndSize struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"cid_to_offset_and_size" yaml:"cid_to_offset_and_size"` // Latest index version. Includes offset and size.
		CidToOffset struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"cid_to_offset" yaml:"cid_to_offset"` // Legacy	index, deprecated. Only includes offset.
		SlotToCid struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"slot_to_cid" yaml:"slot_to_cid"`
		SigToCid struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"sig_to_cid" yaml:"sig_to_cid"`
		Gsfa struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"gsfa" yaml:"gsfa"`
		SigExists struct {
			URI URI `json:"uri" yaml:"uri"`
		} `json:"sig_exists" yaml:"sig_exists"`
	} `json:"indexes" yaml:"indexes"`
	Genesis struct {
		URI URI `json:"uri" yaml:"uri"`
	} `json:"genesis" yaml:"genesis"`
}

// IsDeprecatedIndexes returns true if the config is using the deprecated indexes version.
func (c *Config) IsDeprecatedIndexes() bool {
	// CidToOffsetAndSize is not set and CidToOffset is set.
	return c.Indexes.CidToOffsetAndSize.URI.IsZero() && !c.Indexes.CidToOffset.URI.IsZero()
}

func (c *Config) ConfigFilepath() string {
	return c.originalFilepath
}

func (c *Config) HashOfConfigFile() string {
	return c.hashOfConfigFile
}

func (c *Config) IsSameHash(other *Config) bool {
	return c.hashOfConfigFile == other.hashOfConfigFile
}

func (c *Config) IsSameHashAsFile(filepath string) bool {
	sum, err := hashFileSha256(filepath)
	if err != nil {
		return false
	}
	return c.hashOfConfigFile == sum
}

// IsFilecoinMode returns true if the config is in Filecoin mode.
// This means that the data is going to be fetched from Filecoin directly (by CID).
func (c *Config) IsFilecoinMode() bool {
	return c.Data.Filecoin != nil && c.Data.Filecoin.Enable
}

func (c *Config) IsCarFromPieces() bool {
	if c.Data.Car == nil || c.Data.Car.FromPieces == nil {
		return false
	}
	fromPieces := c.Data.Car.FromPieces
	return !fromPieces.Metadata.URI.IsZero() && (!fromPieces.Deals.URI.IsZero() || len(fromPieces.PieceToURI) > 0)
}

type ConfigSlice []*Config

func (c ConfigSlice) Validate() error {
	for _, config := range c {
		if err := config.Validate(); err != nil {
			return fmt.Errorf("config file %q: %s", config.ConfigFilepath(), err.Error())
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

func isSupportedURI(uri URI, path string) error {
	isSupported := uri.IsLocal() || uri.IsRemoteWeb()
	if !isSupported {
		return fmt.Errorf("%s must be a local file or a remote web URI", path)
	}
	return nil
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	if c.Epoch == nil {
		return fmt.Errorf("epoch must be set")
	}
	if c.Version == nil {
		return fmt.Errorf("version must be set")
	}
	if *c.Version != ConfigVersion {
		return fmt.Errorf("version must be %d", ConfigVersion)
	}
	// Distinguish between CAR-mode and Filecoin-mode.
	// In CAR-mode, the data is fetched from a CAR file (local or remote).
	// In Filecoin-mode, the data is fetched from Filecoin directly (by CID via Lassie).
	isFilecoinMode := c.IsFilecoinMode()
	isCarMode := !isFilecoinMode
	if isCarMode {
		if c.Data.Car == nil {
			return fmt.Errorf("car-mode=true; data.car must be set")
		}
		if c.Data.Car.URI.IsZero() && c.Data.Car.FromPieces == nil {
			return fmt.Errorf("data.car.uri or data.car.from_pieces must be set")
		}
		if !c.Data.Car.URI.IsZero() {
			if err := isSupportedURI(c.Data.Car.URI, "data.car.uri"); err != nil {
				return err
			}
		}
		// can't have both:
		if !c.Data.Car.URI.IsZero() && c.Data.Car.FromPieces != nil {
			return fmt.Errorf("data.car.uri and data.car.from_pieces cannot both be set")
		}
		if c.Data.Car.FromPieces != nil {
			{
				if c.Data.Car.FromPieces.Metadata.URI.IsZero() {
					return fmt.Errorf("data.car.from_pieces.metadata.uri must be set")
				}
				if !c.Data.Car.FromPieces.Metadata.URI.IsLocal() {
					return fmt.Errorf("data.car.from_pieces.metadata.uri must be a local file")
				}
			}
			{
				if c.Data.Car.FromPieces.Deals.URI.IsZero() && len(c.Data.Car.FromPieces.PieceToURI) == 0 {
					return fmt.Errorf("data.car.from_pieces.deals.uri or data.car.from_pieces.piece_to_uri must be set")
				}
				if !c.Data.Car.FromPieces.Deals.URI.IsZero() && len(c.Data.Car.FromPieces.PieceToURI) > 0 {
					return fmt.Errorf("data.car.from_pieces.deals.uri and data.car.from_pieces.piece_to_uri cannot both be set")
				}
				if !c.Data.Car.FromPieces.Deals.URI.IsZero() && !c.Data.Car.FromPieces.Deals.URI.IsLocal() {
					return fmt.Errorf("data.car.from_pieces.deals.uri must be a local file")
				}
				if len(c.Data.Car.FromPieces.PieceToURI) > 0 {
					for pieceCID, uri := range c.Data.Car.FromPieces.PieceToURI {
						if !pieceCID.Defined() {
							return fmt.Errorf("data.car.from_pieces.piece_to_uri[%s] must be a valid CID", pieceCID)
						}
						if uri.URI.IsZero() {
							return fmt.Errorf("data.car.from_pieces.piece_to_uri[%s].uri must be set", pieceCID)
						}
						if !uri.URI.IsRemoteWeb() {
							return fmt.Errorf("data.car.from_pieces.piece_to_uri[%s].uri must be a remote web URI", pieceCID)
						}
					}
				}
			}
		}
		// CidToOffsetAndSize and CidToOffset cannot be both set or both unset.
		if !c.Indexes.CidToOffsetAndSize.URI.IsZero() && !c.Indexes.CidToOffset.URI.IsZero() {
			return fmt.Errorf("indexes.cid_to_offset_and_size.uri and indexes.cid_to_offset.uri cannot both be set")
		}
		if c.Indexes.CidToOffsetAndSize.URI.IsZero() && c.Indexes.CidToOffset.URI.IsZero() {
			return fmt.Errorf("indexes.cid_to_offset_and_size.uri and indexes.cid_to_offset.uri cannot both be unset")
		}
		// validate CidToOffsetAndSize URI:
		if !c.Indexes.CidToOffsetAndSize.URI.IsZero() {
			if err := isSupportedURI(c.Indexes.CidToOffsetAndSize.URI, "indexes.cid_to_offset_and_size.uri"); err != nil {
				return err
			}
		}
		// validate CidToOffset URI:
		if !c.Indexes.CidToOffset.URI.IsZero() {
			if err := isSupportedURI(c.Indexes.CidToOffset.URI, "indexes.cid_to_offset.uri"); err != nil {
				return err
			}
		}
	} else {
		if c.Data.Filecoin == nil {
			return fmt.Errorf("car-mode=false; data.filecoin must be set")
		}
		if !c.Data.Filecoin.RootCID.Defined() {
			return fmt.Errorf("data.filecoin.root_cid must be set")
		}
		// validate providers:
		for providerIndex, provider := range c.Data.Filecoin.Providers {
			if provider == "" {
				return fmt.Errorf("data.filecoin.providers must not be empty")
			}
			_, err := peer.AddrInfoFromString(provider)
			if err != nil {
				return fmt.Errorf("data.filecoin.providers[%d]: error parsing provider %q: %w", providerIndex, provider, err)
			}
		}

	}

	{
		{
			if c.Indexes.SlotToCid.URI.IsZero() {
				return fmt.Errorf("indexes.slot_to_cid.uri must be set")
			}
			if err := isSupportedURI(c.Indexes.SlotToCid.URI, "indexes.slot_to_cid.uri"); err != nil {
				return err
			}
		}
		{
			if c.Indexes.SigToCid.URI.IsZero() {
				return fmt.Errorf("indexes.sig_to_cid.uri must be set")
			}
			if err := isSupportedURI(c.Indexes.SigToCid.URI, "indexes.sig_to_cid.uri"); err != nil {
				return err
			}
		}
		{
			if c.Indexes.SigExists.URI.IsZero() {
				return fmt.Errorf("indexes.sig_exists.uri must be set")
			}
			if err := isSupportedURI(c.Indexes.SigExists.URI, "indexes.sig_exists.uri"); err != nil {
				return err
			}
		}
	}
	{
		// check that the URIs are valid
		if isCarMode {
			if !c.Indexes.CidToOffsetAndSize.URI.IsZero() && !c.Indexes.CidToOffsetAndSize.URI.IsValid() {
				return fmt.Errorf("indexes.cid_to_offset_and_size.uri is invalid")
			}
			if !c.Indexes.CidToOffset.URI.IsZero() && !c.Indexes.CidToOffset.URI.IsValid() {
				return fmt.Errorf("indexes.cid_to_offset.uri is invalid")
			}
			if c.Data.Car.FromPieces != nil {
				if !c.Data.Car.FromPieces.Metadata.URI.IsValid() {
					return fmt.Errorf("data.car.from_pieces.metadata.uri is invalid")
				}
				if !c.Data.Car.FromPieces.Deals.URI.IsZero() && !c.Data.Car.FromPieces.Deals.URI.IsValid() {
					return fmt.Errorf("data.car.from_pieces.deals.uri is invalid")
				}
			} else {
				if !c.Data.Car.URI.IsValid() {
					return fmt.Errorf("data.car.uri is invalid")
				}
			}
		}
		if !c.Indexes.SlotToCid.URI.IsValid() {
			return fmt.Errorf("indexes.slot_to_cid.uri is invalid")
		}
		if !c.Indexes.SigToCid.URI.IsValid() {
			return fmt.Errorf("indexes.sig_to_cid.uri is invalid")
		}
		if !c.Indexes.SigExists.URI.IsValid() {
			return fmt.Errorf("indexes.sig_exists.uri is invalid")
		}
		{
			if !c.Indexes.Gsfa.URI.IsZero() && !c.Indexes.Gsfa.URI.IsValid() {
				return fmt.Errorf("indexes.gsfa.uri is invalid")
			}
			// gsfa index (optional), if set, must be a local directory:
			if !c.Indexes.Gsfa.URI.IsZero() && !c.Indexes.Gsfa.URI.IsLocal() {
				return fmt.Errorf("indexes.gsfa.uri must be a local directory")
			}
		}
	}
	{
		// if epoch is 0, then the genesis URI must be set:
		if *c.Epoch == 0 {
			if c.Genesis.URI.IsZero() {
				return fmt.Errorf("epoch is 0, but genesis.uri is not set")
			}
			if !c.Genesis.URI.IsValid() {
				return fmt.Errorf("genesis.uri is invalid")
			}
			// only support local genesis files for now:
			if !c.Genesis.URI.IsLocal() {
				return fmt.Errorf("genesis.uri must be a local file")
			}
		}
	}
	return nil
}
