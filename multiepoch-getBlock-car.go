package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	txpool "github.com/rpcpool/yellowstone-faithful/tx-pool"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/bytebufferpool"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/klog/v2"
)

func (multi *MultiEpoch) handleGetBlock_car(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
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

	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	childCid, err := epochHandler.FindCidFromSlot(ctx, slot)
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
	_ = childCid // TODO: use this CID to prefetch the block data
	// Find CAR file oasChild for CID in index.
	oasChild, err := epochHandler.FindOffsetAndSizeFromCid(ctx, childCid)
	if err != nil {
		// not found or error
		return nil, fmt.Errorf("failed to find offset for CID %s: %w", childCid, err)
	}
	childData, err := epochHandler.GetNodeByOffsetAndSizeBuffer(ctx, &childCid, oasChild)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Failed to get block",
		}, fmt.Errorf("failed to get block data: %w", err)
	}
	_ = childData // TODO: use this data to prefetch the block data
	block, err := iplddecoders.DecodeBlock(childData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode block: %w", err)
	}
	if uint64(block.Slot) != slot {
		return nil, fmt.Errorf("expected slot %d, got %d", slot, block.Slot)
	}
	bytebufferpool.Put(childData) // return the buffer to the pool
	{
		conn.ctx.Response.Header.Set("DAG-Root-CID", childCid.String())
	}

	parentSlot := uint64(block.Meta.Parent_slot)
	parentIsInPreviousEpoch := slottools.ParentIsInPreviousEpoch(parentSlot, (slot))
	_ = parentIsInPreviousEpoch
	// now we know the parent slot;
	// TODO: the parent object might be in the previous epoch, so we need to handle that case.
	parentCid, err := epochHandler.FindCidFromSlot(ctx, parentSlot)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return nil, fmt.Errorf("parent slot %d was skipped, or missing in long-term storage", parentSlot)
		}
	}
	if parentCid == cid.Undef {
		return nil, fmt.Errorf("parent CID for slot %d is undefined", parentSlot)
	}
	parentOas, err := epochHandler.FindOffsetAndSizeFromCid(ctx, parentCid)
	if err != nil {
		return nil, fmt.Errorf("failed to find offset for parent CID %s: %w", parentCid, err)
	}
	offsetParent := parentOas.Offset
	totalSize := oasChild.Offset + oasChild.Size - offsetParent
	reader, err := epochHandler.GetEpochReaderAt()
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch reader: %w", err)
	}
	// TODO: save this info immediately so for next getBlock(thisBlock) we know immediately where to read in the CAR file,
	// and whether the parent is in the previous epoch or not.
	section, err := carreader.ReadIntoBuffer(offsetParent, totalSize, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read node from CAR: %w", err)
	}
	tim.time("read section from CAR")

	nodes, err := nodetools.SplitIntoDataAndCids(section.Bytes())
	if err != nil {
		panic(err)
	}
	defer nodes.Put() // return the nodes to the pool
	nodes.SortByCid()
	bytebufferpool.Put(section) // return the buffer to the pool
	tim.time("nodes")

	parsedNodes, err := nodes.ToParsedAndCidSlice()
	if err != nil {
		panic(fmt.Errorf("failed to convert nodes to parsed nodes: %w", err))
	}
	defer parsedNodes.Put() // return the parsed nodes to the pool
	// parsedNodes.SortByCid() // NOTE: already sorted by CIDs in SplitIntoDataAndCids; ToParsedAndCidSlice maintains the same order.
	tim.time("parsedNodes")

	blocktime := uint64(block.Meta.Blocktime)

	lastEntryCid := block.Entries[len(block.Entries)-1]
	lastEntry, err := parsedNodes.EntryByCid(lastEntryCid.(cidlink.Link).Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get last entry: %w", err)
	}
	lastEntryHash := solana.HashFromBytes(lastEntry.Hash)
	tim.time("get entries")

	var rewardsUi *jsonbuilder.ArrayBuilder
	defer rewardsUi.Put() // recycle the rewards UI array
	hasRewards := block.HasRewards()
	rewardsCid := block.Rewards.(cidlink.Link).Cid
	if *params.Options.Rewards && hasRewards {
		actualRewards, err := nodetools.GetParsedRewards(parsedNodes, rewardsCid)
		if err != nil {
			slog.Error(
				"failed to parse block rewards",
				"block", slot,
				"rewards_cid", rewardsCid.String(),
				"error", err,
			)
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get parsed rewards by CID %s: %v", rewardsCid, err)
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

	transactionDetails := *params.Options.TransactionDetails

	response := jsonbuilder.NewObject()
	defer response.Put() // recycle the response object
	if transactionDetails != rpc.TransactionDetailsNone {
		if transactionDetails == rpc.TransactionDetailsSignatures {
			signatures := make([][]byte, 0, parsedNodes.CountTransactions())
			for transactionNode := range parsedNodes.Transaction() {
				sig, err := tooling.ReadFirstSignature(transactionNode.Data.Data)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode Transaction: %v", err)
				}
				signatures = append(signatures, sig[:])
			}
			response.Base58Slice("signatures", signatures)
			tim.time("get signatures")
		}
		if transactionDetails == rpc.TransactionDetailsAccounts || transactionDetails == rpc.TransactionDetailsFull {
			allTransactions := make([]*jsonbuilder.OrderedJSONObject, 0, parsedNodes.CountTransactions())
			defer func() {
				for _, tx := range allTransactions {
					tx.Put() // recycle the transaction objects
				}
			}()
			{
				_, buildTxSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_BuildTransactions")
				for _, transactionNode := range parsedNodes.SortedTransactions() {
					err := func() error {
						tx, meta, err := nodetools.ParseTransactionAndMetaFromNode(transactionNode, func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
							df, err := parsedNodes.DataFrameByCid(wantedCid)
							if err != nil {
								return nil, fmt.Errorf("failed to get DataFrame by CID %s: %w", wantedCid, err)
							}
							return df, nil
						})
						if err != nil {
							return fmt.Errorf("failed to decode transaction: %v", err)
						}

						out := solanatxmetaparsers.NewEncodedTransactionWithStatusMeta(
							tx,
							meta,
						)

						txUI, err := out.ToUi(*params.Options.Encoding, transactionDetails)
						if err != nil {
							return fmt.Errorf("failed to encode transaction: %v", err)
						}
						out.Meta.Put()
						txpool.Put(tx) // return the transaction to the pool
						// TODO: include position index in the UI output.
						// pos, ok := transactionNode.GetPositionIndex()
						// if ok {
						// 	txUI.Value("position", pos)
						// }
						allTransactions = append(allTransactions, txUI)
						return nil
					}()
					if err != nil {
						return &jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "Internal error",
						}, fmt.Errorf("failed to build transactions: %w", err)
					}
				}

				buildTxSpan.SetAttributes(attribute.Int("num_transactions", len(allTransactions)))
				buildTxSpan.End()
			}
			tim.time("get transactions")
			response.Value("transactions", allTransactions)
		}
	}

	// sort.Slice(allTransactions, func(i, j int) bool {
	// 	return allTransactions[i].Position < allTransactions[j].Position
	// })

	if slot == 0 {
		response.Uint("blockHeight", 0)

		genesis := epochHandler.GetGenesis()
		if genesis != nil {
			blockZeroBlocktime := uint64(genesis.Config.CreationTime.Unix())
			response.Uint("blockTime", blockZeroBlocktime)
		}
		response.Uint("parentSlot", uint64(0))

		blockZeroBlockHash := lastEntryHash.String()
		response.String("previousBlockhash", blockZeroBlockHash) // NOTE: this is what solana RPC does. Should it be nil instead? Or should it be the genesis hash?
	} else {
		response.Uint("parentSlot", parentSlot)
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
		response.Uint("blockTime", blocktime)
	}
	response.String("blockhash", lastEntryHash.String())
	if rewardsUi != nil {
		response.Array("rewards", rewardsUi)
	} else {
		response.EmptyArray("rewards")
	}
	{
		parentSpanCtx, parentSpan := telemetry.StartSpan(rpcSpanCtx, "GetBlock_GetParentBlockForHash")
		parentSpan.SetAttributes(attribute.Int64("parent_slot", int64(parentSlot)))
		if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
			parentBlock, err := parsedNodes.BlockByCid(parentCid)
			if err != nil {
				telemetry.RecordError(parentSpan, err, "Failed to get parent block")
				parentSpan.End()
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to get/decode block: %v", err)
			}
			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1].(cidlink.Link).Cid

				parentEntryNode, err := epochHandler.GetEntryByCid(parentSpanCtx, lastEntryCidOfParent)
				if err != nil {
					telemetry.RecordError(parentSpan, err, "Failed to get parent entry")
					parentSpan.End()
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode Entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash).String()
				response.String("previousBlockhash", parentEntryHash)
				// response.Null("previousBlockhash")
			}
		} else {
			// TODO: handle the case when the parent is in a different epoch.
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
			}
		}
		parentSpan.End()
	}
	tim.time("get parent block")

	encodedResult, err := response.MarshalJSONToByteBuffer()
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "Internal error",
		}, fmt.Errorf("failed to encode response: %w", err)
	}
	defer bytebufferpool.Put(encodedResult) // return the buffer to the pool
	conn.ReplyRawMessage(
		ctx,
		req.ID,
		json.RawMessage(encodedResult.Bytes()),
	)
	tim.time("reply")
	return nil, nil
}
