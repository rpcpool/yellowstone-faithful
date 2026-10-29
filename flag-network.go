package main

import (
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/urfave/cli/v2"
)

// newFlag_network returns a --network flag; the chosen network selects the epoch
// schedule (see indexes.Network.ApplyEpochSchedule).
func newFlag_network(network *indexes.Network) *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "network",
		Usage:       "the cluster; selects the epoch schedule; one of: mainnet, testnet, devnet",
		Value:       string(indexes.NetworkMainnet),
		Destination: (*string)(network),
		Action: func(c *cli.Context, s string) error {
			if !indexes.IsValidNetwork(indexes.Network(s)) {
				return fmt.Errorf("invalid network: %q", s)
			}
			return nil
		},
	}
}
