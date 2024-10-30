package accum

import (
	"context"
	"fmt"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
)

type TransactionWithSlot struct {
	Offset      uint64
	Length      uint64
	Slot        uint64
	Blocktime   uint64
	Error       error
	Transaction solana.Transaction
	Metadata    *solanatxmetaparsers.TransactionStatusMetaContainer
}

// IsMetaNotFound returns true if the error is a not found error.
func (obj TransactionWithSlot) IsMetaNotFound() bool {
	e, ok := obj.Error.(txMetaError)
	if !ok {
		return false
	}
	return e.IsMetaNotFound()
}

// IsMetaParseError returns true if the error is a parsing error.
func (obj TransactionWithSlot) IsMetaParseError() bool {
	e, ok := obj.Error.(txMetaError)
	if !ok {
		return false
	}
	return e.IsMetaParseError()
}

// Ok returns true if the error is nil.
func (obj TransactionWithSlot) Ok() bool {
	return obj.Error == nil
}

type txMetaError struct {
	Sig          solana.Signature
	Err          error
	isNotFound   bool
	isParseError bool
}

func (obj txMetaError) Error() string {
	switch {
	case obj.isNotFound:
		return fmt.Sprintf("not found: %s", obj.Err)
	case obj.isParseError:
		return fmt.Sprintf("parse error: %s", obj.Err)
	default:
		return fmt.Sprintf("error: %s", obj.Err)
	}
}

func (obj txMetaError) Is(target error) bool {
	if _, ok := target.(txMetaError); ok {
		return true
	}
	return false
}

func (obj txMetaError) Unwrap() error {
	return obj.Err
}

func (obj txMetaError) IsMetaNotFound() bool {
	return obj.isNotFound
}

func (obj txMetaError) IsMetaParseError() bool {
	return obj.isParseError
}

func (obj txMetaError) Ok() bool {
	return obj.Err == nil
}

func newTxMetaErrorNotFound(sig solana.Signature, err error) txMetaError {
	return txMetaError{
		Sig:        sig,
		Err:        err,
		isNotFound: true,
	}
}

func newTxMetaErrorParseError(sig solana.Signature, err error) txMetaError {
	return txMetaError{
		Sig:          sig,
		Err:          err,
		isParseError: true,
	}
}

var poolOfTransactionWithSlotSlices = sync.Pool{
	New: func() interface{} {
		return make([]*TransactionWithSlot, 0, 1000)
	},
}

func getTransactionWithSlotSlice() []*TransactionWithSlot {
	return poolOfTransactionWithSlotSlices.Get().([]*TransactionWithSlot)
}

func PutTransactionWithSlotSlice(slice []*TransactionWithSlot) {
	slice = slice[:0]
	poolOfTransactionWithSlotSlices.Put(slice)
}

var poolDataBlocksMap = sync.Pool{
	New: func() interface{} {
		return make(map[string]ObjectWithMetadata, 0)
	},
}

func clearDataBlocksMap(m map[string]ObjectWithMetadata) {
	for k := range m {
		delete(m, k)
	}
}

func getDatablocksMap() map[string]ObjectWithMetadata {
	return poolDataBlocksMap.Get().(map[string]ObjectWithMetadata)
}

func putDataBlocksMap(m map[string]ObjectWithMetadata) {
	clearDataBlocksMap(m)
	poolDataBlocksMap.Put(m)
}

func ObjectsToTransactionsAndMetadata(
	block *ipldbindcode.Block,
	objects []ObjectWithMetadata,
) ([]*TransactionWithSlot, error) {
	transactions := getTransactionWithSlotSlice()
	dataBlocksMap := getDatablocksMap()
	defer putDataBlocksMap(dataBlocksMap)
	for objI := range objects {
		object := objects[objI]
		// check if the object is a transaction:
		kind := iplddecoders.Kind(object.ObjectData[1])
		if kind == iplddecoders.KindDataFrame {
			dataBlocksMap[object.Cid.String()] = object
			continue
		}
		if kind != iplddecoders.KindTransaction {
			continue
		}
		decodedTxObj, err := iplddecoders.DecodeTransaction(object.ObjectData)
		if err != nil {
			return nil, fmt.Errorf("error while decoding transaction from nodex %s: %w", object.Cid, err)
		}
		tws := &TransactionWithSlot{
			Offset:    object.Offset,
			Length:    object.SectionLength,
			Slot:      uint64(decodedTxObj.Slot),
			Blocktime: uint64(block.Meta.Blocktime),
		}
		tx, err := decodedTxObj.GetSolanaTransaction()
		if err != nil {
			return nil, fmt.Errorf("error while getting solana transaction from object %s: %w", object.Cid, err)
		}
		tws.Transaction = *tx
		sigs := tx.Signatures
		if len(sigs) == 0 {
			return nil, fmt.Errorf("transaction has no signatures: %s", object.Cid)
		}
		sig := sigs[0]

		if total, ok := decodedTxObj.Metadata.GetTotal(); !ok || total == 1 {
			// metadata fit into the transaction object:
			completeBuffer := decodedTxObj.Metadata.Bytes()
			if ha, ok := decodedTxObj.Metadata.GetHash(); ok {
				err := ipldbindcode.VerifyHash(completeBuffer, ha)
				if err != nil {
					return nil, fmt.Errorf("failed to verify metadata hash: %w", err)
				}
			}
			if len(completeBuffer) > 0 {
				uncompressedMeta, err := tooling.DecompressZstd(completeBuffer)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress metadata: %w", err)
				}
				status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
				if err == nil {
					tws.Metadata = status
				} else {
					tws.Error = newTxMetaErrorParseError(sig, err)
				}
			} else {
				tws.Error = newTxMetaErrorNotFound(sig, fmt.Errorf("metadata is empty"))
			}
			clearDataBlocksMap(dataBlocksMap)
		} else {
			// metadata didn't fit into the transaction object, and was split into multiple dataframes:
			metaBuffer, err := tooling.LoadDataFromDataFrames(
				&decodedTxObj.Metadata,
				func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
					if dataBlock, ok := dataBlocksMap[wantedCid.String()]; ok {
						df, err := iplddecoders.DecodeDataFrame(dataBlock.ObjectData)
						if err != nil {
							return nil, err
						}
						return df, nil
					}
					return nil, fmt.Errorf("dataframe not found")
				})
			if err != nil {
				return nil, fmt.Errorf("failed to load metadata: %w", err)
			}
			// clear dataBlocksMap so it can accumulate dataframes for the next transaction:
			clearDataBlocksMap(dataBlocksMap)

			// if we have a metadata buffer, try to decompress it:
			if len(metaBuffer) > 0 {
				uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress metadata: %w", err)
				}
				status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
				if err == nil {
					tws.Metadata = status
				} else {
					tws.Error = newTxMetaErrorParseError(sig, err)
				}
			} else {
				tws.Error = newTxMetaErrorNotFound(sig, fmt.Errorf("metadata is empty"))
			}
		}

		transactions = append(transactions, tws)
	}
	return transactions, nil
}
