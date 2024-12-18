package solanatxmetaparsers

import (
	"fmt"

	metalatest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-latest"
	metaoldest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-oldest"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

type TransactionStatusMetaContainer struct {
	vProtobuf    *confirmed_block.TransactionStatusMeta
	vSerdeLatest *metalatest.TransactionStatusMeta
	vSerdeOldest *metaoldest.TransactionStatusMeta
}

// Ok returns true if the container holds a value.
func (c *TransactionStatusMetaContainer) Ok() bool {
	return c.vProtobuf != nil || c.vSerdeLatest != nil || c.vSerdeOldest != nil
}

// IsEmpty returns true if the container holds no value.
func (c *TransactionStatusMetaContainer) IsEmpty() bool {
	return !c.Ok()
}

// IsProtobuf returns true if the contained value is a protobuf.
func (c *TransactionStatusMetaContainer) IsProtobuf() bool {
	return c.vProtobuf != nil
}

// IsSerdeLatest returns true if the contained value is the latest serde format.
func (c *TransactionStatusMetaContainer) IsSerdeLatest() bool {
	return c.vSerdeLatest != nil
}

// IsSerdeOldest returns true if the contained value is the oldest serde format.
func (c *TransactionStatusMetaContainer) IsSerdeOldest() bool {
	return c.vSerdeOldest != nil
}

// GetProtobuf returns the contained protobuf value.
func (c *TransactionStatusMetaContainer) GetProtobuf() *confirmed_block.TransactionStatusMeta {
	return c.vProtobuf
}

// GetSerdeLatest returns the contained latest serde format value.
func (c *TransactionStatusMetaContainer) GetSerdeLatest() *metalatest.TransactionStatusMeta {
	return c.vSerdeLatest
}

// GetSerdeOldest returns the contained oldest serde format value.
func (c *TransactionStatusMetaContainer) GetSerdeOldest() *metaoldest.TransactionStatusMeta {
	return c.vSerdeOldest
}

func (c *TransactionStatusMetaContainer) GetLoadedAccounts() [][]byte {
	if c.vProtobuf != nil {
		return append(c.vProtobuf.LoadedReadonlyAddresses, c.vProtobuf.LoadedWritableAddresses...)
	}
	return nil
}

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

// ParseTransactionStatusMetaContainer parses the transaction status meta from the given bytes.
// It tries to parse the bytes as the latest protobuf format, then the latest serde format, and finally the oldest serde format.
// It returns a container that holds the parsed value.
func ParseTransactionStatusMetaContainer(buf []byte) (*TransactionStatusMetaContainer, error) {
	any, err := ParseAnyTransactionStatusMeta(buf)
	if err != nil {
		return nil, err
	}
	container := &TransactionStatusMetaContainer{}
	switch v := any.(type) {
	case *confirmed_block.TransactionStatusMeta:
		container.vProtobuf = v
	case *metalatest.TransactionStatusMeta:
		container.vSerdeLatest = v
	case *metaoldest.TransactionStatusMeta:
		container.vSerdeOldest = v
	}
	return container, nil
}
