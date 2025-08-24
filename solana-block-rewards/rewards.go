package solanablockrewards

import (
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
	serdeRewards, err := ParseRewardsSerde(buf)
	if err == nil {
		protoRewards, err := SerdeToProtobuf(*serdeRewards)
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

func ParseRewardsSerde(buf []byte) (*transaction_status_meta_serde_agave.StoredConfirmedBlockRewards, error) {
	rewards, err := transaction_status_meta_serde_agave.BincodeDeserializeStoredConfirmedBlockRewards(buf)
	if err != nil {
		return nil, err
	}
	return rewards, nil
}

func SerdeToProtobuf(rewards transaction_status_meta_serde_agave.StoredConfirmedBlockRewards) (*confirmed_block.Rewards, error) {
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
