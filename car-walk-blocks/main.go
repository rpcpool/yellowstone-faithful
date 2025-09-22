package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	txpool "github.com/rpcpool/yellowstone-faithful/tx-pool"
	"github.com/rpcpool/yellowstone-faithful/uri"
	"github.com/valyala/bytebufferpool"
	"k8s.io/klog/v2"
)

func isAnyOf(str string, options ...string) bool {
	for _, option := range options {
		if str == option {
			return true
		}
	}
	return false
}

func main() {
	var carpath string
	var encoding solana.EncodingType
	var transactionDetails rpc.TransactionDetailsType
	var includeRewards bool
	flag.StringVar(&carpath, "car", "", "Path to the CAR file")
	flag.StringVar((*string)(&encoding), "encoding", "base64", "Transaction encoding (base64|json|jsonParsed)")
	flag.StringVar((*string)(&transactionDetails), "details", "signatures", "Transaction details level (none|signatures|full)")
	flag.BoolVar(&includeRewards, "rewards", false, "Include rewards in the output")
	flag.Parse()
	if carpath == "" {
		flag.Usage()
		return
	}
	if !isAnyOf(string(encoding), "base58", "base64", "json", "jsonParsed") {
		slog.Error("Invalid encoding specified. Supported values are: base58, base64, json, jsonParsed", "got", encoding)
		return
	}
	if !isAnyOf(string(transactionDetails), "none", "signatures", "full", "accounts") {
		slog.Error("Invalid transaction details level specified. Supported values are: none, signatures, full, accounts", "got", transactionDetails)
		return
	}

	// default slog to stderr
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	slog.Info("Going to walk each block in the CAR file and print as JSON line",
		"car", carpath,
		"encoding", encoding,
		"tx-details", transactionDetails,
		"include-rewards", includeRewards,
	)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	highestSlotCar := new(atomic.Uint64)
	numBlocksReadCar := new(atomic.Uint64)
	startedAt := time.Now()

	reader, bytecounter, err := openURI(carpath)
	if err != nil {
		slog.Error("Failed to open CAR file", "error", err, "carpath", carpath)
		return
	}
	go func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Info(
					"Progress report",
					"blocksRead", humanize.Comma(int64(numBlocksReadCar.Load())),
					"highestSlot", humanize.Comma(int64(highestSlotCar.Load())),
					"elapsed", time.Since(startedAt).Round(time.Second),
					"car", carpath,
					"readBytes", func() string {
						if bytecounter != nil {
							return humanize.Bytes(bytecounter.Load())
						}
						return "N/A"
					}(),
				)
			case <-ctx.Done():
				slog.Info("Stopping progress reporting")
				signal.Reset(os.Interrupt)
				return
			}
		}
	}()
	slotToBlockHash := make(map[uint64]solana.Hash)
	mu := &sync.RWMutex{}
	getBlockHash := func(slot uint64) (solana.Hash, bool) {
		mu.RLock()
		defer mu.RUnlock()
		hash, ok := slotToBlockHash[slot]
		return hash, ok
	}
	setBlockHash := func(slot uint64, hash solana.Hash) {
		mu.Lock()
		defer mu.Unlock()
		slotToBlockHash[slot] = hash
	}

	walker, err := NewBlockWalker(reader, func(singleBlockDag *nodetools.DataAndCidSlice) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		blocks, err := singleBlockDag.Blocks()
		if err != nil {
			slog.Error("Failed to get blocks from DataAndCidSlice", "error", err, "carpath", carpath)
			panic(fmt.Sprintf("Fatal: Failed to get blocks from DataAndCidSlice: %v", err))
		}
		if len(blocks) != 1 {
			slog.Error("Expected exactly one block in DataAndCidSlice", "numBlocks", len(blocks), "carpath", carpath)
			panic(fmt.Sprintf("Fatal: Expected exactly one block in DataAndCidSlice, got %d", len(blocks)))
		}
		block := blocks[0].Data.(*ipldbindcode.Block)
		slot := block.GetSlot()
		parentSlot := block.GetParentSlot()
		epochNumber := slottools.CalcEpochForSlot(slot)

		highestSlotCar.Store(uint64(block.Slot))
		numBlocksReadCar.Add(1)

		singleBlockDag.SortByCid()
		parsedNodes, err := singleBlockDag.ToParsedAndCidSlice()
		if err != nil {
			panic(err)
		}
		_ = parsedNodes
		defer func() {
			// NOTE: If you use this dag BEYOND this function, you need to remove this line, and call dag.Put() yourself later.
			singleBlockDag.Put()
			parsedNodes.Put()
		}()
		response := jsonbuilder.NewObject()
		defer response.Put() // recycle the response object
		{
			if transactionDetails != rpc.TransactionDetailsNone {
				if transactionDetails == rpc.TransactionDetailsSignatures {
					signatures := make([][]byte, 0, parsedNodes.CountTransactions())
					for transactionNode := range parsedNodes.Transaction() {
						sig, err := tooling.ReadFirstSignature(transactionNode.Data.Data)
						if err != nil {
							panic(fmt.Sprintf("Fatal: Failed to read first signature from transaction: %v", err))
						}
						signatures = append(signatures, sig[:])
					}
					response.Base58Slice("signatures", signatures)
				}
				if transactionDetails == rpc.TransactionDetailsAccounts || transactionDetails == rpc.TransactionDetailsFull {
					allTransactions := make([]*jsonbuilder.OrderedJSONObject, 0, parsedNodes.CountTransactions())
					defer func() {
						for _, tx := range allTransactions {
							tx.Put() // recycle the transaction objects
						}
					}()
					{
						transactions := parsedNodes.SortedTransactions()
						for _, transactionNode := range transactions {
							tx, meta, err := nodetools.GetTransactionAndMeta(parsedNodes, transactionNode)
							if err != nil {
								slog.Error("Failed to get transaction and meta from node", "error", err, "carpath", carpath)
								panic(fmt.Sprintf("Fatal: Failed to get transaction and meta from node: %v", err))
							}
							_ = tx
							_ = meta
							{
								out := solanatxmetaparsers.NewEncodedTransactionWithStatusMeta(
									tx,
									meta,
								)

								txUI, err := out.ToUi(encoding, transactionDetails)
								if err != nil {
									return fmt.Errorf("failed to encode transaction: %v", err)
								}
								{
									txUI.Uint("slot", slot)
									if block.Meta.Blocktime == 0 {
										txUI.Null("blockTime")
									} else {
										txUI.Int("blockTime", block.GetBlocktime())
									}
								}
								out.Meta.Put()
								txpool.Put(tx) // return the transaction to the pool
								// TODO: include position index in the UI output.
								// pos, ok := transactionNode.GetPositionIndex()
								// if ok {
								// 	txUI.Value("position", pos)
								// }
								allTransactions = append(allTransactions, txUI)
							}
						}
					}
					response.Value("transactions", allTransactions)
				}
			}

			// sort.Slice(allTransactions, func(i, j int) bool {
			// 	return allTransactions[i].Position < allTransactions[j].Position
			// })

			response.Uint("slot", slot) // NOTE: adding this because the block itself doesn't have the slot in its fields.
			if slot == 0 {
				response.Uint("blockHeight", 0)

				// NOTE: this applies only to the genesis block
				blockZeroBlocktime := uint64(1584368940)
				response.Uint("blockTime", blockZeroBlocktime)
				response.Uint("parentSlot", uint64(0))

				blockZeroBlockHash := "4sGjMW1sUnHzSxGspuhpqLDx6wiyjNtZAMdL4VZHirAn"
				response.String("previousBlockhash", blockZeroBlockHash)
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
			blocktime := block.GetBlocktime()
			if blocktime != 0 {
				response.Int("blockTime", blocktime)
			}
			lastEntryCid := block.Entries[len(block.Entries)-1]
			lastEntry, err := parsedNodes.EntryByCid(lastEntryCid.(cidlink.Link).Cid)
			if err != nil {
				panic(fmt.Sprintf("Fatal: Failed to get last entry by CID: %v", err))
			}
			lastEntryHash := solana.HashFromBytes(lastEntry.Hash)
			response.String("blockhash", lastEntryHash.String())
			setBlockHash(slot, lastEntryHash)
			var rewardsUi *jsonbuilder.ArrayBuilder
			defer rewardsUi.Put() // recycle the rewards UI array
			hasRewards := block.HasRewards()
			rewardsCid := block.Rewards.(cidlink.Link).Cid
			if includeRewards && hasRewards {
				actualRewards, err := nodetools.GetParsedRewards(parsedNodes, rewardsCid)
				if err != nil {
					slog.Error(
						"failed to parse block rewards",
						"block", slot,
						"rewards_cid", rewardsCid.String(),
						"error", err,
					)
					panic(fmt.Sprintf("Fatal: failed to parse block rewards for block %d, rewards CID %s: %v", slot, rewardsCid.String(), err))
				} else {
					// encode rewards as JSON, then decode it as a map
					rewards, _, err := solanablockrewards.RewardsToUi(actualRewards)
					if err != nil {
						slog.Error(
							"failed to encode block rewards to UI format",
							"block", slot,
							"rewards_cid", rewardsCid.String(),
							"error", err,
						)
						panic(fmt.Sprintf("Fatal: failed to encode block rewards to UI format for block %d, rewards CID %s: %v", slot, rewardsCid.String(), err))
					}
					rewardsUi = rewards
				}
			} else {
				klog.V(4).Infof("rewards not requested or not available")
			}
			if rewardsUi != nil {
				response.Array("rewards", rewardsUi)
			} else {
				response.EmptyArray("rewards")
			}
			{
				if (parentSlot != 0 || slot == 1) && slottools.CalcEpochForSlot(parentSlot) == epochNumber {
					parentHash, ok := getBlockHash(parentSlot)
					if ok {
						response.String("previousBlockhash", parentHash.String())
					} else {
						response.Null("previousBlockhash")
					}
				} else {
					// TODO: handle the case when the parent is in a different epoch.
					if slot != 0 {
						klog.V(4).Infof("parent slot is in a different epoch, not implemented yet (can't get previousBlockhash)")
					}
				}
			}

			encodedResult, err := response.MarshalJSONToByteBuffer()
			if err != nil {
				panic(fmt.Sprintf("Fatal: Failed to encode block JSON: %v", err))
			}
			defer bytebufferpool.Put(encodedResult) // return the buffer to the pool
			// print as a single line
			encodedResult.WriteByte('\n')
			_, _ = os.Stdout.Write(encodedResult.Bytes())
		}

		return nil
	})
	if err != nil {
		slog.Error("Failed to create BlockDAG for CAR file", "error", err, "carpath", carpath)
		panic(fmt.Sprintf("Fatal: Failed to create BlockDAG for CAR file: %v", err))
	}
	header := walker.Header()
	slog.Info(
		"CAR header info",
		"car", carpath,
		"roots", header.Roots,
		"version", header.Version,
	)

	if err := walker.Do(); err != nil {
		if errors.Is(err, io.EOF) {
			slog.Info("Reached end of CAR file", "carpath", carpath)
			cancel()
			return
		}
		slog.Error("Error processing CAR file", "error", err, "carpath", carpath, "numBlocksRead", numBlocksReadCar.Load(), "highestSlot", highestSlotCar.Load())
		cancel()
		return
	}
	slog.Info("Finished processing CAR file",
		"car", carpath,
		"numBlocksRead", numBlocksReadCar.Load(),
		"highestSlot", highestSlotCar.Load(),
	)
	cancel()
}

func openURI(pathOrURL string) (io.ReadCloser, *atomic.Uint64, error) {
	uri_ := uri.New(pathOrURL)
	if uri_.IsZero() || !uri_.IsValid() || (!uri_.IsFile() && !uri_.IsWeb()) {
		return nil, nil, fmt.Errorf("invalid path or URL: %s", pathOrURL)
	}
	if uri_.IsFile() {
		rc, err := os.Open(pathOrURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open file %q: %w", pathOrURL, err)
		}
		bytecounter := new(atomic.Uint64)
		countingReader := NewCountingReader(rc, bytecounter)
		return io.NopCloser(bufio.NewReaderSize(countingReader, MiB*50)), bytecounter, nil
	}
	{
		client := NewClient(nil)
		stream, err := NewResilientStream(client, pathOrURL, 3, time.Second*5)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get stream from %q: %w", pathOrURL, err)
		}
		bytecounter := new(atomic.Uint64)
		countingReader := NewCountingReader(stream, bytecounter)
		buf := bufio.NewReaderSize(countingReader, MiB*50)
		return io.NopCloser(buf), bytecounter, nil
		// return stream, nil
	}
	// {
	// 	rfspc, byteLen, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
	// 		context.Background(),
	// 		pathOrURL,
	// 	)
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", pathOrURL, err)
	// 	}
	// 	sr := io.NewSectionReader(rfspc, 0, byteLen)
	// 	return io.NopCloser(bufio.NewReaderSize(sr, MiB*50)), nil, nil
	// }
}

type CountingReader struct {
	reader io.Reader
	count  *atomic.Uint64
}

func (cr *CountingReader) Read(p []byte) (n int, err error) {
	n, err = cr.reader.Read(p)
	if n > 0 {
		cr.count.Add(uint64(n))
	}
	return n, err
}

func (cr *CountingReader) Close() error {
	if closer, ok := cr.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func NewCountingReader(reader io.Reader, count *atomic.Uint64) *CountingReader {
	return &CountingReader{
		reader: reader,
		count:  count,
	}
}

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)

func NewBlockWalker(readCloser io.ReadCloser, callback func(*nodetools.DataAndCidSlice) error) (*nodetools.BlockDAGs, error) {
	rd, err := carreader.NewPrefetching(readCloser)
	if err != nil {
		return nil, fmt.Errorf("failed to create prefetching car reader: %w", err)
	}
	return nodetools.NewBlockDagFromReader(rd, callback), nil
}

// Client is a wrapper around http.Client for streaming large files.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new streaming client.
// If no httpClient is provided, a default one with a 30-second timeout is used.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			// do dual stack, try http2 first, then http1
			Transport: &http.Transport{
				ForceAttemptHTTP2:     true,
				DisableKeepAlives:     false,
				IdleConnTimeout:       30 * time.Second,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   100,
				DisableCompression:    false,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	}
	return &Client{httpClient: httpClient}
}

// GetStream makes a GET request, returns the body and the response.
func (c *Client) GetStream(url string, offset int64) (io.ReadCloser, *http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	return resp.Body, resp, nil
}

// ResilientStream is an io.ReadCloser that automatically retries on read errors.
type ResilientStream struct {
	client     *Client
	url        string
	maxRetries int
	retryDelay time.Duration

	stream io.ReadCloser
	offset int64 // Total bytes successfully read through this reader
}

// NewResilientStream creates and initializes a stream that will attempt to recover.
func NewResilientStream(c *Client, url string, retries int, delay time.Duration) (*ResilientStream, error) {
	rs := &ResilientStream{
		client:     c,
		url:        url,
		maxRetries: retries,
		retryDelay: delay,
		offset:     0,
	}

	// Make the initial connection.
	if err := rs.reconnect(); err != nil {
		return nil, fmt.Errorf("initial connection failed: %w", err)
	}
	return rs, nil
}

// reconnect handles the logic of establishing or re-establishing the stream.
func (rs *ResilientStream) reconnect() error {
	if rs.stream != nil {
		rs.stream.Close() // Close the old, broken stream.
	}

	stream, resp, err := rs.client.GetStream(rs.url, rs.offset)
	if err != nil {
		return err
	}

	// Verify the server's response.
	// This approach assumes the server correctly supports Range requests.
	if (rs.offset > 0 && resp.StatusCode != http.StatusPartialContent) || (rs.offset == 0 && resp.StatusCode != http.StatusOK) {
		stream.Close()
		return fmt.Errorf("received unexpected status: %s", resp.Status)
	}

	rs.stream = stream
	return nil
}

// Read implements the io.Reader interface. This is where the retry logic lives.
func (rs *ResilientStream) Read(p []byte) (n int, err error) {
	if rs.stream == nil {
		return 0, io.ErrClosedPipe
	}

	// Attempt the initial read.
	n, err = rs.stream.Read(p)
	rs.offset += int64(n)

	// If there's an error (and it's not a clean EOF), start the retry process.
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "\nRead error: %v. Attempting to recover...\n", err)

		for i := 0; i < rs.maxRetries; i++ {
			time.Sleep(rs.retryDelay)
			fmt.Fprintf(os.Stderr, "Retry %d/%d... ", i+1, rs.maxRetries)

			if reconErr := rs.reconnect(); reconErr != nil {
				fmt.Fprintf(os.Stderr, "reconnect failed: %v\n", reconErr)
				continue // Move to the next retry attempt.
			}

			fmt.Fprint(os.Stderr, "reconnected. Retrying read... ")
			n, err = rs.stream.Read(p) // Try reading from the new stream.
			rs.offset += int64(n)

			if err == nil {
				fmt.Fprintln(os.Stderr, "read successful.")
				return n, nil // Success, exit the retry loop and return data.
			}
		}
		// If all retries fail, return the last error.
		return n, fmt.Errorf("read failed after %d retries: %w", rs.maxRetries, err)
	}

	return n, err
}

// Close closes the underlying stream.
func (rs *ResilientStream) Close() error {
	if rs.stream == nil {
		return nil
	}
	return rs.stream.Close()
}
