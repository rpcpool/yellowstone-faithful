package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // Install the gzip compressor
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// ListeAndServe starts listening on the configured address and serves the RPC API.
func (me *MultiEpoch) ListenAndServeGRPC(ctx context.Context, listenOn string) error {
	lis, err := net.Listen("tcp", listenOn)
	if err != nil {
		return fmt.Errorf("failed to create listener for gRPC server: %w", err)
	}

	grpcServer := grpc.NewServer()
	old_faithful_grpc.RegisterOldFaithfulServer(grpcServer, me)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC server: %w", err)
	}
	return nil
}

func (me *MultiEpoch) GetVersion(context.Context, *old_faithful_grpc.VersionRequest) (*old_faithful_grpc.VersionResponse, error) {
	// faithfulVersion["version"] = GitTag
	// faithfulVersion["commit"] = GitCommit
	// faithfulVersion["epochs"] = me.GetEpochNumbers()
	resp := &old_faithful_grpc.VersionResponse{
		Version: func() string {
			if GitTag == "" {
				return GitCommit
			}
			return GitTag
		}(),
	}
	return resp, nil
}

func (multi *MultiEpoch) GetBlock(ctx context.Context, params *old_faithful_grpc.BlockRequest) (*old_faithful_grpc.BlockResponse, error) {
	// find the epoch that contains the requested slot
	slot := params.Slot
	epochNumber := CalcEpochForSlot(slot)
	epochHandler, err := multi.GetEpoch(epochNumber)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available", epochNumber)
	}

	block, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, true), slot)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "Slot %d was skipped, or missing in long-term storage", slot)
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
	}

	tim := newTimer(getRequestIDFromContext(ctx))
	tim.time("GetBlock")
	{
		prefetcherFromCar := func() error {
			parentIsInPreviousEpoch := CalcEpochForSlot(uint64(block.Meta.Parent_slot)) != CalcEpochForSlot(slot)
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
			return nil, status.Errorf(codes.Internal, "Failed to get entries: %v", err)
		}
	}
	tim.time("get entries")

	resp := &old_faithful_grpc.BlockResponse{
		Slot: uint64(block.Slot),
	}

	var allTransactions []*old_faithful_grpc.Transaction
	hasRewards := !block.Rewards.(cidlink.Link).Cid.Equals(DummyCID)
	if hasRewards {
		rewardsNode, err := epochHandler.GetRewardsByCid(ctx, block.Rewards.(cidlink.Link).Cid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get Rewards: %v", err)
		}
		rewardsBuf, err := loadDataFromDataFrames(&rewardsNode.Data, epochHandler.GetDataFrameByCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to load Rewards dataFrames: %v", err)
		}

		uncompressedRawRewards, err := decompressZstd(rewardsBuf)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to decompress Rewards: %v", err)
		}
		resp.Rewards = uncompressedRawRewards
	}
	tim.time("get rewards")
	{
		for _, transactionNode := range mergeTxNodeSlices(allTransactionNodes) {
			txResp := new(old_faithful_grpc.Transaction)

			// response.Slot = uint64(transactionNode.Slot)
			// if blocktime != 0 {
			// 	response.Blocktime = &blocktime
			// }

			{
				pos, ok := transactionNode.GetPositionIndex()
				if ok {
					txResp.Index = ptrToUint64(uint64(pos))
				}
				txResp.Transaction, txResp.Meta, err = getTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
				}
			}

			allTransactions = append(allTransactions, txResp)
		}
	}

	sort.Slice(allTransactions, func(i, j int) bool {
		if allTransactions[i].Index == nil || allTransactions[j].Index == nil {
			return false
		}
		return *allTransactions[i].Index < *allTransactions[j].Index
	})
	tim.time("get transactions")
	resp.Transactions = allTransactions
	blocktime := uint64(block.Meta.Blocktime)
	if blocktime != 0 {
		resp.BlockTime = int64(blocktime)
	}
	resp.Blockhash = lastEntryHash[:]
	resp.ParentSlot = uint64(block.Meta.Parent_slot)

	if slot == 0 {
		genesis := epochHandler.GetGenesis()
		if genesis != nil {
			blockZeroBlocktime := uint64(genesis.Config.CreationTime.Unix())
			resp.BlockTime = int64(blockZeroBlocktime)
		}
		resp.ParentSlot = uint64(0)

		zeroBlockHeight := uint64(0)
		resp.BlockHeight = zeroBlockHeight

		blockZeroBlockHash := lastEntryHash
		resp.PreviousBlockhash = blockZeroBlockHash[:] // NOTE: this is what solana RPC does. Should it be nil instead? Or should it be the genesis hash?
	}

	{
		blockHeight, ok := block.GetBlockHeight()
		if ok {
			resp.BlockHeight = blockHeight
		}
	}
	{
		// get parent slot
		parentSlot := uint64(block.Meta.Parent_slot)
		if (parentSlot != 0 || slot == 1) && CalcEpochForSlot(parentSlot) == epochNumber {
			// NOTE: if the parent is in the same epoch, we can get it from the same epoch handler as the block;
			// otherwise, we need to get it from the previous epoch (TODO: implement this)
			parentBlock, _, err := epochHandler.GetBlock(WithSubrapghPrefetch(ctx, false), parentSlot)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to get parent block: %v", err)
			}

			if len(parentBlock.Entries) > 0 {
				lastEntryCidOfParent := parentBlock.Entries[len(parentBlock.Entries)-1]
				parentEntryNode, err := epochHandler.GetEntryByCid(ctx, lastEntryCidOfParent.(cidlink.Link).Cid)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "Failed to get parent entry: %v", err)
				}
				parentEntryHash := solana.HashFromBytes(parentEntryNode.Hash)
				resp.PreviousBlockhash = parentEntryHash[:]
			}
		} else {
			if slot != 0 {
				klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
			}
		}
	}
	tim.time("get parent block")

	return resp, nil
}

func (multi *MultiEpoch) GetTransaction(ctx context.Context, params *old_faithful_grpc.TransactionRequest) (*old_faithful_grpc.TransactionResponse, error) {
	if multi.CountEpochs() == 0 {
		return nil, status.Errorf(codes.Internal, "no epochs available")
	}

	sig := solana.SignatureFromBytes(params.Signature)

	startedEpochLookupAt := time.Now()
	epochNumber, err := multi.findEpochNumberFromSignature(ctx, sig)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return nil, status.Errorf(codes.NotFound, "Transaction not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get epoch for signature %s: %v", sig, err)
	}
	klog.V(4).Infof("Found signature %s in epoch %d in %s", sig, epochNumber, time.Since(startedEpochLookupAt))

	epochHandler, err := multi.GetEpoch(uint64(epochNumber))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Epoch %d is not available from this RPC", epochNumber)
	}

	transactionNode, _, err := epochHandler.GetTransaction(WithSubrapghPrefetch(ctx, true), sig)
	if err != nil {
		if errors.Is(err, compactindexsized.ErrNotFound) {
			// NOTE: solana just returns null here in case of transaction not found: {"jsonrpc":"2.0","result":null,"id":1}
			return nil, status.Errorf(codes.NotFound, "Transaction not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
	}

	response := &old_faithful_grpc.TransactionResponse{
		Transaction: &old_faithful_grpc.Transaction{},
	}
	response.Slot = uint64(transactionNode.Slot)
	{
		block, _, err := epochHandler.GetBlock(ctx, uint64(transactionNode.Slot))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get block: %v", err)
		}
		blocktime := uint64(block.Meta.Blocktime)
		if blocktime != 0 {
			response.BlockTime = int64(blocktime)
		}
	}

	{
		pos, ok := transactionNode.GetPositionIndex()
		if ok {
			response.Index = ptrToUint64(uint64(pos))
		}
		response.Transaction.Transaction, response.Transaction.Meta, err = getTransactionAndMetaFromNode(transactionNode, epochHandler.GetDataFrameByCid)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get transaction: %v", err)
		}
	}

	return response, nil
}

func (multi *MultiEpoch) Get(ser old_faithful_grpc.OldFaithful_GetServer) error {
	ctx := ser.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := ser.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		id := req.GetId()

		switch req.Request.(type) {
		case *old_faithful_grpc.GetRequest_Block:
			params := req.GetBlock()
			resp, err := multi.GetBlock(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send block error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get block: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Block{Block: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send block response: %v", id, err)
			}
		case *old_faithful_grpc.GetRequest_Transaction:
			params := req.GetTransaction()
			resp, err := multi.GetTransaction(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send transaction error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get transaction: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Transaction{Transaction: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send transaction response: %v", id, err)
			}
		case *old_faithful_grpc.GetRequest_Version:
			params := req.GetVersion()
			resp, err := multi.GetVersion(ctx, params)
			if err != nil {
				gerr, ok := status.FromError(err)
				if ok {
					err := ser.Send(&old_faithful_grpc.GetResponse{
						Id: id,
						Response: &old_faithful_grpc.GetResponse_Error{
							Error: &old_faithful_grpc.GetResponseError{
								Code: func() old_faithful_grpc.GetResponseErrorCode {
									switch gerr.Code() {
									case codes.NotFound:
										return old_faithful_grpc.GetResponseErrorCode_NOT_FOUND
									default:
										return old_faithful_grpc.GetResponseErrorCode_INTERNAL
									}
								}(),
								Message: gerr.Message(),
							},
						},
					})
					if err != nil {
						return status.Errorf(codes.Internal, "request %d; failed to send version error response: %v", id, err)
					}
					continue
				}
				return status.Errorf(codes.Internal, "request %d; failed to get version: %v", id, err)
			}
			if err := ser.Send(&old_faithful_grpc.GetResponse{
				Id:       id,
				Response: &old_faithful_grpc.GetResponse_Version{Version: resp},
			}); err != nil {
				return status.Errorf(codes.Internal, "request %d; failed to send version response: %v", id, err)
			}
		default:
			return status.Errorf(codes.InvalidArgument, "unknown request type %T", req.Request)
		}
	}
}
