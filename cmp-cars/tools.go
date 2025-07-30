package main

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	ytooling "github.com/rpcpool/yellowstone-faithful/tooling"
)

func getParsedRewards(parsedDag nodetools.ParsedAndCidSlice, rewardsCid cid.Cid) (*confirmed_block.Rewards, error) {
	rewardsNode1, err := parsedDag.RewardsByCid(rewardsCid)
	if err != nil {
		return nil, fmt.Errorf("failed to get rewards node by cid: %v", err)
	}
	rewardsBuf, err := ipldbindcode.LoadDataFromDataFrames(&rewardsNode1.Data, func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
		df, err := parsedDag.DataFrameByCid(wantedCid)
		return df, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load Rewards dataFrames: %v", err)
	}

	uncompressedRewards, err := ytooling.DecompressZstd(rewardsBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress Rewards: %v", err)
	}
	actualRewards, err := solanablockrewards.ParseRewards(uncompressedRewards)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Rewards: %v", err)
	}
	return actualRewards, nil
}
