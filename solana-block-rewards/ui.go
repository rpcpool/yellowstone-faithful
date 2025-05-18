package solanablockrewards

import (
	"fmt"

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
			rewardJson.String("rewardType", reward.RewardType.String())
			rewardJson.Float("commission", asFloat(reward.Commission))
		}
		rewardsArray.AddObject(rewardJson)
	}
	if rewards.NumPartitions != nil {
		numPart := rewards.NumPartitions.NumPartitions
		return rewardsArray, &numPart, nil
	}
	return rewardsArray, nil, nil
}

func asFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		panic(err)
	}
	return f
}
