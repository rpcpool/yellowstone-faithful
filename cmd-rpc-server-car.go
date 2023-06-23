package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	jsoniter "github.com/json-iterator/go"
	"github.com/patrickmn/go-cache"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/urfave/cli/v2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

func newCmd_rpcServerCar() *cli.Command {
	var listenOn string
	return &cli.Command{
		Name:        "rpc-server-car",
		Description: "Start a Solana JSON RPC that exposes getTransaction and getBlock",
		ArgsUsage:   "<car-filepath-or-url> <cid-to-offset-index-filepath-or-url> <slot-to-cid-index-filepath-or-url> <sig-to-cid-index-filepath-or-url> <gsfa-index-dir>",
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
				return cli.Exit("Must provide a CID-to-offset index filepath/url", 1)
			}
			slotToCidIndexFilepath := c.Args().Get(2)
			if slotToCidIndexFilepath == "" {
				return cli.Exit("Must provide a slot-to-CID index filepath/url", 1)
			}
			sigToCidIndexFilepath := c.Args().Get(3)
			if sigToCidIndexFilepath == "" {
				return cli.Exit("Must provide a signature-to-CID index filepath/url", 1)
			}

			cidToOffsetIndexFile, err := openIndexStorage(cidToOffsetIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer cidToOffsetIndexFile.Close()

			cidToOffsetIndex, err := compactindex.Open(cidToOffsetIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			slotToCidIndexFile, err := openIndexStorage(slotToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer slotToCidIndexFile.Close()

			slotToCidIndex, err := compactindex36.Open(slotToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			sigToCidIndexFile, err := openIndexStorage(sigToCidIndexFilepath)
			if err != nil {
				return fmt.Errorf("failed to open index file: %w", err)
			}
			defer sigToCidIndexFile.Close()

			sigToCidIndex, err := compactindex36.Open(sigToCidIndexFile)
			if err != nil {
				return fmt.Errorf("failed to open index: %w", err)
			}

			localCarReader, remoteCarReader, err := openCarStorage(carFilepath)
			if err != nil {
				return fmt.Errorf("failed to open CAR file: %w", err)
			}

			var gsfaIndex *gsfa.GsfaReader
			gsfaIndexDir := c.Args().Get(4)
			if gsfaIndexDir != "" {
				gsfaIndex, err = gsfa.NewGsfaReader(gsfaIndexDir)
				if err != nil {
					return fmt.Errorf("failed to open gsfa index: %w", err)
				}
				defer gsfaIndex.Close()
			}

			return createAndStartRPCServer_withCar(
				c.Context,
				listenOn,
				localCarReader,
				remoteCarReader,
				cidToOffsetIndex,
				slotToCidIndex,
				sigToCidIndex,
				gsfaIndex,
			)
		},
	}
}

// openIndexStorage open a compactindex from a local file, or from a remote URL.
// Supported protocols are:
// - http://
// - https://
func openIndexStorage(where string) (ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if strings.HasPrefix(where, "http://") || strings.HasPrefix(where, "https://") {
		return remoteHTTPFileAsIoReaderAt(where)
	}
	// TODO: add support for IPFS gateways.
	// TODO: add support for Filecoin gateways.
	return os.Open(where)
}

func openCarStorage(where string) (*carv2.Reader, ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if strings.HasPrefix(where, "http://") || strings.HasPrefix(where, "https://") {
		rem, err := remoteHTTPFileAsIoReaderAt(where)
		return nil, rem, err
	}
	// TODO: add support for IPFS gateways.
	// TODO: add support for Filecoin gateways.

	carReader, err := carv2.OpenReader(where)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	return carReader, nil, nil
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

// remoteHTTPFileAsIoReaderAt returns a ReaderAtCloser for a remote file.
// The returned ReaderAtCloser is backed by a http.Client.
func remoteHTTPFileAsIoReaderAt(url string) (ReaderAtCloser, error) {
	// send a request to the server to get the file size:
	resp, err := http.Head(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	contentLength := resp.ContentLength

	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	ca := cache.New(5*time.Minute, 10*time.Minute)

	return &HTTPSingleFileRemoteReaderAt{
		url:           url,
		contentLength: contentLength,
		client:        newHTTPClient(),
		ca:            ca,
	}, nil
}

type HTTPSingleFileRemoteReaderAt struct {
	url           string
	contentLength int64
	client        *http.Client
	// TODO: add caching
	ca *cache.Cache
}

func getCacheKey(off int64, p []byte) string {
	return fmt.Sprintf("%d-%d", off, len(p))
}

func (r *HTTPSingleFileRemoteReaderAt) getFromCache(off int64, p []byte) (n int, err error, has bool) {
	key := getCacheKey(off, p)
	if v, ok := r.ca.Get(key); ok {
		return copy(p, v.([]byte)), nil, true
	}
	return 0, nil, false
}

func (r *HTTPSingleFileRemoteReaderAt) putInCache(off int64, p []byte) {
	key := getCacheKey(off, p)
	r.ca.Set(key, p, cache.DefaultExpiration)
}

// Close implements io.Closer.
func (r *HTTPSingleFileRemoteReaderAt) Close() error {
	r.client.CloseIdleConnections()
	return nil
}

func retryExpotentialBackoff(
	ctx context.Context,
	startDuration time.Duration,
	maxRetries int,
	fn func() error,
) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(startDuration):
			startDuration *= 2
		}
	}
	return fmt.Errorf("failed after %d retries; last error: %w", maxRetries, err)
}

func (r *HTTPSingleFileRemoteReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= r.contentLength {
		return 0, io.EOF
	}
	fmt.Print(".")
	if n, err, has := r.getFromCache(off, p); has {
		return n, err
	}
	req, err := http.NewRequest("GET", r.url, nil)
	if err != nil {
		return 0, err
	}
	{
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Keep-Alive", "timeout=600")
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))))

	var resp *http.Response
	err = retryExpotentialBackoff(
		context.Background(),
		100*time.Millisecond,
		3,
		func() error {
			resp, err = r.client.Do(req)
			return err
		})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	{
		n, err := io.ReadFull(resp.Body, p)
		if err != nil {
			return 0, err
		}
		copyForCache := make([]byte, len(p))
		copy(copyForCache, p)
		r.putInCache(off, copyForCache)
		return n, nil
	}
}

// createAndStartRPCServer_withCar creates and starts a JSON RPC server.
// Data:
//   - Nodes: the node data is read from a CAR file (which can be a local file or a remote URL).
//   - Indexes: the indexes are read from files (which can be a local file or a remote URL).
//
// The server is backed by a CAR file (meaning that it can only serve the content of the CAR file).
// It blocks until the server is stopped.
// It returns an error if the server fails to start or stops unexpectedly.
// It returns nil if the server is stopped gracefully.
func createAndStartRPCServer_withCar(
	ctx context.Context,
	listenOn string,
	carReader *carv2.Reader,
	remoteCarReader ReaderAtCloser,
	cidToOffsetIndex *compactindex.DB,
	slotToCidIndex *compactindex36.DB,
	sigToCidIndex *compactindex36.DB,
	gsfaReader *gsfa.GsfaReader,
) error {
	handler := &rpcServer{
		localCarReader:   carReader,
		remoteCarReader:  remoteCarReader,
		cidToOffsetIndex: cidToOffsetIndex,
		slotToCidIndex:   slotToCidIndex,
		sigToCidIndex:    sigToCidIndex,
		gsfaReader:       gsfaReader,
	}

	h := newRPCHandler_fast(handler)
	h = fasthttp.CompressHandler(h)

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

func createAndStartRPCServer_lassie(
	ctx context.Context,
	listenOn string,
	lassieWr *lassieWrapper,
	slotToCidIndex *compactindex36.DB,
	sigToCidIndex *compactindex36.DB,
	gsfaReader *gsfa.GsfaReader,
) error {
	handler := &rpcServer{
		lassieFetcher:  lassieWr,
		slotToCidIndex: slotToCidIndex,
		sigToCidIndex:  sigToCidIndex,
		gsfaReader:     gsfaReader,
	}

	h := newRPCHandler_fast(handler)
	h = fasthttp.CompressHandler(h)

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

type rpcServer struct {
	lassieFetcher    *lassieWrapper
	localCarReader   *carv2.Reader
	remoteCarReader  ReaderAtCloser
	cidToOffsetIndex *compactindex.DB
	slotToCidIndex   *compactindex36.DB
	sigToCidIndex    *compactindex36.DB
	gsfaReader       *gsfa.GsfaReader
}

type requestContext struct {
	ctx *fasthttp.RequestCtx
}

// ReplyWithError(ctx context.Context, id ID, respErr *Error) error {
func (c *requestContext) ReplyWithError(ctx context.Context, id jsonrpc2.ID, respErr *jsonrpc2.Error) error {
	resp := &jsonrpc2.Response{
		ID:    id,
		Error: respErr,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return nil
}

func toMapAny(v any) (map[string]any, error) {
	b, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// MapToCamelCase converts a map[string]interface{} to a map[string]interface{} with camelCase keys
func MapToCamelCase(m map[string]any) map[string]any {
	newMap := make(map[string]any)
	for k, v := range m {
		newMap[toLowerCamelCase(k)] = MapToCamelCaseAny(v)
	}
	return newMap
}

func MapToCamelCaseAny(m any) any {
	if m == nil {
		return nil
	}
	if m, ok := m.(map[string]any); ok {
		return MapToCamelCase(m)
	}
	// if array, convert each element
	if m, ok := m.([]any); ok {
		for i, v := range m {
			m[i] = MapToCamelCaseAny(v)
		}
	}
	return m
}

func toLowerCamelCase(v string) string {
	pascal := bin.ToPascalCase(v)
	if len(pascal) == 0 {
		return ""
	}
	if len(pascal) == 1 {
		return strings.ToLower(pascal)
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// Reply(ctx context.Context, id ID, result interface{}) error {
func (c *requestContext) Reply(
	ctx context.Context,
	id jsonrpc2.ID,
	result interface{},
	remapCallback func(map[string]any) map[string]any,
) error {
	mm, err := toMapAny(result)
	if err != nil {
		return err
	}
	result = MapToCamelCaseAny(mm)
	if remapCallback != nil {
		if mp, ok := result.(map[string]any); ok {
			result = remapCallback(mp)
		}
	}
	resRaw, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return err
}

func (c *requestContext) ReplyNoMod(
	ctx context.Context,
	id jsonrpc2.ID,
	result interface{},
) error {
	resRaw, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resRaw)
	resp := &jsonrpc2.Response{
		ID:     id,
		Result: &raw,
	}
	replyJSON(c.ctx, http.StatusOK, resp)
	return err
}

func (s *rpcServer) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
	if s.lassieFetcher != nil {
		// Fetch the node from lassie.
		data, err := s.lassieFetcher.GetNodeByCid(ctx, wantedCid)
		if err == nil {
			return data, nil
		}
		klog.Errorf("failed to get node from lassie: %v", err)
		return nil, err
	}
	// Find CAR file offset for CID in index.
	offset, err := s.FindOffsetFromCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find offset for CID %s: %v", wantedCid, err)
		// not found or error
		return nil, err
	}
	return s.GetNodeByOffset(ctx, wantedCid, offset)
}

func readNodeFromReaderAt(reader ReaderAtCloser, wantedCid cid.Cid, offset uint64) ([]byte, error) {
	// read MaxVarintLen64 bytes
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := reader.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return nil, err
	}
	// read uvarint
	dataLen, n := binary.Uvarint(lenBuf)
	offset += uint64(n)
	if dataLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	data := make([]byte, dataLen)
	_, err = reader.ReadAt(data, int64(offset))
	if err != nil {
		return nil, err
	}

	n, gotCid, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// verify that the CID we read matches the one we expected.
	if !gotCid.Equals(wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data[n:], nil
}

func (s *rpcServer) GetNodeByOffset(ctx context.Context, wantedCid cid.Cid, offset uint64) ([]byte, error) {
	if s.localCarReader == nil {
		// try remote reader
		if s.remoteCarReader == nil {
			return nil, fmt.Errorf("no CAR reader available")
		}
		return readNodeFromReaderAt(s.remoteCarReader, wantedCid, offset)
	}
	// Get reader and seek to offset, then read node.
	dr, err := s.localCarReader.DataReader()
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
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
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
	slotRaw, ok := params[0].(float64)
	if !ok {
		klog.Errorf("first argument must be a number, got %T", params[0])
		return nil, nil
	}

	return &GetBlockRequest{
		Slot: uint64(slotRaw),
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
	klog.Infof("found CID for slot %d: %s", slot, wantedCid)
	// get the block by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
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

func (ser *rpcServer) GetEntryByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Entry, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as an Entry node.
	decoded, err := iplddecoders.DecodeEntry(data)
	if err != nil {
		klog.Errorf("failed to decode entry: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *rpcServer) GetTransactionByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Transaction, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
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

func (ser *rpcServer) GetDataFrameByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a DataFrame node.
	decoded, err := iplddecoders.DecodeDataFrame(data)
	if err != nil {
		klog.Errorf("failed to decode data frame: %v", err)
		return nil, err
	}
	return decoded, nil
}

func (ser *rpcServer) GetRewardsByCid(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.Rewards, error) {
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to find node by cid: %v", err)
		return nil, err
	}
	// try parsing the data as a Rewards node.
	decoded, err := iplddecoders.DecodeRewards(data)
	if err != nil {
		klog.Errorf("failed to decode rewards: %v", err)
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
	klog.Infof("found CID for signature %s: %s", sig, wantedCid)
	// get the transaction by CID
	data, err := ser.GetNodeByCid(ctx, wantedCid)
	if err != nil {
		klog.Errorf("failed to get node by cid: %v", err)
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
		ser.getBlock(ctx, conn, req)
	case "getTransaction":
		ser.getTransaction(ctx, conn, req)
	case "getSignaturesForAddress":
		ser.getSignaturesForAddress(ctx, conn, req)
	default:
		conn.ReplyWithError(
			ctx,
			req.ID,
			&jsonrpc2.Error{
				Code:    jsonrpc2.CodeMethodNotFound,
				Message: "Method not found",
			})
	}
}

type GetBlockResponse struct {
	BlockHeight       uint64                   `json:"blockHeight"`
	BlockTime         *uint64                  `json:"blockTime"`
	Blockhash         string                   `json:"blockhash"`
	ParentSlot        uint64                   `json:"parentSlot"`
	PreviousBlockhash string                   `json:"previousBlockhash"`
	Rewards           any                      `json:"rewards"` // TODO: use same format as solana
	Transactions      []GetTransactionResponse `json:"transactions"`
}

type GetTransactionResponse struct {
	// TODO: use same format as solana
	Blocktime   *uint64 `json:"blockTime,omitempty"`
	Meta        any     `json:"meta"`
	Slot        *uint64 `json:"slot,omitempty"`
	Transaction []any   `json:"transaction"`
	Version     any     `json:"version"`
}

func loadDataFromDataFrames(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]byte, error) {
	dataBuffer := new(bytes.Buffer)
	allFrames, err := getAllFramesFromDataFrame(firstDataFrame, dataFrameGetter)
	if err != nil {
		return nil, err
	}
	for _, frame := range allFrames {
		dataBuffer.Write(frame.Bytes())
	}
	// verify the data hash (if present)
	bufHash, ok := firstDataFrame.GetHash()
	if !ok {
		return dataBuffer.Bytes(), nil
	}
	err = ipldbindcode.VerifyHash(dataBuffer.Bytes(), bufHash)
	if err != nil {
		return nil, err
	}
	return dataBuffer.Bytes(), nil
}

func getAllFramesFromDataFrame(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]*ipldbindcode.DataFrame, error) {
	frames := []*ipldbindcode.DataFrame{firstDataFrame}
	// get the next data frames
	next, ok := firstDataFrame.GetNext()
	if !ok || len(next) == 0 {
		return frames, nil
	}
	for _, cid := range next {
		nextDataFrame, err := dataFrameGetter(context.Background(), cid.(cidlink.Link).Cid)
		if err != nil {
			return nil, err
		}
		nextFrames, err := getAllFramesFromDataFrame(nextDataFrame, dataFrameGetter)
		if err != nil {
			return nil, err
		}
		frames = append(frames, nextFrames...)
	}
	return frames, nil
}

func parseTransactionAndMetaFromNode(
	transactionNode *ipldbindcode.Transaction,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) (tx solana.Transaction, meta any, _ error) {
	{
		transactionBuffer, err := loadDataFromDataFrames(&transactionNode.Data, dataFrameGetter)
		if err != nil {
			return solana.Transaction{}, nil, err
		}
		if err := bin.UnmarshalBin(&tx, transactionBuffer); err != nil {
			klog.Errorf("failed to unmarshal transaction: %v", err)
			return solana.Transaction{}, nil, err
		} else if len(tx.Signatures) == 0 {
			klog.Errorf("transaction has no signatures")
			return solana.Transaction{}, nil, err
		}
	}

	{
		metaBuffer, err := loadDataFromDataFrames(&transactionNode.Metadata, dataFrameGetter)
		if err != nil {
			return solana.Transaction{}, nil, err
		}
		if len(metaBuffer) > 0 {
			uncompressedMeta, err := decompressZstd(metaBuffer)
			if err != nil {
				klog.Errorf("failed to decompress metadata: %v", err)
				return
			}
			status, err := solanatxmetaparsers.ParseAnyTransactionStatusMeta(uncompressedMeta)
			if err != nil {
				klog.Errorf("failed to parse metadata: %v", err)
				return
			}
			meta = status
		}
	}
	return
}

func calcBlockHeight(slot uint64) uint64 {
	// TODO: fix this
	return 0
}
