package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

func (multi *MultiEpoch) handleGetBlock(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	tim := newTimer()
	params, err := parseGetBlockRequest(req.Params)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "Invalid params",
		}, fmt.Errorf("failed to parse params: %w", err)
	}
	tim.time("parseGetBlockRequest")
	slot := params.Slot

	// find the epoch that contains the requested slot
	epochNumber := CalcEpochForSlot(slot)
	epoch, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return &jsonrpc2.Error{
			Code:    CodeNotFound,
			Message: fmt.Sprintf("Epoch %d is not available", epochNumber),
		}, fmt.Errorf("failed to get epoch %d: %w", epochNumber, err)
	}

	ser := epoch

	block, err := ser.GetBlock(WithSubrapghPrefetch(ctx, true), slot)
	if err != nil {
		if errors.Is(err, compactindex36.ErrNotFound) {
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
	tim.time("GetBlock")
	{
		prefetcherFromCar := func() error {
			var blockCid, parentCid cid.Cid
			wg := new(errgroup.Group)
			wg.Go(func() (err error) {
				blockCid, err = ser.FindCidFromSlot(ctx, slot)
				if err != nil {
					return err
				}
				return nil
			})
			wg.Go(func() (err error) {
				parentCid, err = ser.FindCidFromSlot(ctx, uint64(block.Meta.Parent_slot))
				if err != nil {
					return err
				}
				return nil
			})
			err = wg.Wait()
			if err != nil {
				return err
			}
			{
				var blockOffset, parentOffset uint64
				wg := new(errgroup.Group)
				wg.Go(func() (err error) {
					blockOffset, err = ser.FindOffsetFromCid(ctx, blockCid)
					if err != nil {
						return err
					}
					return nil
				})
				wg.Go(func() (err error) {
					parentOffset, err = ser.FindOffsetFromCid(ctx, parentCid)
					if err != nil {
						// If the parent is not found, it (probably) means that it's outside of the car file.
						parentOffset = 0
					}
					return nil
				})
				err = wg.Wait()
				if err != nil {
					return err
				}

				parentIsInPreviousEpoch := CalcEpochForSlot(uint64(block.Meta.Parent_slot)) != CalcEpochForSlot(slot)

				length := blockOffset - parentOffset
				// cap the length to 1GB
				GiB := uint64(1024 * 1024 * 1024)
				if length > GiB {
					length = GiB
				}
				carSection, err := ser.ReadAtFromCar(ctx, parentOffset, length)
				if err != nil {
					return err
				}
				dr := bytes.NewReader(carSection)

				br := bufio.NewReader(dr)

				gotCid, data, err := util.ReadNode(br)
				if err != nil {
					return err
				}
				if !parentIsInPreviousEpoch && !gotCid.Equals(parentCid) {
					return fmt.Errorf("CID mismatch: expected %s, got %s", parentCid, gotCid)
				}
				ser.putNodeInCache(gotCid, data)

				for {
					gotCid, data, err = util.ReadNode(br)
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						return err
					}
					if gotCid.Equals(blockCid) {
						break
					}
					ser.putNodeInCache(gotCid, data)
				}
			}
			return nil
		}
		if ser.lassieFetcher == nil {
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
				entryNode, err := ser.GetEntryByCid(ctx, entryCid)
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
						txNode, err := ser.GetTransactionByCid(ctx, tcid)
						if err != nil {
							klog.Errorf("failed to decode Transaction %s: %v", tcid, err)
							return nil
						}
						// NOTE: this messes up the order of transactions,
						// but we sort them later anyway.
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
	hasRewards := !block.Rewards.(cidlink.Link).Cid.Equals(DummyCID)
	if hasRewards {
		rewardsNode, err := ser.GetRewardsByCid(ctx, block.Rewards.(cidlink.Link).Cid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to decode Rewards: %v", err)
		}
		rewardsBuf, err := loadDataFromDataFrames(&rewardsNode.Data, ser.GetDataFrameByCid)
		if err != nil {
			return &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			}, fmt.Errorf("failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRewards, err := decompressZstd(rewardsBuf)
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
				buf, err := json.Marshal(actualRewards)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to encode rewards: %v", err)
				}
				var m map[string]any
				err = json.Unmarshal(buf, &m)
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
						rewardAsMap["commission"] = nil

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
								rewardAsMap["rewardType"] = rentTypeToString(int(asFloat))
							}
						}
					}
					rewards = m["rewards"]
				} else {
					klog.Errorf("did not find rewards field in rewards")
				}
			}
		}
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
				tx, meta, err := parseTransactionAndMetaFromNode(transactionNode, ser.GetDataFrameByCid)
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
				txResp.Meta = meta

				b64Tx, err := tx.ToBase64()
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to encode transaction: %v", err)
				}

				txResp.Transaction = []any{b64Tx, "base64"}
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
	blockResp.BlockTime = &blocktime
	blockResp.Blockhash = lastEntryHash.String()
	blockResp.ParentSlot = uint64(block.Meta.Parent_slot)
	blockResp.Rewards = rewards

	{
		blockHeight, ok := block.GetBlockHeight()
		if ok {
			blockResp.BlockHeight = &blockHeight
		}
	}
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		if parentSlot != 0 {
			parentBlock, err := ser.GetBlock(WithSubrapghPrefetch(ctx, false), parentSlot)
			if err != nil {
				return &jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				}, fmt.Errorf("failed to decode block: %v", err)
			}

			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := ser.GetEntryByCid(ctx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					return &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					}, fmt.Errorf("failed to decode Entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash)
				blockResp.PreviousBlockhash = parentEntryHash.String()
			}
		}
	}
	tim.time("get parent block")

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

func mergeTxNodeSlices(slices [][]*ipldbindcode.Transaction) []*ipldbindcode.Transaction {
	var out []*ipldbindcode.Transaction
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}
