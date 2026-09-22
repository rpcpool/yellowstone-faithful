package solanablockrewards

import (
	"fmt"
	"strconv"

	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
)

func RewardsToUi(
	rewards *confirmed_block.Rewards,
) (*jsonbuilder.ArrayBuilder, *uint64, error) {
	rewardsArray := jsonbuilder.NewArray()

	for _, reward := range rewards.Rewards {
		rewardJson := jsonbuilder.NewObject()
		{
			rewardJson.String("pubkey", reward.Pubkey)
			rewardJson.Int("lamports", reward.Lamports)
			rewardJson.Uint("postBalance", reward.PostBalance)
			if reward.RewardType != 0 {
				rewardJson.String("rewardType", reward.RewardType.String())
			} else {
				rewardJson.Null("rewardType")
			}
			if reward.Commission != "" {
				rewardJson.Float("commission", asFloat(reward.Commission))
			} else {
				rewardJson.Null("commission")
			}
			// commissionBps (SIMD-0291) is skipped rather than nulled when absent,
			// matching #[serde(skip_serializing_if = "Option::is_none")] in agave.
			if reward.CommissionBps != "" {
				rewardJson.Uint("commissionBps", asUint16(reward.CommissionBps))
			}
		}
		rewardsArray.AddObject(rewardJson)
	}
	if rewards.NumPartitions != nil {
		numPart := rewards.NumPartitions.NumPartitions
		return rewardsArray, &numPart, nil
	}
	return rewardsArray, nil, nil
}

func asUint16(s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0
	}
	return v
}

func asFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0
	}
	return f
}
