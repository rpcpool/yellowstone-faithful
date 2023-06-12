package solanablockrewards

import (
	"github.com/golang/protobuf/proto"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
)

func ParseRewards(buf []byte) (*confirmed_block.Rewards, error) {
	var rewards confirmed_block.Rewards
	err := proto.Unmarshal(buf, &rewards)
	if err != nil {
		return nil, err
	}
	return &rewards, nil
}
