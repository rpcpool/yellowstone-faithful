package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anjor/carlet"
	"github.com/davecgh/go-spew/spew"
	"github.com/multiformats/go-multiaddr"
	"github.com/ybbus/jsonrpc/v3"

	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

type commaSeparatedStringSliceFlag struct {
	slice []string
}

func (f *commaSeparatedStringSliceFlag) String() string {
	return fmt.Sprintf("%v", f.slice)
}

func (f *commaSeparatedStringSliceFlag) Set(value string) error {
	// split by ",":
	split := strings.Split(value, ",")
	for _, item := range split {
		// trim spaces:
		item = strings.TrimSpace(item)
		f.slice = append(f.slice, item)
	}
	return nil
}

// Has
func (f *commaSeparatedStringSliceFlag) Has(value string) bool {
	for _, item := range f.slice {
		if item == value {
			return true
		}
	}
	return false
}

func (f *commaSeparatedStringSliceFlag) Len() int {
	return len(f.slice)
}

func newCmd_check_deals() *cli.Command {
	var includePatterns cli.StringSlice
	var excludePatterns cli.StringSlice
	var providerAllowlist commaSeparatedStringSliceFlag
	return &cli.Command{
		Name:        "check-deals",
		Description: "Validate remote split car retrieval for the given config files",
		ArgsUsage:   "<one or more config files or directories containing config files (nested is fine)>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:        "include",
				Usage:       "Include files or dirs matching the given glob patterns",
				Value:       cli.NewStringSlice(),
				Destination: &includePatterns,
			},
			&cli.StringSliceFlag{
				Name:        "exclude",
				Usage:       "Exclude files or dirs matching the given glob patterns",
				Value:       cli.NewStringSlice(".git"),
				Destination: &excludePatterns,
			},
			&cli.GenericFlag{
				Name:  "provider-allowlist",
				Usage: "List of providers to allow checking (comma-separated, can be specified multiple times); will ignore all pieces that correspond to a provider not in the allowlist.",
				Value: &providerAllowlist,
			},
		},
		Action: func(c *cli.Context) error {
			src := c.Args().Slice()
			configFiles, err := GetListOfConfigFiles(
				src,
				includePatterns.Value(),
				excludePatterns.Value(),
			)
			if err != nil {
				return cli.Exit(err.Error(), 1)
			}
			klog.Infof("Found %d config files:", len(configFiles))
			for _, configFile := range configFiles {
				fmt.Printf("  - %s\n", configFile)
			}

			// Load configs:
			configs := make(ConfigSlice, 0)
			for _, configFile := range configFiles {
				config, err := LoadConfig(configFile)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to load config file %q: %s", configFile, err.Error()), 1)
				}
				configs = append(configs, config)
			}

			configs.SortByEpoch()
			klog.Infof("Loaded %d epoch configs (NO VALIDATION)", len(configs))
			klog.Info("Will check remote storage pieces for each epoch config")

			// Check provider allowlist:
			if providerAllowlist.Len() > 0 {
				klog.Infof("Provider allowlist: %v", providerAllowlist.slice)
			} else {
				klog.Infof("Provider allowlist: <empty>")
			}

			lotusAPIAddress := "https://api.node.glif.io"
			cl := jsonrpc.NewClient(lotusAPIAddress)
			dm := splitcarfetcher.NewMinerInfo(
				cl,
				24*time.Hour,
				5*time.Second,
			)

			// Check deals:
			for _, config := range configs {
				epoch := *config.Epoch
				isLassieMode := config.IsFilecoinMode()
				isCarMode := !isLassieMode
				if isCarMode && config.IsCarFromPieces() {
					klog.Infof("Checking pieces for epoch %d from %q", epoch, config.ConfigFilepath())

					metadata, err := splitcarfetcher.MetadataFromYaml(string(config.Data.Car.FromPieces.Metadata.URI))
					if err != nil {
						return fmt.Errorf("failed to read pieces metadata: %w", err)
					}

					dealRegistry, err := splitcarfetcher.DealsFromCSV(string(config.Data.Car.FromPieces.Deals.URI))
					if err != nil {
						return fmt.Errorf("failed to read deals: %w", err)
					}

					err = checkAllPieces(
						c.Context,
						epoch,
						metadata,
						dealRegistry,
						providerAllowlist,
						dm,
					)
					if err != nil {
						return fmt.Errorf(
							"error while checking pieces for epoch %d from %q: failed to open CAR file from pieces: %w",
							epoch,
							config.ConfigFilepath(),
							err,
						)
					} else {
						klog.Infof("[OK] Pieces for epoch %d from %q are all retrievable", epoch, config.ConfigFilepath())
					}
				} else {
					klog.Infof("Car file for epoch %d is not stored as split pieces, skipping", epoch)
				}
			}

			return nil
		},
	}
}

func checkAllPieces(
	ctx context.Context,
	epoch uint64,
	meta *splitcarfetcher.Metadata,
	dealRegistry *splitcarfetcher.DealRegistry,
	providerAllowlist commaSeparatedStringSliceFlag,
	dm *splitcarfetcher.MinerInfoCache,
) error {
	errs := make([]error, 0)
	numPieces := len(meta.CarPieces.CarPieces)
	for pieceIndex, piece := range meta.CarPieces.CarPieces {
		pieceIndex := pieceIndex
		err := func(piece carlet.CarFile) error {
			minerID, ok := dealRegistry.GetMinerByPieceCID(piece.CommP)
			if !ok {
				return fmt.Errorf("failed to find miner for piece CID %s", piece.CommP)
			}
			klog.Infof(
				"piece %d/%d with CID %s is supposedly stored on miner %s",
				pieceIndex+1,
				numPieces,
				piece.CommP,
				minerID,
			)
			if providerAllowlist.Len() > 0 {
				if !providerAllowlist.Has(minerID.String()) {
					klog.Infof("skipping piece %d/%d with CID %s, because miner %s is not in the allowlist", pieceIndex+1, numPieces, piece.CommP, minerID)
					return nil
				}
			}
			minerInfo, err := dm.GetProviderInfo(ctx, minerID)
			if err != nil {
				return fmt.Errorf("failed to get miner info for miner %s, for piece %s: %w", minerID, piece.CommP, err)
			}
			if len(minerInfo.Multiaddrs) == 0 {
				return fmt.Errorf("miner %s has no multiaddrs", minerID)
			}
			spew.Dump(minerInfo)
			// extract the IP address from the multiaddr:
			split := multiaddr.Split(minerInfo.Multiaddrs[0])
			if len(split) < 2 {
				return fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
			}
			component0 := split[0].(*multiaddr.Component)
			component1 := split[1].(*multiaddr.Component)

			var ip string

			if component0.Protocol().Code == multiaddr.P_IP4 {
				ip = component0.Value()
			} else if component1.Protocol().Code == multiaddr.P_IP4 {
				ip = component1.Value()
			} else {
				return fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
			}
			// reset the port to 80:
			// TODO: use the appropriate port (80, better if 443 with TLS)
			port := "80"
			minerIP := fmt.Sprintf("%s:%s", ip, port)
			klog.Infof("epoch %d: piece CID %s is stored on miner %s (%s)", epoch, piece.CommP, minerID, minerIP)
			formattedURL := fmt.Sprintf("http://%s/piece/%s", minerIP, piece.CommP.String())

			size, err := splitcarfetcher.GetContentSizeWithHeadOrZeroRange(formattedURL)
			if err != nil {
				return fmt.Errorf(
					"piece %d/%d with CID %s is supposedly stored on miner %s (%s), but failed to get content size from %q: %w",
					pieceIndex+1,
					numPieces,
					piece.CommP,
					minerID,
					minerIP,
					formattedURL,
					err,
				)
			}
			klog.Infof(
				"[OK] piece %d/%d: content size for piece CID %s is %d (from miner %s, resolved to %s)",
				pieceIndex+1,
				numPieces,
				piece.CommP,
				size,
				minerID,
				minerIP,
			)
			return nil
		}(piece)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
