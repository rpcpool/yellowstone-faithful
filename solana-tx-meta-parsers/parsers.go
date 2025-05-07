package solanatxmetaparsers

import (
	"errors"
	"fmt"

	serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

type TransactionStatusMetaContainer struct {
	vProtobuf *confirmed_block.TransactionStatusMeta
	vSerde    *serde_agave.TransactionStatusMeta
}

// Ok returns true if the container holds a value.
func (c *TransactionStatusMetaContainer) Ok() bool {
	return c.vProtobuf != nil || c.vSerde != nil
}

// IsEmpty returns true if the container holds no value.
func (c *TransactionStatusMetaContainer) IsEmpty() bool {
	return !c.Ok()
}

// IsProtobuf returns true if the contained value is a protobuf.
func (c *TransactionStatusMetaContainer) IsProtobuf() bool {
	return c.vProtobuf != nil
}

// IsSerde returns true if the contained value is the latest serde format.
func (c *TransactionStatusMetaContainer) IsSerde() bool {
	return c.vSerde != nil
}

// GetProtobuf returns the contained protobuf value.
func (c *TransactionStatusMetaContainer) GetProtobuf() *confirmed_block.TransactionStatusMeta {
	return c.vProtobuf
}

// GetSerde returns the contained latest serde format value.
func (c *TransactionStatusMetaContainer) GetSerde() *serde_agave.TransactionStatusMeta {
	return c.vSerde
}

func (c *TransactionStatusMetaContainer) GetLoadedAccounts() [][]byte {
	if c.vProtobuf != nil {
		return append(
			c.vProtobuf.LoadedReadonlyAddresses,
			c.vProtobuf.LoadedWritableAddresses...,
		)
	}
	if c.vSerde != nil {
		return append(
			serdePubkeySliceToBytesSlice(c.vSerde.LoadedAddresses.Readonly),
			serdePubkeySliceToBytesSlice(c.vSerde.LoadedAddresses.Writable)...,
		)
	}
	return nil
}

func serdePubkeySliceToBytesSlice(serdePubkeys []serde_agave.Pubkey) [][]byte {
	bytesSlice := make([][]byte, len(serdePubkeys))
	for i, pubkey := range serdePubkeys {
		bytesSlice[i] = pubkey[:]
	}
	return bytesSlice
}

func ParseTransactionStatusMeta(buf []byte) (*confirmed_block.TransactionStatusMeta, error) {
	var status confirmed_block.TransactionStatusMeta
	err := proto.Unmarshal(buf, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func ParseTransactionStatusMeta_Serde(buf []byte) (*serde_agave.TransactionStatusMeta, error) {
	legacyStatus, err := serde_agave.BincodeDeserializeTransactionStatusMeta(buf)
	if err != nil {
		return nil, err
	}
	return &legacyStatus, nil
}

func ParseAnyTransactionStatusMeta(buf []byte) (any, error) {
	errs := make([]error, 0)
	// try to parse as protobuf (latest format)
	asProtobuf, err := ParseTransactionStatusMeta(buf)
	if err == nil {
		return asProtobuf, nil
	}
	errs = append(errs, err)
	// try to parse as legacy serde format
	asSerde, err := ParseTransactionStatusMeta_Serde(buf)
	if err == nil {
		return asSerde, nil
	}
	errs = append(errs, err)
	return nil, fmt.Errorf("failed to parse tx meta: %w", errors.Join(errs...))
}

// ParseTransactionStatusMetaContainer parses the transaction status meta from the given bytes.
// It tries to parse the bytes as the latest protobuf format, then the latest serde format, and finally the oldest serde format.
// It returns a container that holds the parsed value.
func ParseTransactionStatusMetaContainer(buf []byte) (*TransactionStatusMetaContainer, error) {
	whatever, err := ParseAnyTransactionStatusMeta(buf)
	if err != nil {
		return nil, err
	}
	container := &TransactionStatusMetaContainer{}
	switch val := whatever.(type) {
	case *confirmed_block.TransactionStatusMeta:
		container.vProtobuf = val
	case *serde_agave.TransactionStatusMeta:
		container.vSerde = val
	}
	return container, nil
}
