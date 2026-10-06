package solanablockrewards

import (
	"encoding/json"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

// Field-for-field parity with what agave 4.2 returns for the same rewards.
func TestRewardsToUi(t *testing.T) {
	buf, err := proto.Marshal(&confirmed_block.Rewards{
		Rewards: []*confirmed_block.Reward{
			{
				Pubkey:        "3ZFkP99s4GvhibGHUtQnF14bnZfmpDmbDd8xeUV37mC8",
				Lamports:      81902744,
				PostBalance:   279084185624,
				RewardType:    confirmed_block.RewardType_DeactivatedStake,
				CommissionBps: "0",
			},
			{
				Pubkey:        "GdnSyH3YtwcxFvQrVVJMm1JhTS4QVX7MFsX56uJLUfiZ",
				Lamports:      2500,
				PostBalance:   100000,
				RewardType:    confirmed_block.RewardType_Voting,
				Commission:    "7",
				CommissionBps: "725",
			},
			{
				Pubkey:      "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
				Lamports:    -5000,
				PostBalance: 42,
				RewardType:  confirmed_block.RewardType_Fee,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal rewards: %v", err)
	}

	rewards, err := ParseRewards(buf)
	if err != nil {
		t.Fatalf("parse rewards: %v", err)
	}
	got, numPartitions, err := RewardsToUi(rewards)
	if err != nil {
		t.Fatalf("rewards to ui: %v", err)
	}
	if numPartitions != nil {
		t.Fatalf("unexpected numPartitions: %d", *numPartitions)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("encode rewards: %v", err)
	}
	const want = `[` +
		`{"pubkey":"3ZFkP99s4GvhibGHUtQnF14bnZfmpDmbDd8xeUV37mC8","lamports":81902744,"postBalance":279084185624,"rewardType":"DeactivatedStake","commission":null,"commissionBps":0},` +
		`{"pubkey":"GdnSyH3YtwcxFvQrVVJMm1JhTS4QVX7MFsX56uJLUfiZ","lamports":2500,"postBalance":100000,"rewardType":"Voting","commission":7,"commissionBps":725},` +
		`{"pubkey":"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM","lamports":-5000,"postBalance":42,"rewardType":"Fee","commission":null}` +
		`]`
	if string(encoded) != want {
		t.Fatalf("unexpected rewards json:\n got: %s\nwant: %s", encoded, want)
	}
}
