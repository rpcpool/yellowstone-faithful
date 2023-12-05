package main

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	metalatest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-latest"
	metaoldest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-oldest"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/sync/errgroup"
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

func countSignatures(v map[uint64][]solana.Signature) int {
	var count int
	for _, sigs := range v {
		count += len(sigs)
	}
	return count
}

func (multi *MultiEpoch) handleGetSignaturesForAddress(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// TODO:
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

	// Get the signatures:
	foundSignatures, err := gsfaMulti.GetBeforeUntil(
		ctx,
		pk,
		limit,
		params.Before,
		params.Until,
	)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get signatures: %w", err)
	}

	if len(foundSignatures) == 0 {
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

	var blockTimeCache struct {
		m  map[uint64]uint64
		mu sync.Mutex
	}
	blockTimeCache.m = make(map[uint64]uint64)
	getBlockTime := func(slot uint64, ser *Epoch) uint64 {
		blockTimeCache.mu.Lock()
		defer blockTimeCache.mu.Unlock()
		if blockTime, ok := blockTimeCache.m[slot]; ok {
			return blockTime
		}
		block, err := ser.GetBlock(ctx, slot)
		if err != nil {
			klog.Errorf("failed to get block time for slot %d: %v", slot, err)
			return 0
		}
		blockTimeCache.m[slot] = uint64(block.Meta.Blocktime)
		return uint64(block.Meta.Blocktime)
	}

	wg := new(errgroup.Group)
	wg.SetLimit(runtime.NumCPU() * 2)
	// The response is an array of objects: [{signature: string}]
	response := make([]map[string]any, countSignatures(foundSignatures))
	numBefore := 0
	for ei := range foundSignatures {
		epoch := ei
		ser, err := multi.GetEpoch(epoch)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get epoch %d: %w", epoch, err)
		}

		sigs := foundSignatures[ei]
		for i := range sigs {
			ii := numBefore + i
			sig := sigs[i]
			wg.Go(func() error {
				response[ii] = map[string]any{
					"signature": sig.String(),
				}
				if signaturesOnly {
					return nil
				}
				transactionNode, err := ser.GetTransaction(ctx, sig)
				if err != nil {
					klog.Errorf("failed to get tx %s: %v", sig, err)
					return nil
				}
				if transactionNode != nil {
					{
						tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, ser.GetDataFrameByCid)
						if err == nil {
							switch metaValue := meta.(type) {
							case *confirmed_block.TransactionStatusMeta:
								response[ii]["err"] = metaValue.Err
							case *metalatest.TransactionStatusMeta:
								response[ii]["err"] = metaValue.Status
							case *metaoldest.TransactionStatusMeta:
								response[ii]["err"] = metaValue.Status
							}

							if _, ok := response[ii]["err"]; ok {
								response[ii]["err"], _ = parseTransactionError(response[ii]["err"])
							}

							memoData := getMemoInstructionDataFromTransaction(&tx)
							if memoData != nil {
								response[ii]["memo"] = string(memoData)
							}
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
			})
		}
		numBefore += len(sigs)
	}
	if err := wg.Wait(); err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to get tx data: %w", err)
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
