package main

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

// getGsfaReadersInEpochDescendingOrder returns a list of gsfa readers in epoch order (from most recent to oldest).
func (ser *MultiEpoch) getGsfaReadersInEpochDescendingOrder() ([]*gsfa.GsfaReader, []uint64) {
	ser.mu.RLock()
	defer ser.mu.RUnlock()

	epochs := make([]*Epoch, 0, len(ser.epochs))
	for _, epoch := range ser.epochs {
		epochs = append(epochs, epoch)
	}

	// sort epochs by epoch number (from biggest to smallest):
	sort.Slice(epochs, func(i, j int) bool {
		return epochs[i].epoch > epochs[j].epoch
	})

	gsfaReaders := make([]*gsfa.GsfaReader, 0, len(epochs))
	epochNums := make([]uint64, 0, len(epochs))
	for _, epoch := range epochs {
		if epoch.gsfaReader != nil {
			epoch.gsfaReader.SetEpoch(epoch.Epoch())
			gsfaReaders = append(gsfaReaders, epoch.gsfaReader)
			epochNums = append(epochNums, epoch.Epoch())
		}
	}
	return gsfaReaders, epochNums
}

func countTransactions(v gsfa.EpochToTransactionObjects) int {
	var count int
	for _, txs := range v {
		count += len(txs)
	}
	return count
}

func (multi *MultiEpoch) handleGetSignaturesForAddress(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// - parse and validate request
	// - get list of epochs (from most recent to oldest)
	// - iterate until we find the requested number of signatures
	// - expand the signatures with tx data
	signaturesOnly := multi.options.GsfaOnlySignatures

	params, err := parseGetSignaturesForAddressParams(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %v", err)
	}
	pk := params.Address
	limit := params.Limit

	gsfaIndexes, _ := multi.getGsfaReadersInEpochDescendingOrder()
	if len(gsfaIndexes) == 0 {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "getSignaturesForAddress method is not enabled",
		}, fmt.Errorf("no gsfa indexes found")
	}

	gsfaMulti, err := gsfa.NewGsfaReaderMultiepoch(gsfaIndexes)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to create gsfa multiepoch reader: %w", err)
	}

	var blockTimeCache struct {
		m  map[uint64]uint64
		mu sync.Mutex
	}
	blockTimeCache.m = make(map[uint64]uint64)
	getBlockTime := func(slot uint64, ser *Epoch) uint64 {
		// NOTE: this means that you have to potentially fetch 1k blocks to get the blocktime for each transaction.
		// TODO: include blocktime into the transaction data, or in the gsfaindex.
		// return 0
		blockTimeCache.mu.Lock()
		defer blockTimeCache.mu.Unlock()
		if blockTime, ok := blockTimeCache.m[slot]; ok {
			return blockTime
		}
		block, _, err := ser.GetBlock(ctx, slot)
		if err != nil {
			klog.Errorf("failed to get block time for slot %d: %v", slot, err)
			return 0
		}
		blockTimeCache.m[slot] = uint64(block.Meta.Blocktime)
		return uint64(block.Meta.Blocktime)
	}

	// Get the transactions:
	foundTransactions, err := gsfaMulti.GetBeforeUntil(
		ctx,
		pk,
		limit,
		params.Before,
		params.Until,
		func(epochNum uint64, oas linkedlog.OffsetAndSizeAndBlocktime) (*ipldbindcode.Transaction, error) {
			epoch, err := multi.GetEpoch(epochNum)
			if err != nil {
				return nil, fmt.Errorf("failed to get epoch %d: %w", epochNum, err)
			}
			raw, err := epoch.GetNodeByOffsetAndSize(ctx, nil, &indexes.OffsetAndSize{
				Offset: oas.Offset,
				Size:   oas.Size,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get signature: %w", err)
			}
			decoded, err := iplddecoders.DecodeTransaction(raw)
			if err != nil {
				return nil, fmt.Errorf("error while decoding transaction from nodex at offset %d: %w", oas.Offset, err)
			}
			blockTimeCache.m[uint64(decoded.Slot)] = uint64(oas.Blocktime)
			return decoded, nil
		},
	)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get signatures: %w", err)
	}

	if len(foundTransactions) == 0 {
		err = conn.ReplyRaw(
			ctx,
			req.ID,
			[]map[string]any{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to reply: %w", err)
		}
		return nil, nil
	}

	// The response is an array of objects: [{signature: string}]
	response := make([]map[string]any, countTransactions(foundTransactions))
	numBefore := 0
	for ei := range foundTransactions {
		epoch := ei
		ser, err := multi.GetEpoch(epoch)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get epoch %d: %w", epoch, err)
		}

		sigs := foundTransactions[ei]
		for i := range sigs {
			ii := numBefore + i
			transactionNode := sigs[i]
			err := func() error {
				sig, err := transactionNode.Signature()
				if err != nil {
					klog.Errorf("failed to get signature: %v", err)
					return nil
				}
				response[ii] = map[string]any{
					"signature": sig.String(),
				}
				if signaturesOnly {
					return nil
				}

				{
					{
						tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, ser.GetDataFrameByCid)
						if err == nil {
							response[ii]["err"] = getErr(meta)

							memoData := getMemoInstructionDataFromTransaction(&tx)
							if memoData != nil {
								response[ii]["memo"] = string(memoData)
							}
						} else {
							klog.Errorf("failed to parse transaction and meta for signature %s: %v", sig, err)
						}

						if _, ok := response[ii]["memo"]; !ok {
							response[ii]["memo"] = nil
						}
						if _, ok := response[ii]["err"]; !ok {
							response[ii]["err"] = nil
						}
					}
					slot := uint64(transactionNode.Slot)
					response[ii]["slot"] = slot
					if blockTime := getBlockTime(slot, ser); blockTime != 0 {
						response[ii]["blockTime"] = blockTime
					} else {
						response[ii]["blockTime"] = nil
					}
					response[ii]["confirmationStatus"] = "finalized"
				}
				return nil
			}()
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to get tx data: %w", err)
			}
		}
		numBefore += len(sigs)
	}
	// reply with the data
	err = conn.ReplyRaw(
		ctx,
		req.ID,
		response,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}

	return nil, nil
}
