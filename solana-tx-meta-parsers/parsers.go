package solanatxmetaparsers

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gagliardetto/solana-go"
	serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"google.golang.org/protobuf/proto"
)

type TransactionStatusMetaContainer struct {
	vProtobuf *confirmed_block.TransactionStatusMeta
	vSerde    *serde_agave.StoredTransactionStatusMeta
}

// HasMeta returns true if the container holds a value.
func (c *TransactionStatusMetaContainer) HasMeta() bool {
	return c.vProtobuf != nil || c.vSerde != nil
}

// IsEmpty returns true if the container holds no value.
func (c *TransactionStatusMetaContainer) IsEmpty() bool {
	return !c.HasMeta()
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
func (c *TransactionStatusMetaContainer) GetSerde() *serde_agave.StoredTransactionStatusMeta {
	return c.vSerde
}

// IsErr whether the metadata tells us that the transaction failed.
func (c *TransactionStatusMetaContainer) IsErr() bool {
	if c.vProtobuf != nil {
		return c.vProtobuf.Err != nil
	}
	if c.vSerde != nil {
		if c.vSerde.Status == nil {
			return false
		}
		_, ok := c.vSerde.Status.(*serde_agave.Result__Err)
		return ok
	}
	return false
}

func (c *TransactionStatusMetaContainer) GetTxError() (transaction_status_meta_serde_agave.TransactionError, bool, error) {
	if c.vProtobuf != nil {
		if c.vProtobuf.Err == nil {
			return nil, false, nil
		}
		unmarshaledErr, err := transaction_status_meta_serde_agave.BincodeDeserializeResult(c.vProtobuf.Err.Err)
		if err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal error: %w", err)
		}
		if _, ok := unmarshaledErr.(*transaction_status_meta_serde_agave.Result__Ok); ok {
			return nil, false, nil
		}
		if e, ok := unmarshaledErr.(*transaction_status_meta_serde_agave.Result__Err); !ok {
			return nil, false, fmt.Errorf("unexpected error type: %T", unmarshaledErr)
		} else {
			return e.Value, true, nil
		}
	}
	if c.vSerde != nil {
		if c.vSerde.Status == nil {
			return nil, false, nil
		}
		if _, ok := c.vSerde.Status.(*transaction_status_meta_serde_agave.Result__Ok); ok {
			return nil, false, nil
		}
		if e, ok := c.vSerde.Status.(*transaction_status_meta_serde_agave.Result__Err); !ok {
			return nil, false, fmt.Errorf("unexpected error type: %T", c.vSerde.Status)
		} else {
			return e.Value, true, nil
		}
	}
	return nil, false, fmt.Errorf("no error found")
}

func (c *TransactionStatusMetaContainer) GetLoadedAccountsRaw() ([][]byte, [][]byte) {
	if c.vProtobuf != nil {
		return c.vProtobuf.LoadedWritableAddresses, c.vProtobuf.LoadedReadonlyAddresses
	}
	if c.vSerde != nil {
		return serdePubkeySliceToBytesSlice(c.vSerde.LoadedAddresses.Writable), serdePubkeySliceToBytesSlice(c.vSerde.LoadedAddresses.Readonly)
	}
	return nil, nil
}

func (c *TransactionStatusMetaContainer) GetLoadedAccounts() (solana.PublicKeySlice, solana.PublicKeySlice) {
	writable, readonly := c.GetLoadedAccountsRaw()
	writableKeys := make(solana.PublicKeySlice, len(writable))
	readonlyKeys := make(solana.PublicKeySlice, len(readonly))
	for i, pubkey := range writable {
		writableKeys[i] = solana.PublicKeyFromBytes(pubkey)
	}
	for i, pubkey := range readonly {
		readonlyKeys[i] = solana.PublicKeyFromBytes(pubkey)
	}
	return writableKeys, readonlyKeys
}

func serdePubkeySliceToBytesSlice(serdePubkeys []serde_agave.Pubkey) [][]byte {
	bytesSlice := make([][]byte, len(serdePubkeys))
	for i, pubkey := range serdePubkeys {
		bytesSlice[i] = pubkey[:]
	}
	return bytesSlice
}

var confirmed_blockTransactionStatusMetaPool = &sync.Pool{
	New: func() any {
		return &confirmed_block.TransactionStatusMeta{}
	},
}

func getConfirmedBlockTransactionStatusMeta() *confirmed_block.TransactionStatusMeta {
	if v := confirmed_blockTransactionStatusMetaPool.Get(); v != nil {
		return v.(*confirmed_block.TransactionStatusMeta)
	}
	return &confirmed_block.TransactionStatusMeta{}
}

func PutConfirmedBlockTransactionStatusMeta(meta *confirmed_block.TransactionStatusMeta) {
	if meta == nil {
		return
	}
	meta.Reset()
	{
		meta.Err = nil
		meta.Fee = 0
		meta.PreBalances = nil
		meta.PostBalances = nil
		meta.InnerInstructions = nil
		meta.InnerInstructionsNone = false
		meta.LogMessages = nil
		meta.LogMessagesNone = false
		meta.PreTokenBalances = nil
		meta.PostTokenBalances = nil
		meta.Rewards = nil
		meta.LoadedWritableAddresses = nil
		meta.LoadedReadonlyAddresses = nil
		meta.ReturnData = nil
		meta.ReturnDataNone = false
		meta.ComputeUnitsConsumed = nil
		meta.CostUnits = nil
	}
	confirmed_blockTransactionStatusMetaPool.Put(meta)
}

func ParseTransactionStatusMeta(buf []byte) (*confirmed_block.TransactionStatusMeta, error) {
	status := getConfirmedBlockTransactionStatusMeta()
	err := proto.Unmarshal(buf, status)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func ParseTransactionStatusMeta_Serde(buf []byte) (*serde_agave.StoredTransactionStatusMeta, error) {
	legacyStatus, err := serde_agave.BincodeDeserializeStoredTransactionStatusMeta(buf)
	if err != nil {
		if errors.Is(err, serde_agave.ErrSomeBytesNotRead) {
			return &legacyStatus, nil
		}
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
	errs = append(errs, fmt.Errorf("failed to parse protobuf: %w", err))
	// try to parse as legacy serde format
	asSerde, err := ParseTransactionStatusMeta_Serde(buf)
	if err == nil {
		return asSerde, nil
	}
	errs = append(errs, fmt.Errorf("failed to parse serde: %w", err))
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
	case *serde_agave.StoredTransactionStatusMeta:
		container.vSerde = val
	}
	return container, nil
}

// TransactionStatusMetaContainer.Put()
func (c *TransactionStatusMetaContainer) Put() {
	if c == nil {
		return
	}
	if c.vProtobuf != nil {
		PutConfirmedBlockTransactionStatusMeta(c.vProtobuf)
		c.vProtobuf = nil
	}
	if c.vSerde != nil {
		// TODO: Implement serde cleanup.
		// serde_agave.PutTransactionStatusMeta(c.vSerde)
		// c.vSerde = nil
	}
}
