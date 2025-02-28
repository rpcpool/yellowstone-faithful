package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	jsoniter "github.com/json-iterator/go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"github.com/sourcegraph/jsonrpc2"
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
	tim := newTimer(getRequestIDFromContext(ctx))
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
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available", epochNumber),
		}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
	}

	block, blockCid, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, true), slot)
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
				blockCid, err = epochHandler.FindCidFromSlot(ctx, slot)
				if err != nil {
					return err
				}
				return nil
			})
			wg.Go(func() (err error) {
				if parentIsInPreviousEpoch {
					return nil
				}
				parentBlockCid, err = epochHandler.FindCidFromSlot(ctx, uint64(block.Meta.Parent_slot))
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
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctx, blockCid)
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
					offsetAndSize, err := epochHandler.FindOffsetAndSizeFromCid(ctx, parentBlockCid)
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
				carSection, err := epochHandler.ReadAtFromCar(ctx, start, length)
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
	}
	blocktime := uint64(block.Meta.Blocktime)

	allTransactionNodes := make([][]*ipldbindcode.Transaction, len(block.Entries))
	mu := &sync.Mutex{}
	var lastEntryHash solana.Hash
	{
		wg := new(errgroup.Group)
		wg.SetLimit(runtime.NumCPU() * 2)
		// get entries from the block
		for entryIndex, entry := range block.Entries {
			entryIndex := entryIndex
			entryCid := entry.(cidlink.Link).Cid
			wg.Go(func() error {
				// get the entry by CID
				entryNode, err := epochHandler.GetEntryByCid(ctx, entryCid)
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
						txNode, err := epochHandler.GetTransactionByCid(ctx, tcid)
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
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to get entries: %v", err)
		}
	}
	tim.time("get entries")

	var allTransactions []GetTransactionResponse
	var rewards any
	rewardsCid := block.Rewards.(cidlink.Link).Cid
	hasRewards := !rewardsCid.Equals(DummyCID)
	if *params.Options.Rewards && hasRewards {
		rewardsNode, err := epochHandler.GetRewardsByCid(ctx, rewardsCid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decode Rewards: %v", err)
		}
		rewardsBuf, err := tooling.LoadDataFromDataFrames(&rewardsNode.Data, epochHandler.GetDataFrameByCid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRewards, err := tooling.DecompressZstd(rewardsBuf)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decompress Rewards: %v", err)
		}
		// try decoding as protobuf
		actualRewards, err := solanablockrewards.ParseRewards(uncompressedRewards)
		if err != nil {
			// TODO: add support for legacy rewards format
			fmt.Println("Rewards are not protobuf: " + err.Error())
		} else {
			{
				// encode rewards as JSON, then decode it as a map
				buf, err := fasterJson.Marshal(actualRewards)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to encode rewards: %v", err)
				}
				var m map[string]any
				err = fasterJson.Unmarshal(buf, &m)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode rewards: %v", err)
				}
				if _, ok := m["rewards"]; ok {
					// iter over rewards as an array of maps, and add a "commission" field to each = nil
					rewardsAsArray := m["rewards"].([]any)
					for _, reward := range rewardsAsArray {
						rewardAsMap := reward.(map[string]any)
						if _, ok := rewardAsMap["commission"]; !ok {
							rewardAsMap["commission"] = nil
						}
						// if the commission field is a string, convert it to a float
						if asString, ok := rewardAsMap["commission"].(string); ok {
							rewardAsMap["commission"] = asFloat(asString)
						}
						// if no lamports field, add it and set it to 0
						if _, ok := rewardAsMap["lamports"]; !ok {
							rewardAsMap["lamports"] = uint64(0)
						}

						// if it has a post_balance field, convert it to postBalance
						if _, ok := rewardAsMap["post_balance"]; ok {
							rewardAsMap["postBalance"] = rewardAsMap["post_balance"]
							delete(rewardAsMap, "post_balance")
						}
						// if it has a reward_type field, convert it to rewardType
						if _, ok := rewardAsMap["reward_type"]; ok {
							rewardAsMap["rewardType"] = rewardAsMap["reward_type"]
							delete(rewardAsMap, "reward_type")

							// if it's a float, convert to int and use rentTypeToString
							if asFloat, ok := rewardAsMap["rewardType"].(float64); ok {
								rewardAsMap["rewardType"] = rewardTypeToString(int(asFloat))
							}
						}
					}
					rewards = rewardsAsArray
					// sort.Slice(rewardsAsArray, func(i, j int) bool {
					// 	// sort by rewardType, then by pubkey
					// 	if rewardTypeStringToInt(rewardsAsArray[i].(map[string]any)["rewardType"].(string)) != rewardTypeStringToInt(rewardsAsArray[j].(map[string]any)["rewardType"].(string)) {
					// 		return rewardTypeStringToInt(rewardsAsArray[i].(map[string]any)["rewardType"].(string)) > rewardTypeStringToInt(rewardsAsArray[j].(map[string]any)["rewardType"].(string))
					// 	}
					// 	return bytes.Compare(solana.MPK(rewardsAsArray[i].(map[string]any)["pubkey"].(string)).Bytes(), solana.MPK(rewardsAsArray[j].(map[string]any)["pubkey"].(string)).Bytes()) < 0
					// })
				} else {
					klog.Errorf("did not find rewards field in rewards")
					rewards = make([]any, 0)
				}
			}
		}
	} else {
		rewards = make([]any, 0)
	}
	tim.time("get rewards")
	{
		for _, transactionNode := range mergeTxNodeSlices(allTransactionNodes) {
			var txResp GetTransactionResponse

			// response.Slot = uint64(transactionNode.Slot)
			// if blocktime != 0 {
			// 	response.Blocktime = &blocktime
			// }

			{
				pos, ok := transactionNode.GetPositionIndex()
				if ok {
					txResp.Position = uint64(pos)
				}
				tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode transaction: %v", err)
				}
				txResp.Signatures = tx.Signatures
				if tx.Message.IsVersioned() {
					txResp.Version = tx.Message.GetVersion() - 1
				} else {
					txResp.Version = "legacy"
				}

				encodedTx, encodedMeta, err := encodeTransactionResponseBasedOnWantedEncoding(*params.Options.Encoding, tx, meta)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to encode transaction: %v", err)
				}
				txResp.Transaction = encodedTx
				txResp.Meta = encodedMeta
			}

			allTransactions = append(allTransactions, txResp)
		}
	}

	sort.Slice(allTransactions, func(i, j int) bool {
		return allTransactions[i].Position < allTransactions[j].Position
	})
	tim.time("get transactions")
	var blockResp GetBlockResponse
	blockResp.Transactions = allTransactions
	if blocktime != 0 {
		blockResp.BlockTime = &blocktime
	}
	blockResp.Blockhash = lastEntryHash.String()
	blockResp.ParentSlot = uint64(block.Meta.Parent_slot)
	blockResp.Rewards = rewards

	if slot == 0 {
		genesis := epochHandler.GetGenesis()
		if genesis != nil {
			blockZeroBlocktime := uint64(genesis.Config.CreationTime.Unix())
			blockResp.BlockTime = &blockZeroBlocktime
		}
		blockResp.ParentSlot = uint64(0)

		zeroBlockHeight := uint64(0)
		blockResp.BlockHeight = &zeroBlockHeight

		blockZeroBlockHash := lastEntryHash.String()
		blockResp.PreviousBlockhash = &blockZeroBlockHash // NOTE: this is what solana RPC does. Should it be nil instead? Or should it be the genesis hash?
	}

	{
		blockHeight, ok := block.GetBlockHeight()
		if ok {
			blockResp.BlockHeight = &blockHeight
		}
	}
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
			// NOTE: if the parent is in the same epoch, we can get it from the same epoch handler as the block;
			// otherwise, we need to get it from the previous epoch (TODO: implement this)
			parentBlock, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, false), parentSlot)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to get/decode block: %v", err)
			}

			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := epochHandler.GetEntryByCid(ctx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode Entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash).String()
				blockResp.PreviousBlockhash = &parentEntryHash
			}
		} else {
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
			}
		}
	}
	tim.time("get parent block")

	{
		if len(blockResp.Transactions) == 0 {
			blockResp.Transactions = make([]GetTransactionResponse, 0)
		}
		if blockResp.Rewards == nil || len(blockResp.Rewards.([]any)) == 0 {
			blockResp.Rewards = make([]any, 0)
		}
	}

	err = conn.Reply(
		ctx,
		req.ID,
		blockResp,
		func(m map[string]any) map[string]any {
			transactions, ok := m["transactions"].([]any)
			if !ok {
				return m
			}
			for i := range transactions {
				transaction, ok := transactions[i].(map[string]any)
				if !ok {
					continue
				}
				transactions[i] = adaptTransactionMetaToExpectedOutput(transaction)
			}

			return m
		},
	)
	tim.time("reply")
	if err != nil {
		return nil, fmt.Errorf("failed to reply: %w", err)
	}
	return nil, nil
}

func asFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		panic(err)
	}
	return f
}

func mergeTxNodeSlices(slices [][]*ipldbindcode.Transaction) []*ipldbindcode.Transaction {
	var out []*ipldbindcode.Transaction
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}
