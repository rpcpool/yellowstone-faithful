package nodetools

import (
	"context"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	ytooling "github.com/rpcpool/yellowstone-faithful/tooling"
	txpool "github.com/rpcpool/yellowstone-faithful/tx-pool"
	"k8s.io/klog/v2"
)

func GetParsedRewards(parsedDag ParsedAndCidSlice, rewardsCid cid.Cid) (*confirmed_block.Rewards, error) {
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

func GetTransactionAndMeta(
	parsedDag ParsedAndCidSlice,
	txNode *ipldbindcode.Transaction,
) (*solana.Transaction, *solanatxmetaparsers.TransactionStatusMetaContainer, error) {
	tx, meta, err := ParseTransactionAndMetaFromNode(txNode, func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
		df, err := parsedDag.DataFrameByCid(wantedCid)
		if err != nil {
			return nil, fmt.Errorf("failed to get DataFrame by CID %s: %w", wantedCid, err)
		}
		return df, nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse transaction and meta from node: %w", err)
	}
	return tx, meta, nil
}

func ParseTransactionAndMetaFromNode(
	transactionNode *ipldbindcode.Transaction,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) (tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, _ error) {
	{
		transactionBuffer, err := ipldbindcode.LoadDataFromDataFrames(&transactionNode.Data, dataFrameGetter)
		if err != nil {
			return nil, nil, err
		}
		tx = txpool.Get()
		if err := bin.UnmarshalBin(tx, transactionBuffer); err != nil {
			klog.Errorf("failed to unmarshal transaction: %v", err)
			return nil, nil, err
		} else if len(tx.Signatures) == 0 {
			klog.Errorf("transaction has no signatures")
			return nil, nil, err
		}
	}

	{
		metaBuffer, err := ipldbindcode.LoadDataFromDataFrames(&transactionNode.Metadata, dataFrameGetter)
		if err != nil {
			return nil, nil, err
		}
		if len(metaBuffer) > 0 {
			uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
			if err != nil {
				klog.Errorf("failed to decompress metadata: %v", err)
				return nil, nil, err
			}
			status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
			if err != nil {
				klog.Errorf("failed to parse metadata: %v", err)
				return nil, nil, err
			}
			meta = status
		}
	}
	return
}
