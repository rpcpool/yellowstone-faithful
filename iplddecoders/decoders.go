package iplddecoders

import (
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
)

type Kind int

const (
	KindTransaction Kind = iota
	KindEntry
	KindBlock
	KindSubset
	KindEpoch
	KindRewards
	KindDataFrame
)

type KindSlice []Kind

func (ks KindSlice) Has(k Kind) bool {
	for _, kind := range ks {
		if kind == k {
			return true
		}
	}
	return false
}

func (ks KindSlice) HasAny(kinds ...Kind) bool {
	for _, kind := range kinds {
		if ks.Has(kind) {
			return true
		}
	}
	return false
}

// String returns the string representation of the Kind.
func (k Kind) String() string {
	switch k {
	case KindTransaction:
		return "Transaction"
	case KindEntry:
		return "Entry"
	case KindBlock:
		return "Block"
	case KindSubset:
		return "Subset"
	case KindEpoch:
		return "Epoch"
	case KindRewards:
		return "Rewards"
	case KindDataFrame:
		return "DataFrame"
	default:
		return fmt.Sprintf("Unknown kind %d", int(k))
	}
}

func DecodeEpoch(epochRaw []byte) (*ipldbindcode.Epoch, error) {
	return _DecodeEpochFast(epochRaw)
}

func _DecodeEpochFast(epochRaw []byte) (*ipldbindcode.Epoch, error) {
	epoch := GetEpoch()
	err := epoch.UnmarshalCBOR(epochRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Epoch node: %w", err)
	}
	if epoch.Kind != int(KindEpoch) {
		return nil, fmt.Errorf("expected Epoch node, got %s", Kind(epoch.Kind))
	}
	return epoch, nil
}

func _DecodeEpochClassic(epochRaw []byte) (*ipldbindcode.Epoch, error) {
	var epoch ipldbindcode.Epoch
	_, err := ipld.Unmarshal(epochRaw, dagcbor.Decode, &epoch, ipldbindcode.Prototypes.Epoch.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Epoch node: %w", err)
	}
	if epoch.Kind != int(KindEpoch) {
		return nil, fmt.Errorf("expected Epoch node, got %s", Kind(epoch.Kind))
	}
	return &epoch, nil
}

func DecodeSubset(subsetRaw []byte) (*ipldbindcode.Subset, error) {
	return _DecodeSubsetFast(subsetRaw)
}

func _DecodeSubsetFast(subsetRaw []byte) (*ipldbindcode.Subset, error) {
	subset := GetSubset()
	err := subset.UnmarshalCBOR(subsetRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Subset node: %w", err)
	}
	if subset.Kind != int(KindSubset) {
		return nil, fmt.Errorf("expected Subset node, got %s", Kind(subset.Kind))
	}
	return subset, nil
}

func _DecodeSubsetClassic(subsetRaw []byte) (*ipldbindcode.Subset, error) {
	var subset ipldbindcode.Subset
	_, err := ipld.Unmarshal(subsetRaw, dagcbor.Decode, &subset, ipldbindcode.Prototypes.Subset.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Subset node: %w", err)
	}
	if subset.Kind != int(KindSubset) {
		return nil, fmt.Errorf("expected Subset node, got %s", Kind(subset.Kind))
	}
	return &subset, nil
}

func DecodeBlock(blockRaw []byte) (*ipldbindcode.Block, error) {
	return _DecodeBlockFast(blockRaw)
}

func _DecodeBlockFast(blockRaw []byte) (*ipldbindcode.Block, error) {
	block := GetBlock()
	err := block.UnmarshalCBOR(blockRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Block node: %w", err)
	}
	if block.Kind != int(KindBlock) {
		return nil, fmt.Errorf("expected Block node, got %s", Kind(block.Kind))
	}
	return block, nil
}

func _DecodeBlockClassic(blockRaw []byte) (*ipldbindcode.Block, error) {
	var block ipldbindcode.Block
	_, err := ipld.Unmarshal(blockRaw, dagcbor.Decode, &block, ipldbindcode.Prototypes.Block.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Block node: %w", err)
	}
	if block.Kind != int(KindBlock) {
		return nil, fmt.Errorf("expected Block node, got %s", Kind(block.Kind))
	}
	return &block, nil
}

func DecodeEntry(entryRaw []byte) (*ipldbindcode.Entry, error) {
	return _DecodeEntryFast(entryRaw)
}

func _DecodeEntryFast(entryRaw []byte) (*ipldbindcode.Entry, error) {
	entry := GetEntry()
	err := entry.UnmarshalCBOR(entryRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Entry node: %w", err)
	}
	if entry.Kind != int(KindEntry) {
		return nil, fmt.Errorf("expected Entry node, got %s", Kind(entry.Kind))
	}
	return entry, nil
}

func _DecodeEntryClassic(entryRaw []byte) (*ipldbindcode.Entry, error) {
	var entry ipldbindcode.Entry
	_, err := ipld.Unmarshal(entryRaw, dagcbor.Decode, &entry, ipldbindcode.Prototypes.Entry.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Entry node: %w", err)
	}
	if entry.Kind != int(KindEntry) {
		return nil, fmt.Errorf("expected Entry node, got %s", Kind(entry.Kind))
	}
	return &entry, nil
}

func DecodeTransaction(transactionRaw []byte) (*ipldbindcode.Transaction, error) {
	return _DecodeTransactionFast(transactionRaw)
}

func _DecodeTransactionFast(transactionRaw []byte) (*ipldbindcode.Transaction, error) {
	transaction := GetTransaction()
	err := transaction.UnmarshalCBOR(transactionRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Transaction node: %w", err)
	}
	if transaction.Kind != int(KindTransaction) {
		return nil, fmt.Errorf("expected Transaction node, got %s", Kind(transaction.Kind))
	}
	return transaction, nil
}

func _DecodeTransactionClassic(transactionRaw []byte) (*ipldbindcode.Transaction, error) {
	var transaction ipldbindcode.Transaction
	_, err := ipld.Unmarshal(transactionRaw, dagcbor.Decode, &transaction, ipldbindcode.Prototypes.Transaction.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Transaction node: %w", err)
	}
	if transaction.Kind != int(KindTransaction) {
		return nil, fmt.Errorf("expected Transaction node, got %s", Kind(transaction.Kind))
	}
	return &transaction, nil
}

func DecodeRewards(rewardsRaw []byte) (*ipldbindcode.Rewards, error) {
	return _DecodeRewardsFast(rewardsRaw)
}

func _DecodeRewardsFast(rewardsRaw []byte) (*ipldbindcode.Rewards, error) {
	rewards := GetRewards()
	err := rewards.UnmarshalCBOR(rewardsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Rewards node: %w", err)
	}
	if rewards.Kind != int(KindRewards) {
		return nil, fmt.Errorf("expected Rewards node, got %s", Kind(rewards.Kind))
	}
	return rewards, nil
}

func _DecodeRewardsClassic(rewardsRaw []byte) (*ipldbindcode.Rewards, error) {
	var rewards ipldbindcode.Rewards
	_, err := ipld.Unmarshal(rewardsRaw, dagcbor.Decode, &rewards, ipldbindcode.Prototypes.Rewards.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Rewards node: %w", err)
	}
	if rewards.Kind != int(KindRewards) {
		return nil, fmt.Errorf("expected Rewards node, got %s", Kind(rewards.Kind))
	}
	return &rewards, nil
}

func DecodeDataFrame(dataFrameRaw []byte) (*ipldbindcode.DataFrame, error) {
	return _DecodeDataFrameFast(dataFrameRaw)
}

func _DecodeDataFrameFast(dataFrameRaw []byte) (*ipldbindcode.DataFrame, error) {
	dataFrame := GetDataFrame()
	err := dataFrame.UnmarshalCBOR(dataFrameRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DataFrame node: %w", err)
	}
	if dataFrame.Kind != int(KindDataFrame) {
		return nil, fmt.Errorf("expected DataFrame node, got %s", Kind(dataFrame.Kind))
	}
	return dataFrame, nil
}

func _DecodeDataFrameClassic(dataFrameRaw []byte) (*ipldbindcode.DataFrame, error) {
	var dataFrame ipldbindcode.DataFrame
	_, err := ipld.Unmarshal(dataFrameRaw, dagcbor.Decode, &dataFrame, ipldbindcode.Prototypes.DataFrame.Type())
	if err != nil {
		return nil, fmt.Errorf("failed to decode DataFrame node: %w", err)
	}
	if dataFrame.Kind != int(KindDataFrame) {
		return nil, fmt.Errorf("expected DataFrame node, got %s", Kind(dataFrame.Kind))
	}
	return &dataFrame, nil
}

func DecodeAny(anyRaw []byte) (ipldbindcode.Node, error) {
	kind, err := GetKind(anyRaw)
	if err != nil {
		return nil, err
	}

	switch kind {
	case KindTransaction:
		return DecodeTransaction(anyRaw)
	case KindEntry:
		return DecodeEntry(anyRaw)
	case KindBlock:
		return DecodeBlock(anyRaw)
	case KindSubset:
		return DecodeSubset(anyRaw)
	case KindEpoch:
		return DecodeEpoch(anyRaw)
	case KindRewards:
		return DecodeRewards(anyRaw)
	case KindDataFrame:
		return DecodeDataFrame(anyRaw)
	default:
		return nil, fmt.Errorf("unknown kind %d", int(kind))
	}
}

func GetKind(anyRaw []byte) (Kind, error) {
	if len(anyRaw) == 0 {
		return Kind(-1), fmt.Errorf("empty bytes")
	}
	if len(anyRaw) < 2 {
		return Kind(-1), fmt.Errorf("not enough bytes")
	}
	kind := Kind(anyRaw[1])
	return kind, nil
}
