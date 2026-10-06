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
			if rewardType, ok := RewardTypeToUi(reward.RewardType); ok {
				rewardJson.String("rewardType", rewardType)
			} else {
				rewardJson.Null("rewardType")
			}
			if reward.Commission != "" {
				rewardJson.Float("commission", asFloat(reward.Commission))
			} else {
				rewardJson.Null("commission")
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

// RewardTypeToUi returns the JSON name of a reward type (Agave's serde variant
// name). Unspecified and unknown values have none and serialize as null, as in Agave.
func RewardTypeToUi(t confirmed_block.RewardType) (string, bool) {
	if t == confirmed_block.RewardType_Unspecified {
		return "", false
	}
	name, ok := confirmed_block.RewardType_name[int32(t)]
	return name, ok
}

func asFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0
	}
	return f
}
