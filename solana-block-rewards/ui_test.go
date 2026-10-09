package solanablockrewards

import (
	"testing"

	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"github.com/stretchr/testify/require"
)

func TestRewardsToUi(t *testing.T) {
	rewards := &confirmed_block.Rewards{
		Rewards: []*confirmed_block.Reward{
			{Pubkey: "a", Lamports: 1, PostBalance: 2, RewardType: confirmed_block.RewardType_Fee},
			{Pubkey: "b", Lamports: -3, PostBalance: 4, RewardType: confirmed_block.RewardType_DeactivatedStake},
			{Pubkey: "c", Lamports: -5, PostBalance: 6, RewardType: confirmed_block.RewardType_VATDebit},
			{Pubkey: "d", Lamports: 7, PostBalance: 8, RewardType: confirmed_block.RewardType_Voting, Commission: "10"},
			{Pubkey: "e", Lamports: 9, PostBalance: 10, RewardType: confirmed_block.RewardType(99)},
		},
		NumPartitions: &confirmed_block.NumPartitions{NumPartitions: 43},
	}
	arr, numPartitions, err := RewardsToUi(rewards)
	require.NoError(t, err)
	defer arr.Put()
	require.NotNil(t, numPartitions)
	require.Equal(t, uint64(43), *numPartitions)

	got, err := arr.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"pubkey":"a","lamports":1,"postBalance":2,"rewardType":"Fee","commission":null},
		{"pubkey":"b","lamports":-3,"postBalance":4,"rewardType":"DeactivatedStake","commission":null},
		{"pubkey":"c","lamports":-5,"postBalance":6,"rewardType":"VATDebit","commission":null},
		{"pubkey":"d","lamports":7,"postBalance":8,"rewardType":"Voting","commission":10},
		{"pubkey":"e","lamports":9,"postBalance":10,"rewardType":null,"commission":null}
	]`, string(got))

	_, numPartitions, err = RewardsToUi(&confirmed_block.Rewards{})
	require.NoError(t, err)
	require.Nil(t, numPartitions)
}
