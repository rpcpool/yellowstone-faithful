package solanatxmetaparsers

import (
	"fmt"

	metalatest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-latest"
	metaoldest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-oldest"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

func ParseTransactionStatusMeta(buf []byte) (*confirmed_block.TransactionStatusMeta, error) {
	var status confirmed_block.TransactionStatusMeta
	err := proto.Unmarshal(buf, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// From https://github.com/solana-labs/solana/blob/ce598c5c98e7384c104fe7f5121e32c2c5a2d2eb/transaction-status/src/lib.rs#L140-L147
func ParseLegacyTransactionStatusMeta(buf []byte) (*metalatest.TransactionStatusMeta, error) {
	legacyStatus, err := metalatest.BincodeDeserializeTransactionStatusMeta(buf)
	if err != nil {
		return nil, err
	}
	return &legacyStatus, nil
}

// From https://github.com/solana-labs/solana/blob/b7b4aa5d4d34ebf3fd338a64f4f2a5257b047bb4/transaction-status/src/lib.rs#L22-L27
func ParseLegacyTransactionStatusMetaOldest(buf []byte) (*metaoldest.TransactionStatusMeta, error) {
	legacyStatus, err := metaoldest.BincodeDeserializeTransactionStatusMeta(buf)
	if err != nil {
		return nil, err
	}
	return &legacyStatus, nil
}

func ParseAnyTransactionStatusMeta(buf []byte) (any, error) {
	// try to parse as protobuf (latest format)
	status, err := ParseTransactionStatusMeta(buf)
	if err == nil {
		return status, nil
	}
	// try to parse as legacy serde format (last serde format used by solana)
	status2, err := ParseLegacyTransactionStatusMeta(buf)
	if err == nil {
		return status2, nil
	}
	// try to parse as legacy serde format (probably the oldest serde format used by solana)
	status1, err := ParseLegacyTransactionStatusMetaOldest(buf)
	if err == nil {
		return status1, nil
	}
	return nil, fmt.Errorf("failed to parse tx meta: %w", err)
}
