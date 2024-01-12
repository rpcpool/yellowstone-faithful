package main

import (
	"fmt"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/ybbus/jsonrpc/v3"

	"github.com/anjor/carlet"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_check_deals() *cli.Command {
	var includePatterns cli.StringSlice
	var excludePatterns cli.StringSlice
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

			// Check deals:
			for _, config := range configs {
				epoch := *config.Epoch
				klog.Infof("Checking pieces for epoch %d", epoch)
				isLassieMode := config.IsFilecoinMode()
				isCarMode := !isLassieMode
				if isCarMode && config.IsSplitCarMode() {
					klog.Infof("Checking pieces for epoch %d, CAR mode", epoch)

					metadata, err := splitcarfetcher.MetadataFromYaml(string(config.Data.Car.FromPieces.Metadata.URI))
					if err != nil {
						return fmt.Errorf("failed to read pieces metadata: %w", err)
					}

					dealRegistry, err := splitcarfetcher.DealsFromCSV(string(config.Data.Car.FromPieces.Deals.URI))
					if err != nil {
						return fmt.Errorf("failed to read deals: %w", err)
					}

					lotusAPIAddress := "https://api.node.glif.io"
					cl := jsonrpc.NewClient(lotusAPIAddress)
					dm := splitcarfetcher.NewMinerInfo(
						cl,
						5*time.Minute,
						5*time.Second,
					)

					_, err = splitcarfetcher.NewSplitCarReader(metadata.CarPieces,
						func(piece carlet.CarFile) (splitcarfetcher.ReaderAtCloserSize, error) {
							minerID, ok := dealRegistry.GetMinerByPieceCID(piece.CommP)
							if !ok {
								return nil, fmt.Errorf("failed to find miner for piece CID %s", piece.CommP)
							}
							klog.Infof("piece CID %s is supposedly stored on miner %s", piece.CommP, minerID)
							minerInfo, err := dm.GetProviderInfo(c.Context, minerID)
							if err != nil {
								return nil, fmt.Errorf("failed to get miner info for miner %s, for piece %s: %w", minerID, piece.CommP, err)
							}
							if len(minerInfo.Multiaddrs) == 0 {
								return nil, fmt.Errorf("miner %s has no multiaddrs", minerID)
							}
							// spew.Dump(minerInfo)
							// extract the IP address from the multiaddr:
							split := multiaddr.Split(minerInfo.Multiaddrs[0])
							if len(split) < 2 {
								return nil, fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
							}
							component0 := split[0].(*multiaddr.Component)
							component1 := split[1].(*multiaddr.Component)

							var ip string

							if component0.Protocol().Code == multiaddr.P_IP4 {
								ip = component0.Value()
							} else if component1.Protocol().Code == multiaddr.P_IP4 {
								ip = component1.Value()
							} else {
								return nil, fmt.Errorf("invalid multiaddr: %s", minerInfo.Multiaddrs[0])
							}
							// reset the port to 80:
							// TODO: use the appropriate port (80, better if 443 with TLS)
							port := "80"
							minerIP := fmt.Sprintf("%s:%s", ip, port)
							klog.Infof("epoch %d: piece CID %s is stored on miner %s (%s)", epoch, piece.CommP, minerID, minerIP)
							formattedURL := fmt.Sprintf("http://%s/piece/%s", minerIP, piece.CommP.String())

							size, err := splitcarfetcher.GetContentSizeWithHeadOrZeroRange(formattedURL)
							if err != nil {
								return nil, fmt.Errorf("epoch %d: failed to get content size from %q: %s", epoch, formattedURL, err)
							}
							klog.Infof("[OK] content size for piece CID %s is %d", piece.CommP, size)
							return splitcarfetcher.NewRemoteFileSplitCarReader(
								piece.CommP.String(),
								formattedURL,
							)
						})
					if err != nil {
						return fmt.Errorf("epoch %d: failed to open CAR file from pieces: %w", epoch, err)
					} else {
						klog.Infof("[OK] Pieces for epoch %d are all retrievable", epoch)
					}
				} else {
					klog.Infof("Car file for epoch %d is not stored as split pieces, skipping", epoch)
				}
			}

			return nil
		},
	}
}
