package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

type InternalError struct {
	Err error
}

func (e *InternalError) Error() string {
	return fmt.Sprintf("internal error: %s", e.Err)
}

func (e *InternalError) Unwrap() error {
	return e.Err
}

func (e *InternalError) IsPublic() bool {
	return false
}

func (e *InternalError) Is(err error) bool {
	return errors.Is(e.Err, err)
}

func (e *InternalError) As(target interface{}) bool {
	return errors.As(e.Err, target)
}

type timer struct {
	start time.Time
	prev  time.Time
}

func newTimer() *timer {
	now := time.Now()
	return &timer{
		start: now,
		prev:  now,
	}
}

func (t *timer) time(name string) {
	klog.V(2).Infof("TIMED: %s: %s (overall %s)", name, time.Since(t.prev), time.Since(t.start))
	t.prev = time.Now()
}

func (ser *rpcServer) handleGetBlock(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	tim := newTimer()
	params, err := parseGetBlockRequest(req.Params)
	if err != nil {
		klog.Errorf("failed to parse params: %v", err)
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeInvalidParams,
				Message: "Invalid params",
			})
		return
	}
	tim.time("parseGetBlockRequest")
	slot := params.Slot

	block, err := ser.GetBlock(WithSubrapghPrefetch(ctx, true), slot)
	if err != nil {
		klog.Errorf("failed to get block: %v", err)
		if errors.Is(err, compactindex36.ErrNotFound) {
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    CodeNotFound,
					Message: fmt.Sprintf("Slot %d was skipped, or missing in long-term storage", slot),
				})
		} else {
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Failed to get block",
				})
		}
		return
	}
	tim.time("GetBlock")
	blocktime := uint64(block.Meta.Blocktime)

	allTransactionNodes := make([]*ipldbindcode.Transaction, 0)
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
						allTransactionNodes = append(allTransactionNodes, txNode)
						mu.Unlock()
						return nil
					})
				}
				return twg.Wait()
			})
		}
		err = wg.Wait()
		if err != nil {
			klog.Errorf("failed to get entries: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}
	}
	tim.time("get entries")

	var allTransactions []GetTransactionResponse
	var rewards any // TODO: implement rewards as in solana
	hasRewards := !block.Rewards.(cidlink.Link).Cid.Equals(DummyCID)
	if hasRewards {
		rewardsNode, err := ser.GetRewardsByCid(ctx, block.Rewards.(cidlink.Link).Cid)
		if err != nil {
			klog.Errorf("failed to decode Rewards: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}
		rewardsBuf, err := loadDataFromDataFrames(&rewardsNode.Data, ser.GetDataFrameByCid)
		if err != nil {
			klog.Errorf("failed to load Rewards dataFrames: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
		}

		uncompressedRewards, err := decompressZstd(rewardsBuf)
		if err != nil {
			klog.Errorf("failed to decompress Rewards: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "Internal error",
				})
			return
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
					klog.Errorf("failed to encode rewards: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "Internal error",
						})
					return
				}
				var m map[string]any
				err = json.Unmarshal(buf, &m)
				if err != nil {
					klog.Errorf("failed to decode rewards: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "Internal error",
						})
					return
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
		for _, transactionNode := range allTransactionNodes {
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
					klog.Errorf("failed to decode transaction: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "Internal error",
						})
					return
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
					klog.Errorf("failed to encode transaction: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "Internal error",
						})
					return
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
	blockResp.Rewards = rewards                                 // TODO: implement rewards as in solana
	blockResp.BlockHeight = calcBlockHeight(uint64(block.Slot)) // TODO: implement block height
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		if parentSlot != 0 {
			parentBlock, err := ser.GetBlock(WithSubrapghPrefetch(ctx, false), parentSlot)
			if err != nil {
				klog.Errorf("failed to decode block: %v", err)
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					})
				return
			}

			lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
			parentEntryNode, err := ser.GetEntryByCid(ctx, lastEntryCidOfParent.(cidlink.Link).Cid)
			if err != nil {
				klog.Errorf("failed to decode Entry: %v", err)
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "Internal error",
					})
				return
			}
			parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash)
			blockResp.PreviousBlockhash = parentEntryHash.String()
		}
	}
	tim.time("get parent block")

	// TODO: get all the transactions from the block
	// reply with the data
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
		klog.Errorf("failed to reply: %v", err)
	}
}

//	pub enum RewardType {
//	    Fee,
//	    Rent,
//	    Staking,
//	    Voting,
//	}
func rentTypeToString(typ int) string {
	switch typ {
	case 1:
		return "Fee"
	case 2:
		return "Rent"
	case 3:
		return "Staking"
	case 4:
		return "Voting"
	default:
		return "Unknown"
	}
}

const CodeNotFound = -32009
