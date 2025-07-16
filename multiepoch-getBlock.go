package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	jsoniter "github.com/json-iterator/go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"github.com/sourcegraph/jsonrpc2"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

var fasterJson = jsoniter.ConfigCompatibleWithStandardLibrary

type MyContextKey string

const requestIDKey = MyContextKey("requestID")

func setRequestIDToContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func getRequestIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

func (multi *MultiEpoch) handleGetBlock(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	// Start top-level span
	rpcSpanCtx, rpcSpan := telemetry.StartSpan(ctx, "jsonrpc.GetBlock")
	defer rpcSpan.End()

	tim := newTimer(getRequestIDFromContext(rpcSpanCtx))
	params, err := parseGetBlockRequest(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %w", err)
	}
	if err := params.Validate(); err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: err.Error(),
		}, fmt.Errorf("failed to validate params: %w", err)
	}
	tim.time("parseGetBlockRequest")
	slot := params.Slot

	// find the epoch that contains the requested slot
	epochNumber := slottools.CalcEpochForSlot(slot)
	_, epochLookupSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_EpochLookup")
	epochHandler, err := multi.GetEpoch(epochNumber)
	epochLookupSpan.End()
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available", epochNumber),
		}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
	}

	blockRetrievalCtx, blockRetrievalSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_GetBlockFromEpoch")
	block, blockCid, err := epochHandler.GetBlock(WithSubrapghPrefetch(blockRetrievalCtx, true), slot)
	blockRetrievalSpan.End()
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return &jsonrpc2.Error{
				Code:    CodeNotFound,
				Message: fmt.Sprintf("Slot %d was skipped, or missing in long-term storage", slot),
			}, err
		} else {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Failed to get block",
			}, fmt.Errorf("failed to get block: %w", err)
		}
	}
	// set the headers:
	{
		conn.ctx.Response.Header.Set("DAG-Root-CID", blockCid.String())
	}

	tim.time("GetBlock")
	{
		// Span for prefetch
		carPrefetchCtx, carPrefetchSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_CarPrefetch")
		prefetcherFromCar := func() error {
			parentIsInPreviousEpoch := slottools.CalcEpochForSlot(uint64(block.Meta.Parent_slot)) != slottools.CalcEpochForSlot(slot)
			if slot == 0 {
				parentIsInPreviousEpoch = true
			}
			if slot > 1 && block.Meta.Parent_slot == 0 {
				parentIsInPreviousEpoch = true
			}

			var blockCid, parentBlockCid cid.Cid
			wg := new(errgroup.Group)
			wg.Go(func() (err error) {
				blockCid, err = epochHandler.FindCidFromSlot(carPrefetchCtx, slot)
				if err != nil {
					return err
				}
				return nil
			})
			wg.Go(func() (err error) {
				if parentIsInPreviousEpoch {
					return nil
				}
				parentBlockCid, err = epochHandler.FindCidFromSlot(carPrefetchCtx, uint64(block.Meta.Parent_slot))
				if err != nil {
					return err
				}
				return nil
			})
			err = wg.Wait()
			if err != nil {
				return err
			}
			if slot == 0 {
				klog.V(4).Infof("car start to slot(0)::%s", blockCid)
			} else {
				klog.V(4).Infof(
					"slot(%d)::%s to slot(%d)::%s",
					uint64(block.Meta.Parent_slot),
					parentBlockCid,
					slot,
					blockCid,
				)
			}
			{
				var blockOffset, parentOffset uint64
				wg := new(errgroup.Group)
				wg.Go(func() (err error) {
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(carPrefetchCtx, blockCid)
					if err != nil {
						return err
					}
					blockOffset = offsetAndSize.Offset
					return nil
				})
				wg.Go(func() (err error) {
					if parentIsInPreviousEpoch {
						// get car file header size
						parentOffset = epochHandler.carHeaderSize
						return nil
					}
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(carPrefetchCtx, parentBlockCid)
					if err != nil {
						return err
					}
					parentOffset = offsetAndSize.Offset
					return nil
				})
				err = wg.Wait()
				if err != nil {
					return err
				}

				length := blockOffset - parentOffset
				MiB := uint64(1024 * 1024)
				maxPrefetchSize := MiB * 10 // let's cap prefetching size
				if length > maxPrefetchSize {
					length = maxPrefetchSize
				}

				start := parentOffset

				klog.V(4).Infof("prefetching CAR: start=%d length=%d (parent_offset=%d)", start, length, parentOffset)
				carSection, err := epochHandler.ReadAtFromCar(carPrefetchCtx, start, length)
				if err != nil {
					return err
				}
				dr := bytes.NewReader(carSection)
				br := bufio.NewReader(dr)

				gotCid, data, err := util.ReadNode(br)
				if err != nil {
					return fmt.Errorf("failed to read first node: %w", err)
				}
				if !parentIsInPreviousEpoch && !gotCid.Equals(parentBlockCid) {
					return fmt.Errorf("CID mismatch: expected %s, got %s", parentBlockCid, gotCid)
				}
				epochHandler.GetCache().PutRawCarObject(gotCid, data)

				for {
					gotCid, data, err = util.ReadNode(br)
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						return fmt.Errorf("failed to read node: %w", err)
					}
					if gotCid.Equals(blockCid) {
						break
					}
					epochHandler.GetCache().PutRawCarObject(gotCid, data)
				}
			}
			return nil
		}
		if epochHandler.lassieFetcher == nil {
			err := prefetcherFromCar()
			if err != nil {
				klog.Errorf("failed to prefetch from car: %v", err)
			}
		}
		carPrefetchSpan.End()
	}
	blocktime := uint64(block.Meta.Blocktime)

	allTransactionNodes := make([][]*ipldbindcode.Transaction, len(block.Entries))
	mu := &sync.Mutex{}
	var lastEntryHash solana.Hash
	{
		entryProcCtx, entryProcSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_ProcessEntries")
		wg := new(errgroup.Group)
		wg.SetLimit(runtime.NumCPU() * 2)
		// get entries from the block
		for entryIndex, entry := range block.Entries {
			entryIndex := entryIndex
			entryCid := entry.(cidlink.Link).Cid
			wg.Go(func() error {
				// get the entry by CID
				entryNode, err := epochHandler.GetEntryByCid(entryProcCtx, entryCid)
				if err != nil {
					klog.Errorf("failed to decode Entry: %v", err)
					return err
				}

				if entryIndex == len(block.Entries)-1 {
					lastEntryHash = solana.HashFromBytes(entryNode.Hash)
				}

				twg := new(errgroup.Group)
				twg.SetLimit(runtime.NumCPU())
				// get the transactions from the entry
				allTransactionNodes[entryIndex] = make([]*ipldbindcode.Transaction, len(entryNode.Transactions))
				for txI := range entryNode.Transactions {
					txI := txI
					tx := entryNode.Transactions[txI]
					twg.Go(func() error {
						// get the transaction by CID
						tcid := tx.(cidlink.Link).Cid
						txNode, err := epochHandler.GetTransactionByCid(entryProcCtx, tcid)
						if err != nil {
							klog.Errorf("failed to decode Transaction %s: %v", tcid, err)
							return nil
						}
						mu.Lock()
						allTransactionNodes[entryIndex][txI] = txNode
						mu.Unlock()
						return nil
					})
				}
				return twg.Wait()
			})
		}
		err = wg.Wait()
		entryProcSpan.End()
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get entries: %v", err)
		}
	}
	tim.time("get entries")

	var allTransactions []*jsonbuilder.OrderedJSONObject
	var rewardsUi *jsonbuilder.ArrayBuilder
	rewardsCid := block.Rewards.(cidlink.Link).Cid
	hasRewards := block.HasRewards()
	if *params.Options.Rewards && hasRewards {
		rewardsSpanCtx, rewardsSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_RewardsProcessing")
		rewardsNode, err := epochHandler.GetRewardsByCid(rewardsSpanCtx, rewardsCid)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to get RewardsByCid")
			rewardsSpan.End()
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decode Rewards: %v", err)
		}
		rewardsBuf, err := ipldbindcode.LoadDataFromDataFrames(&rewardsNode.Data, epochHandler.GetDataFrameByCid)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to load Rewards dataFrames")
			rewardsSpan.End()
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRewards, err := tooling.DecompressZstd(rewardsBuf)
		if err != nil {
			telemetry.RecordError(rewardsSpan, err, "Failed to decompress Rewards")
			rewardsSpan.End()
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decompress Rewards: %v", err)
		}
		rewardsSpan.SetAttributes(
			attribute.Int("rewards_compressed_size", len(rewardsBuf)),
			attribute.Int("rewards_uncompressed_size", len(uncompressedRewards)),
		)
		// try decoding as protobuf
		actualRewards, err := solanablockrewards.ParseRewards(uncompressedRewards)
		if err != nil {
			// TODO: add support for legacy rewards format
			fmt.Println("Rewards are not protobuf: " + err.Error())
		} else {
			// encode rewards as JSON, then decode it as a map
			rewards, _, err := solanablockrewards.RewardsToUi(actualRewards)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to encode rewards: %v", err)
			}
			rewardsUi = rewards
		}
	} else {
		klog.V(4).Infof("rewards not requested or not available")
	}
	tim.time("get rewards")
	{
		_, buildTxSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_BuildTransactions")
		for _, transactionNode := range mergeTxNodeSlices(allTransactionNodes) {
			tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to decode transaction: %v", err)
			}

			out := solanatxmetaparsers.NewEncodedTransactionWithStatusMeta(
				tx,
				meta,
			)

			txUI, err := out.ToUi(*params.Options.Encoding)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to encode transaction: %v", err)
			}
			// TODO: include position index in the UI output.
			// pos, ok := transactionNode.GetPositionIndex()
			// if ok {
			// 	txUI.Value("position", pos)
			// }

			allTransactions = append(allTransactions, txUI)
		}
		buildTxSpan.SetAttributes(attribute.Int("num_transactions", len(allTransactions)))
		buildTxSpan.End()
	}

	// sort.Slice(allTransactions, func(i, j int) bool {
	// 	return allTransactions[i].Position < allTransactions[j].Position
	// })
	tim.time("get transactions")

	response := jsonbuilder.NewObject()

	if slot == 0 {
		response.Uint("blockHeight", 0)

		genesis := epochHandler.GetGenesis()
		if genesis != nil {
			blockZeroBlocktime := uint64(genesis.Config.CreationTime.Unix())
			response.Value("blockTime", blockZeroBlocktime)
		}
		response.Uint("parentSlot", uint64(0))

		blockZeroBlockHash := lastEntryHash.String()
		response.Value("previousBlockhash", blockZeroBlockHash) // NOTE: this is what solana RPC does. Should it be nil instead? Or should it be the genesis hash?
	} else {
		response.Uint("parentSlot", uint64(block.Meta.Parent_slot))
		{
			blockHeight, ok := block.GetBlockHeight()
			if ok {
				response.Uint("blockHeight", blockHeight)
			} else {
				response.Null("blockHeight")
			}
		}
	}
	if blocktime != 0 {
		response.Value("blockTime", blocktime)
	}
	response.Value("blockhash", lastEntryHash.String())
	if rewardsUi != nil {
		response.Array("rewards", rewardsUi)
	} else {
		response.Value("rewards", make([]any, 0))
	}
	{
		parentSpanCtx, parentSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_GetParentBlockForHash")
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		parentSpan.SetAttributes(attribute.Int64("parent_slot", int64(parentSlot)))
		if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
			parentBlock, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(parentSpanCtx, false), parentSlot)
			if err != nil {
				telemetry.RecordError(parentSpan, err, "Failed to get parent block")
				parentSpan.End()
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to get/decode block: %v", err)
			}
			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := epochHandler.GetEntryByCid(parentSpanCtx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					telemetry.RecordError(parentSpan, err, "Failed to get parent entry")
					parentSpan.End()
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode Entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash).String()
				response.Value("previousBlockhash", parentEntryHash)
			}
		} else {
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
			}
		}
		parentSpan.End()
	}
	tim.time("get parent block")
	response.Value("transactions", allTransactions)

	err = conn.Reply(
		ctx,
		req.ID,
		response,
	)
	tim.time("reply")
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}

func mergeTxNodeSlices(slices [][]*ipldbindcode.Transaction) []*ipldbindcode.Transaction {
	var out []*ipldbindcode.Transaction
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}
