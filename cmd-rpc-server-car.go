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

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/urfave/cli/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"go.firedancer.io/radiance/pkg/blockstore"
	"go.firedancer.io/radiance/third_party/solana_proto/confirmed_block"
	"k8s.io/klog/v2"
)

func newCmd_rpcServerCar() *cli.Command {
	return &cli.Command{
		Name:        "rpc-server-car",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<car-path> <cid-to-offset-index-filepath> <slot-to-cid-index-filepath> <sig-to-cid-index-filepath>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
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

	http.HandleFunc("/", newRPC(handler))

	listenOn := ":8899"
	klog.Infof("Listening on %s", listenOn)
	return http.ListenAndServe(listenOn, nil)
}

func newRPC(handler *rpcServer) func(resp http.ResponseWriter, req *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		// read request body
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			klog.Errorf("failed to read request body: %v", err)
			return
		}

		// parse request
		var rpcRequest jsonrpc2.Request
		if err := json.Unmarshal(body, &rpcRequest); err != nil {
			klog.Errorf("failed to unmarshal request: %v", err)
			return
		}

		klog.Infof("request: %s", string(body))

		rf := &fakeConn{resp: resp}

		// handle request
		handler.Handle(context.Background(), rf, &rpcRequest)
	}
}

type responseWriter struct {
	http.ResponseWriter
}

func (w *responseWriter) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (w *responseWriter) Write(p []byte) (n int, err error) {
	return w.ResponseWriter.Write(p)
}

func (w *responseWriter) Close() error {
	return nil
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

type fakeConn struct {
	resp http.ResponseWriter
}

// ReplyWithError(ctx context.Context, id ID, respErr *Error) error {
func (c *fakeConn) ReplyWithError(ctx context.Context, id jsonrpc2.ID, respErr *jsonrpc2.Error) error {
	resp := &jsonrpc2.Response{
		ID:    id,
		Error: respErr,
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	c.resp.Header().Set("Content-Type", "application/json")
	_, err = c.resp.Write(respBytes)
	return err
}

// Reply(ctx context.Context, id ID, result interface{}) error {
func (c *fakeConn) Reply(ctx context.Context, id jsonrpc2.ID, result interface{}) error {
	resRaw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	// set content type
	c.resp.Header().Set("Content-Type", "application/json")
	_, err = c.resp.Write(respBytes)
	return err
}

// jsonrpc2.RequestHandler interface
func (s *rpcServer) Handle(ctx context.Context, conn *fakeConn, req *jsonrpc2.Request) {
	switch req.Method {
	case "getBlock":
		// get first argument
		var params []any
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			klog.Errorf("failed to unmarshal params: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		slotRaw, ok := params[0].(json.Number)
		if !ok {
			klog.Errorf("first argument must be a number")
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		slot, err := slotRaw.Int64()
		if err != nil {
			klog.Errorf("failed to convert slot to int64: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		// get CID for slot
		wantedCid, err := findCidFromSlot(s.slotToCidIndex, uint64(slot))
		if err != nil {
			klog.Errorf("failed to find CID for slot %d: %v", slot, err)
			// not found or error
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}
		// get offset for CID
		offset, err := findOffsetFromCid(s.cidToOffsetIndex, wantedCid)
		if err != nil {
			klog.Errorf("failed to find offset for CID %s: %v", wantedCid, err)
			// not found or error
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}
		// seek to offset
		dr, err := s.carReader.DataReader()
		if err != nil {
			klog.Errorf("failed to get data reader: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}
		dr.Seek(int64(offset), io.SeekStart)
		br := bufio.NewReader(dr)

		gotCid, data, err := util.ReadNode(br)
		if err != nil {
			klog.Errorf("failed to read node: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}
		// verify that the CID we read matches the one we expected.
		if !gotCid.Equals(wantedCid) {
			klog.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}
		// try parsing the data as an Epoch node.
		decoded, err := iplddecoders.DecodeBlock(data)
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
		spew.Dump(decoded)
		// reply with the data
		err = conn.Reply(ctx, req.ID, data)
		if err != nil {
			klog.Errorf("failed to reply: %v", err)
		}
	case "getTransaction":
		// get first argument
		var params []any
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			klog.Errorf("failed to unmarshal params: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		sig, ok := params[0].(string)
		if !ok {
			klog.Errorf("first argument is not a string: %T", params[0])
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		parsedSig, err := solana.SignatureFromBase58(sig)
		if err != nil {
			klog.Errorf("failed to parse signature: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInvalidParams,
					Message: "invalid params",
				})
			return
		}

		// get CID for signature
		wantedCid, err := findCidFromSignature(s.sigToCidIndex, parsedSig)
		if err != nil {
			klog.Errorf("failed to find CID for signature %s: %v", parsedSig, err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}

		// get offset for CID
		offset, err := findOffsetFromCid(s.cidToOffsetIndex, wantedCid)
		if err != nil {
			klog.Errorf("failed to find offset for CID %s: %v", wantedCid, err)

			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}

		// seek to offset
		dr, err := s.carReader.DataReader()
		if err != nil {
			klog.Errorf("failed to get data reader: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}
		dr.Seek(int64(offset), io.SeekStart)
		br := bufio.NewReader(dr)

		gotCid, data, err := util.ReadNode(br)
		if err != nil {
			klog.Errorf("failed to read node: %v", err)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}

		// verify that the CID we read matches the one we expected.
		if !gotCid.Equals(wantedCid) {
			klog.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
			conn.ReplyWithError(
				ctx,
				req.ID,
				&jsonrpc2.Error{
					Code:    jsonrpc2.CodeInternalError,
					Message: "internal error",
				})
			return
		}

		// try parsing the data as a Transaction node.
		decoded, err := iplddecoders.DecodeTransaction(data)
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
		spew.Dump(decoded)

		var txResponse struct {
			// TODO: use same format as solana-core
			Transaction []any                                  `json:"transaction"`
			Meta        *confirmed_block.TransactionStatusMeta `json:"meta"`
		}

		{
			var tx solana.Transaction
			if err := bin.UnmarshalBin(&tx, decoded.Data); err != nil {
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

			txResponse.Transaction = []any{b64Tx}

			if len(decoded.Metadata) > 0 {
				uncompressedMeta, err := decodeZstd(decoded.Metadata)
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
				status, err := blockstore.ParseTransactionStatusMeta(uncompressedMeta)
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
				txResponse.Meta = status
			}
		}

		// reply with the data
		err = conn.Reply(ctx, req.ID, txResponse)
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
