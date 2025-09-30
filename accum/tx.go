package accum

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"golang.org/x/sync/errgroup"
)

type TransactionWithSlot struct {
	Offset         uint64
	Length         uint64
	Slot           uint64
	Blocktime      uint64
	Position       uint64 // Position in the block, used for sorting
	Error          error
	Transaction    *solana.Transaction
	Metadata       *solanatxmetaparsers.TransactionStatusMetaContainer
	MetadataPieces MetadataPieceSectionRefs // Used for multipiece metadata
}

// TransactionWithSlot.GetTotalOffsetAndLengthAndCount returns the total offset and length of the transaction and its metadata pieces.
func (obj TransactionWithSlot) GetTotalOffsetAndLengthAndCount() (uint64, uint64, int) {
	// assume the metadata comes before the transaction:
	// e.g. {piece}{piece}{piece}{transaction}
	totalOffset := obj.Offset
	totalLength := obj.Length
	if len(obj.MetadataPieces) > 0 {
		// if we have metadata pieces, we need to calculate the total offset and length:
		sort.Slice(obj.MetadataPieces, func(i, j int) bool {
			return obj.MetadataPieces[i].Offset < obj.MetadataPieces[j].Offset
		})
		totalOffset = obj.MetadataPieces[0].Offset
		totalLength = obj.Offset + obj.Length - totalOffset
	}
	return totalOffset, totalLength, 1 + len(obj.MetadataPieces) // transaction + metadata pieces
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
		made := make([]*TransactionWithSlot, 0, 1000)
		return &made
	},
}

func getTransactionWithSlotSlice() []*TransactionWithSlot {
	got := poolOfTransactionWithSlotSlices.Get().(*[]*TransactionWithSlot)
	return *got
}

func PutTransactionWithSlotSlice(slice []*TransactionWithSlot) {
	slice = slice[:0]
	poolOfTransactionWithSlotSlices.Put(&slice)
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
	defer func() {
		clearDataBlocksMap(dataBlocksMap)
		putDataBlocksMap(dataBlocksMap)
	}()
	for objI := range objects {
		object := objects[objI]
		objectData := object.ObjectData.Bytes()
		// check if the object is a transaction:
		kind, err := iplddecoders.GetKind(objectData)
		if err != nil {
			return nil, fmt.Errorf("error while getting kind from object %s: %w", object.Cid, err)
		}
		if kind == iplddecoders.KindDataFrame {
			dataBlocksMap[object.Cid.String()] = object
			continue
		}
	}
	wg := new(errgroup.Group)
	mu := &sync.Mutex{}
	for objI := range objects {
		wg.Go(func() error {
			object := objects[objI]
			objectData := object.ObjectData.Bytes()
			// check if the object is a transaction:
			kind := iplddecoders.Kind(objectData[1])
			if kind != iplddecoders.KindTransaction {
				// not a transaction, skip it:
				return nil
			}
			decodedTxObj, err := iplddecoders.DecodeTransaction(objectData)
			if err != nil {
				return fmt.Errorf("error while decoding transaction from nodex %s: %w", object.Cid, err)
			}
			defer iplddecoders.PutTransaction(decodedTxObj)
			tws := &TransactionWithSlot{
				Offset:    object.Offset,
				Length:    object.SectionLength,
				Slot:      uint64(decodedTxObj.Slot),
				Blocktime: uint64(block.Meta.Blocktime),
			}
			pos, ok := decodedTxObj.GetPositionIndex()
			if ok {
				tws.Position = uint64(pos)
			} else {
				tws.Position = uint64(objI) // fallback to the index in the objects slice
			}
			tx, err := decodedTxObj.GetSolanaTransaction()
			if err != nil {
				return fmt.Errorf("error while getting solana transaction from object %s: %w", object.Cid, err)
			}
			tws.Transaction = tx
			sigs := tx.Signatures
			if len(sigs) == 0 {
				return fmt.Errorf("transaction has no signatures: %s", object.Cid)
			}
			sig := sigs[0]

			if total, ok := decodedTxObj.Metadata.GetTotal(); !ok || total == 1 {
				// metadata fit into the transaction object:
				completeBuffer := decodedTxObj.Metadata.Bytes()
				if ha, ok := decodedTxObj.Metadata.GetHash(); ok {
					err := ipldbindcode.VerifyHash(completeBuffer, ha)
					if err != nil {
						return fmt.Errorf("failed to verify metadata hash: %w", err)
					}
				}
				if len(completeBuffer) > 0 {
					uncompressedMeta, err := tooling.DecompressZstd(completeBuffer)
					if err != nil {
						return fmt.Errorf("failed to decompress metadata: %w", err)
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
			} else {
				// metadata didn't fit into the transaction object, and was split into multiple dataframes:
				metaBuffer, err := ipldbindcode.LoadDataFromDataFrames(
					&decodedTxObj.Metadata,
					func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
						if dataBlock, ok := dataBlocksMap[wantedCid.String()]; ok {
							df, err := iplddecoders.DecodeDataFrame(dataBlock.ObjectData.Bytes())
							if err != nil {
								return nil, err
							}
							tws.MetadataPieces = append(tws.MetadataPieces, MetadataPieceSectionRef{
								Offset: dataBlock.Offset,
								Length: dataBlock.SectionLength,
							})
							return df, nil
						}
						return nil, fmt.Errorf("dataframe not found")
					})
				if err != nil {
					return fmt.Errorf("failed to load metadata: %w", err)
				}

				// if we have a metadata buffer, try to decompress it:
				if len(metaBuffer) > 0 {
					uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
					if err != nil {
						return fmt.Errorf("failed to decompress metadata: %w", err)
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

			mu.Lock()
			transactions = append(transactions, tws)
			mu.Unlock()
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return nil, fmt.Errorf("error while processing transactions: %w", err)
	}
	sort.Slice(transactions, func(i, j int) bool {
		if transactions[i].Slot == transactions[j].Slot {
			return transactions[i].Position < transactions[j].Position
		}
		return transactions[i].Slot < transactions[j].Slot
	})
	return transactions, nil
}
