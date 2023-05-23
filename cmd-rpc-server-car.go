package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/urfave/cli/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/ipld/ipldbindcode"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"go.firedancer.io/radiance/pkg/blockstore"
	"k8s.io/klog/v2"
)

func newCmd_rpcServerCar() *cli.Command {
	var listenOn string
	return &cli.Command{
		Name:        "rpc-server-car",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<car-path> <cid-to-offset-index-filepath> <slot-to-cid-index-filepath> <sig-to-cid-index-filepath>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Listen address",
				Value:       ":8899",
				Destination: &listenOn,
			},
		},
		Action: func(c *cli.Context) error {
			carFilepath := c.Args().Get(0)
			if carFilepath == "" {
				return cli.Exit("Must provide a CAR filepath", 1)
			}
			cidToOffsetIndexFilepath := c.Args().Get(1)
			if cidToOffsetIndexFilepath == "" {
				return cli.Exit("Must provide a CID-to-offset index filepath", 1)
			}
			slotToCidIndexFilepath := c.Args().Get(2)
			if slotToCidIndexFilepath == "" {
				return cli.Exit("Must provide a slot-to-CID index filepath", 1)
			}
			sigToCidIndexFilepath := c.Args().Get(3)
			if sigToCidIndexFilepath == "" {
				return cli.Exit("Must provide a signature-to-CID index filepath", 1)
			}

			carReader, err := carv2.OpenReader(carFilepath)
			if err != nil {
				return fmt.Errorf("failed to open CAR file: %w", err)
			}
			defer carReader.Close()

			cidToOffsetIndexFile, err := os.Open(cidToOffsetIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer cidToOffsetIndexFile.Close()

			cidToOffsetIndex, err := compactindex.Open(cidToOffsetIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			slotToCidIndexFile, err := os.Open(slotToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer slotToCidIndexFile.Close()

			slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			sigToCidIndexFile, err := os.Open(sigToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer sigToCidIndexFile.Close()

			sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			return newRPCServer(
				c.Context,
				listenOn,
				carReader,
				cidToOffsetIndex,
				slotToCidIndex,
				sigToCidIndex,
			)
		},
	}
}

func newRPCServer(
	ctx context.Context,
	listenOn string,
	carReader *carv2.Reader,
	cidToOffsetIndex *compactindex.DB,
	slotToCidIndex *compactindex36.DB,
	sigToCidIndex *compactindex36.DB,
) error {
	// start a JSON RPC server
	handler := &rpcServer{
		carReader:        carReader,
		cidToOffsetIndex: cidToOffsetIndex,
		slotToCidIndex:   slotToCidIndex,
		sigToCidIndex:    sigToCidIndex,
	}

	r := gin.Default()
	r.POST("/", newRPC(handler))
	klog.Infof("Listening on %s", listenOn)
	return r.Run(listenOn)
}

func newRPC(handler *rpcServer) func(c *gin.Context) {
	return func(c *gin.Context) {
		startedAt := time.Now()
		defer func() {
			klog.Infof("request took %s", time.Since(startedAt))
		}()
		// read request body
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			klog.Errorf("failed to read request body: %v", err)
			// reply with error
			c.JSON(http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}

		// parse request
		var rpcRequest jsonrpc2.Request
		if err := json.Unmarshal(body, &rpcRequest); err != nil {
			klog.Errorf("failed to unmarshal request: %v", err)
			c.JSON(http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}

		klog.Infof("request: %s", string(body))

		rf := &requestContext{ctx: c}

		// handle request
		handler.Handle(c.Request.Context(), rf, &rpcRequest)
	}
}

type responseWriter struct {
	http.ResponseWriter
}

type logger struct{}

func (l logger) Printf(tmpl string, args ...interface{}) {
	klog.Infof(tmpl, args...)
}

type rpcServer struct {
	carReader        *carv2.Reader
	cidToOffsetIndex *compactindex.DB
	slotToCidIndex   *compactindex36.DB
	sigToCidIndex    *compactindex36.DB
}

type requestContext struct {
	ctx *gin.Context
}

// ReplyWithError(ctx context.Context, id ID, respErr *Error) error {
func (c *requestContext) ReplyWithError(ctx context.Context, id jsonrpc2.ID, respErr *jsonrpc2.Error) error {
	resp := &jsonrpc2.Response{
		ID:    id,
		Error: respErr,
	}
	c.ctx.JSON(http.StatusOK, resp)
	return nil
}

// Reply(ctx context.Context, id ID, result interface{}) error {
func (c *requestContext) Reply(ctx context.Context, id jsonrpc2.ID, result interface{}) error {
	resRaw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	c.ctx.JSON(http.StatusOK, resp)
	return err
}

func (s *rpcServer) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
	offset, err := s.FindOffsetFromCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find offset for CID %s: %v", wantedCid, err)
		// not found or error
		return nil, err
	}
	return s.GetNodeByOffset(ctx, wantedCid, offset)
}

func (s *rpcServer) GetNodeByOffset(ctx context.Context, wantedCid cid.Cid, offset uint64) ([]byte, error) {
	// seek to offset
	dr, err := s.carReader.DataReader()
	if err != nil {
		klog.Errorf("failed to get data reader: %v", err)
		return nil, err
	}
	dr.Seek(int64(offset), io.SeekStart)
	br := bufio.NewReader(dr)

	gotCid, data, err := util.ReadNode(br)
	if err != nil {
		klog.Errorf("failed to read node: %v", err)
		return nil, err
	}
	// verify that the CID we read matches the one we expected.
	if !gotCid.Equals(wantedCid) {
		klog.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
		return nil, err
	}
	return data, nil
}

type GetBlockRequest struct {
	Slot uint64 `json:"slot"`
	// TODO: add more params
}

func parseGetBlockRequest(raw *json.RawMessage) (*GetBlockRequest, error) {
	var params []any
	if err := json.Unmarshal(*raw, &params); err != nil {
		klog.Errorf("failed to unmarshal params: %v", err)
		return nil, err
	}
	slotRaw, ok := params[0].(json.Number)
	if !ok {
		klog.Errorf("first argument must be a number")
		return nil, nil
	}

	slot, err := slotRaw.Int64()
	if err != nil {
		klog.Errorf("failed to convert slot to int64: %v", err)
		return nil, err
	}
	return &GetBlockRequest{
		Slot: uint64(slot),
	}, nil
}

func (ser *rpcServer) FindCidFromSlot(ctx context.Context, slot uint64) (cid.Cid, error) {
	return findCidFromSlot(ser.slotToCidIndex, slot)
}

func (ser *rpcServer) FindCidFromSignature(ctx context.Context, sig solana.Signature) (cid.Cid, error) {
	return findCidFromSignature(ser.sigToCidIndex, sig)
}

func (ser *rpcServer) FindOffsetFromCid(ctx context.Context, cid cid.Cid) (uint64, error) {
	return findOffsetFromCid(ser.cidToOffsetIndex, cid)
}

func (ser *rpcServer) GetBlock(ctx context.Context, slot uint64) (*ipldbindcode.Block, error) {
	// get the slot by slot number
	wantedCid, err := ser.FindCidFromSlot(ctx, slot)
	if err != nil {
		klog.Errorf("failed to find CID for slot %d: %v", slot, err)
		return nil, err
	}
	// get the block by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by offset: %v", err)
		return nil, err
	}
	// try parsing the data as a Block node.
	decoded, err := iplddecoders.DecodeBlock(data)
	if err != nil {
		klog.Errorf("failed to decode block: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *rpcServer) GetTransaction(ctx context.Context, sig solana.Signature) (*ipldbindcode.Transaction, error) {
	// get the CID by signature
	wantedCid, err := ser.FindCidFromSignature(ctx, sig)
	if err != nil {
		klog.Errorf("failed to find CID for signature %s: %v", sig, err)
		return nil, err
	}
	// get the transaction by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to get node by offset: %v", err)
		return nil, err
	}
	// try parsing the data as a Transaction node.
	decoded, err := iplddecoders.DecodeTransaction(data)
	if err != nil {
		klog.Errorf("failed to decode transaction: %v", err)
		return nil, err
	}
	return decoded, nil
}

type GetTransactionRequest struct {
	Signature solana.Signature `json:"signature"`
	// TODO: add more params
}

func parseGetTransactionRequest(raw *json.RawMessage) (*GetTransactionRequest, error) {
	var params []any
	if err := json.Unmarshal(*raw, &params); err != nil {
		klog.Errorf("failed to unmarshal params: %v", err)
		return nil, err
	}
	sigRaw, ok := params[0].(string)
	if !ok {
		klog.Errorf("first argument must be a string")
		return nil, nil
	}

	sig, err := solana.SignatureFromBase58(sigRaw)
	if err != nil {
		klog.Errorf("failed to convert signature from base58: %v", err)
		return nil, err
	}
	return &GetTransactionRequest{
		Signature: sig,
	}, nil
}

// jsonrpc2.RequestHandler interface
func (ser *rpcServer) Handle(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	switch req.Method {
	case "getBlock":
		params, err := parseGetBlockRequest(req.Params)
		if err != nil {
			klog.Errorf("failed to parse params: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}
		slot := params.Slot

		block, err := ser.GetBlock(ctx, slot)
		if err != nil {
			klog.Errorf("failed to get block: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "failed to get block",
				})
			return
		}
		// TODO: get all the transactions from the block
		// reply with the data
		err = conn.Reply(ctx, req.ID, block)
		if err != nil {
			klog.Errorf("failed to reply: %v", err)
		}
	case "getTransaction":
		params, err := parseGetTransactionRequest(req.Params)
		if err != nil {
			klog.Errorf("failed to parse params: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		sig := params.Signature

		transactionNode, err := ser.GetTransaction(ctx, sig)
		if err != nil {
			klog.Errorf("failed to decode Transaction: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}

		var GetTransactionResponse struct {
			// TODO: use same format as solana-core
			Blocktime   *int64 `json:"blockTime"`
			Meta        any    `json:"meta"`
			Slot        uint64 `json:"slot"`
			Transaction []any  `json:"transaction"`
			Version     string `json:"version"`
		}

		GetTransactionResponse.Slot = uint64(transactionNode.Slot)
		{
			block, err := ser.GetBlock(ctx, uint64(transactionNode.Slot))
			if err != nil {
				klog.Errorf("failed to decode block: %v", err)
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "internal error",
					})
				return
			}
			blocktime := int64(block.Meta.Blocktime)
			if blocktime != 0 {
				GetTransactionResponse.Blocktime = &blocktime
			}
		}

		{
			var tx solana.Transaction
			if err := bin.UnmarshalBin(&tx, transactionNode.Data); err != nil {
				klog.Errorf("failed to unmarshal transaction: %v", err)
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "internal error",
					})
				return
			} else if len(tx.Signatures) == 0 {
				klog.Errorf("transaction has no signatures")
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "internal error",
					})
				return
			}
			if tx.Message.IsVersioned() {
				// TODO: use the actual version
				GetTransactionResponse.Version = fmt.Sprintf("%d", tx.Message.GetVersion())
			} else {
				GetTransactionResponse.Version = "legacy"
			}

			b64Tx, err := tx.ToBase64()
			if err != nil {
				klog.Errorf("failed to encode transaction: %v", err)
				conn.ReplyWithError(
					ctx,
					req.ID,
					&jsonrpc2.Error{
						Code:    jsonrpc2.CodeInternalError,
						Message: "internal error",
					})
				return
			}

			GetTransactionResponse.Transaction = []any{b64Tx, "base64"}

			if len(transactionNode.Metadata) > 0 {
				uncompressedMeta, err := decodeZstd(transactionNode.Metadata)
				if err != nil {
					klog.Errorf("failed to decompress metadata: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "internal error",
						})
					return
				}
				status, err := blockstore.ParseAnyTransactionStatusMeta(uncompressedMeta)
				if err != nil {
					klog.Errorf("failed to parse metadata: %v", err)
					conn.ReplyWithError(
						ctx,
						req.ID,
						&jsonrpc2.Error{
							Code:    jsonrpc2.CodeInternalError,
							Message: "internal error",
						})
					return
				}
				GetTransactionResponse.Meta = status
			}
		}

		// reply with the data
		err = conn.Reply(ctx, req.ID, GetTransactionResponse)
		if err != nil {
			klog.Errorf("failed to reply: %v", err)
		}
	default:
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeMethodNotFound,
				Message: "method not found",
			})
	}
}
