package solanablockrewards

import (
	"fmt"
	"sort"

	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

func SortRewardsByPubkey(rewards *confirmed_block.Rewards) {
	// Sort rewards by Pubkey for consistent output.
	if rewards != nil && rewards.Rewards != nil {
		sort.Slice(rewards.Rewards, func(i, j int) bool {
			return rewards.Rewards[i].Pubkey < rewards.Rewards[j].Pubkey
		})
	}
}

func ParseRewards(buf []byte) (*confirmed_block.Rewards, error) {
	protobufRewards, err := ParseRewardsProtobuf(buf)
	if err == nil {
		return protobufRewards, nil
	}
	{
		rewards, err := ParseSerdeRewards(buf)
		if err == nil {
			protoRewards, err := SerdeRewardsToProtobuf(rewards)
			if err != nil {
				return nil, err
			}
			return protoRewards, nil
		}
	}
	serdeStoredRewards, err := ParseSerdeStoredConfirmedBlockRewards(buf)
	if err == nil {
		protoRewards, err := SerdeStoredConfirmedBlockRewardsToProtobuf(*serdeStoredRewards)
		if err != nil {
			return nil, err
		}
		return protoRewards, nil
	}
	return nil, err
}

func ParseRewardsProtobuf(buf []byte) (*confirmed_block.Rewards, error) {
	var rewards confirmed_block.Rewards
	err := proto.Unmarshal(buf, &rewards)
	if err != nil {
		return nil, err
	}
	return &rewards, nil
}

func ParseSerdeStoredConfirmedBlockRewards(buf []byte) (*transaction_status_meta_serde_agave.StoredConfirmedBlockRewards, error) {
	rewards, err := transaction_status_meta_serde_agave.BincodeDeserializeStoredConfirmedBlockRewards(buf)
	if err != nil {
		return nil, err
	}
	return rewards, nil
}

func ParseSerdeRewards(buf []byte) (transaction_status_meta_serde_agave.Rewards, error) {
	rewards, err := transaction_status_meta_serde_agave.BincodeDeserializeRewards(buf)
	if err != nil {
		return nil, err
	}
	return rewards, nil
}

func SerdeStoredConfirmedBlockRewardsToProtobuf(rewards transaction_status_meta_serde_agave.StoredConfirmedBlockRewards) (*confirmed_block.Rewards, error) {
	protoRewards := &confirmed_block.Rewards{}
	for _, reward := range rewards {
		protoReward := &confirmed_block.Reward{
			Pubkey:   reward.Pubkey,
			Lamports: reward.Lamports,
		}
		protoRewards.Rewards = append(protoRewards.Rewards, protoReward)
	}
	return protoRewards, nil
}

func SerdeRewardsToProtobuf(rewards transaction_status_meta_serde_agave.Rewards) (*confirmed_block.Rewards, error) {
	protoRewards := &confirmed_block.Rewards{}
	for _, reward := range rewards {
		protoReward := &confirmed_block.Reward{
			Pubkey:      reward.Pubkey,
			Lamports:    reward.Lamports,
			PostBalance: reward.PostBalance,
		}
		{
			if reward.RewardType != nil {
				protoReward.RewardType = func() confirmed_block.RewardType {
					switch (*reward.RewardType).(type) {
					case *transaction_status_meta_serde_agave.RewardType__Fee:
						return confirmed_block.RewardType_Fee
					case *transaction_status_meta_serde_agave.RewardType__Rent:
						return confirmed_block.RewardType_Rent
					case *transaction_status_meta_serde_agave.RewardType__Voting:
						return confirmed_block.RewardType_Voting
					case *transaction_status_meta_serde_agave.RewardType__Staking:
						return confirmed_block.RewardType_Staking
					default:
						return confirmed_block.RewardType_Unspecified
					}
				}()
			}
			if reward.Commission != nil {
				// encode uint8 to string
				asString := fmt.Sprintf("%d", *reward.Commission)
				protoReward.Commission = asString
			}
		}
		protoRewards.Rewards = append(protoRewards.Rewards, protoReward)
	}
	return protoRewards, nil
}
