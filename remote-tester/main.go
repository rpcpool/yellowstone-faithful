package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/uri"
	"github.com/yudai/gojsondiff"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

const defaultURLTemplate = "https://files.old-faithful.net/{{.epoch}}/epoch-{{.epoch}}.car"

func renderTemplate(templateStr string, vars map[string]any) (string, error) {
	tmpl, err := template.New("tpl").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func formatRemoteCarFileURL(epoch int, urlTemplate string) string {
	data := map[string]any{
		"epoch": epoch,
	}
	formatted, err := renderTemplate(urlTemplate, data)
	if err != nil {
		panic(fmt.Errorf("failed to render URL template %q with epoch %d: %w", urlTemplate, epoch, err))
	}
	if formatted == "" {
		panic(fmt.Errorf("rendered URL template is empty for epoch %d", epoch))
	}
	return formatted
}

func generateListOfURIs(
	// either the list is provided by the user or
	uris uri.List,
	// or we generate a list of URIs based on the epoch range and the URL template
	startEpoch, endEpoch uint64,
	urlTemplate string,
) (uri.List, error) {
	if len(uris) > 0 {
		fmt.Printf("Using provided URIs: %s\n", uris)
		return uris, nil
	}
	fmt.Printf("Generating URIs for epochs %d to %d using template %q\n", startEpoch, endEpoch, urlTemplate)
	if startEpoch == 0 || endEpoch == 0 {
		return nil, fmt.Errorf("start and end epoch must be set")
	}
	if startEpoch > endEpoch {
		return nil, fmt.Errorf("start epoch must be less than end epoch")
	}
	var list uri.List
	rangeSlice := generateRange(int(startEpoch), int(endEpoch))
	for _, epoch := range rangeSlice {
		formattedURL := formatRemoteCarFileURL(epoch, urlTemplate)
		if formattedURL == "" {
			return nil, fmt.Errorf("formatted URL is empty for epoch %d", epoch)
		}
		list = append(list, uri.New(formattedURL))
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no URIs generated for the given epoch range %d-%d", startEpoch, endEpoch)
	}
	return list, nil
}

func main() {
	var startEpoch, endEpoch uint64
	flag.Uint64Var(&startEpoch, "start", 0, "Start epoch")
	flag.Uint64Var(&endEpoch, "end", 0, "End epoch")
	var rpcURL string
	flag.StringVar(&rpcURL, "rpc", "https://api.mainnet-beta.solana.com", "RPC URL")
	var limitSlots int
	flag.IntVar(&limitSlots, "slots", 0, "How many slots to process per epoch (0 means no limit)")
	var stopOnDiff bool
	flag.BoolVar(&stopOnDiff, "stop-on-diff", true, "Stop on diff")
	var uris uri.List
	flag.Var((*uri.List)(&uris), "uri", "URI to a CAR file or directory (can be specified multiple times)")
	var urlTemplate string
	flag.StringVar(&urlTemplate, "url-template", defaultURLTemplate, "URL template for remote CAR files (e.g. https://files.old-faithful.net/{{.epoch}}/epoch-{{.epoch}}.car)")
	flag.Parse()

	client := NewHTTP(
		rpcURL,
		&http.Client{
			Timeout: 15 * time.Second,
		},
	)

	format := solana.EncodingJSON

	uris, err := generateListOfURIs(
		uris,
		startEpoch,
		endEpoch,
		urlTemplate,
	)
	if err != nil {
		panic(fmt.Errorf("failed to generate list of URIs: %w", err))
	}
	firstEpochWithTxMetadata := uint64(10)

	for _, carURI := range uris {
		fmt.Printf("Processing epoch from %q\n", carURI)

		rao, err := openURI(carURI.String())
		if err != nil {
			panic(fmt.Errorf("failed to create multi reader from %q: %w", carURI, err))
		}

		numSlotsSeen := new(atomic.Uint64)

		accum := accum.NewObjectAccumulator(
			rao,
			iplddecoders.KindBlock,
			accum.IgnoreKinds(
				// Ignore these kinds in the accumulator (only need Transactions and DataFrames):
				iplddecoders.KindEntry,
				iplddecoders.KindRewards,
			),
			func(blockObject *accum.ObjectWithMetadata, dagObjects accum.ObjectsWithMetadata) error {
				numSlotsSeen.Add(1)

				rawData := blockObject.ObjectData.Bytes()

				block, err := iplddecoders.DecodeBlock(rawData)
				if err != nil {
					return fmt.Errorf("error while decoding block: %w", err)
				}
				epoch := slottools.CalcEpochForSlot(uint64(block.Slot))
				// Process the objects here
				// For example, you can print the number of objects
				slog.Info(
					"Processing block",
					"slot", block.Slot,
					"epoch", epoch,
					"numDagChildren", len(dagObjects),
				)

				transactions, err := accum.ObjectsToTransactionsAndMetadata(block, dagObjects)
				if err != nil {
					return fmt.Errorf("error while converting objects to transactions: %w", err)
				}
				defer accum.PutTransactionWithSlotSlice(transactions)

				batchSize := 100
				batches := IntoBatchesOf(batchSize, transactions)
				fmt.Printf("Slot %d: Split %d tx into %d batches\n", block.Slot, len(transactions), len(batches))

				for _, batch := range batches {
					fmt.Print("⬆️")
					txIDs := make([]solana.Signature, len(batch))
					for i, tx := range batch {
						txIDs[i] = tx.Transaction.Signatures[0]
					}
					txJsons, err := client.GetTransactionBatch(
						context.Background(),
						format,
						txIDs,
					)
					if err != nil {
						panic(fmt.Errorf("failed to get transaction batch: %w", err))
					}
					fmt.Print("⤵️")
					for i, txJson := range txJsons {
						txWithInfo := batch[i]
						if len(txJson) == 0 || bytes.Equal(txJson, []byte("null")) {
							fmt.Printf("Transaction %s not found\n", txWithInfo.Transaction.Signatures[0])
							continue
						}
					}
					for ii := range batch {
						txWithInfo := batch[ii]
						rpcJson := txJsons[ii]
						sig := txWithInfo.Transaction.Signatures[0]
						hasMeta := txWithInfo.Metadata != nil // We include this to know whether isSuccess is valid.
						if !hasMeta && epoch > firstEpochWithTxMetadata {
							fmt.Printf("Transaction %s has no metadata\n", sig)
							spew.Dump(txWithInfo.Error, txWithInfo.IsMetaParseError())
							spew.Dump(txWithInfo)
							panic(fmt.Errorf("transaction %s has no metadata", sig))
						}

						{
							uiBoth := solanatxmetaparsers.NewEncodedTransactionWithStatusMeta(
								txWithInfo.Transaction,
								txWithInfo.Metadata,
							)
							gotUi, err := uiBoth.ToUi(format)
							if err != nil {
								panic(fmt.Errorf("tx %s : failed to convert to UI: %w", sig, err))
							}
							gotUi.Uint("slot", txWithInfo.Slot)
							if block.Meta.Blocktime == 0 {
								gotUi.Null("blockTime")
							} else {
								gotUi.Int("blockTime", int64(block.Meta.Blocktime))
							}
							{
								carJson, err := gotUi.MarshalJSON()
								if err != nil {
									panic(fmt.Errorf("tx %s : failed to marshal JSON: %w", sig, err))
								}
								if numSlotsSeen.Load()%100 == 0 {
									fmt.Println(string(carJson))
								}

								{
									differ := diff.New()
									d, err := differ.Compare(rpcJson, (carJson))
									if err != nil {
										panic(fmt.Errorf("tx %s : failed to compare JSON: %w", sig, err))
									}
									fmt.Print(".")
									if d.Modified() {
										{
											// ignore meta.postTokenBalances[#].uiAmount
											{
											}
										}
										{
											// Skip known differences:
											if len(d.Deltas()) == 1 && fmt.Sprint(d.Deltas()[0]) == "blockTime" {
												continue
											}
											// if second delta is *gojsondiff.Object
											if len(d.Deltas()) > 1 {
												if o, ok := d.Deltas()[1].(*gojsondiff.Object); ok {
													if isOneOf(
														fmt.Sprint(o.Deltas[0]),
														"innerInstructions",
														"logMessages",
														"computeUnitsConsumed",
														"postTokenBalances",
														"preTokenBalances",
													) {
														continue
													}
												}
											}
											if len(d.Deltas()) == 1 {
												if o, ok := d.Deltas()[0].(*gojsondiff.Object); ok {
													if isOneOf(
														fmt.Sprint(o.Deltas[0]),
														"innerInstructions",
														"logMessages",
														"computeUnitsConsumed",
														"postTokenBalances",
														"preTokenBalances",
													) {
														continue
													}
												}
											}
											if len(d.Deltas()) == 1 && fmt.Sprint(d.Deltas()[0]) == "version" {
												continue
											}
											for _, delta := range d.Deltas() {
												spew.Dump(delta)
											}
										}
										if txWithInfo.Metadata.IsSerde() {
											fmt.Printf("Transaction %s has meta encoded in serde\n", sig)
										}
										if txWithInfo.Metadata.IsProtobuf() {
											fmt.Printf("Transaction %s has meta encoded in protobuf\n", sig)
										}
										if txWithInfo.IsMetaParseError() {
											fmt.Printf("Transaction %s has meta parse error\n", sig)
										}
										fmt.Println("CAR:", string(carJson))
										fmt.Println("RPC:", string(rpcJson))
										format := "ascii"
										var diffString string
										if format == "ascii" {
											var aJson map[string]interface{}
											json.Unmarshal(rpcJson, &aJson)

											config := formatter.AsciiFormatterConfig{
												ShowArrayIndex: true,
												Coloring:       true,
											}

											formatter := formatter.NewAsciiFormatter(aJson, config)
											diffString, err = formatter.Format(d)
											if err != nil {
												// No error can occur
											}
										} else if format == "delta" {
											formatter := formatter.NewDeltaFormatter()
											diffString, err = formatter.Format(d)
											if err != nil {
												// No error can occur
											}
										} else {
											fmt.Printf("Unknown Foramt %s\n", format)
											os.Exit(4)
										}

										fmt.Print(diffString)
										if stopOnDiff {
											fmt.Printf("Stopping on diff for transaction %s\n", sig)
											os.Exit(1)
										}
									}
								}
							}
						}
					}
				}

				if limitSlots > 0 && numSlotsSeen.Load() >= uint64(limitSlots) && len(transactions) > 0 {
					fmt.Printf("Limit of %d slots per epoch %d reached\n", limitSlots, epoch)
					return accum.ErrStop
				}
				return nil
			},
		)

		if err := accum.Run(context.Background()); err != nil {
			// if is context.Canceled, we just ignore it
			if errors.Is(err, context.Canceled) {
				fmt.Printf("Accumulation stopped\n")
				continue
			}
			panic(fmt.Errorf("error while accumulating objects: %w", err))
		}

	}
}

func openURI(pathOrURL string) (*readasonecar.MultiReader, error) {
	uri_ := uri.New(pathOrURL)
	if uri_.IsZero() || !uri_.IsValid() || (!uri_.IsFile() && !uri_.IsWeb()) {
		return nil, fmt.Errorf("invalid path or URL: %s", pathOrURL)
	}
	if uri_.IsFile() {
		return readasonecar.NewFromFilepaths(pathOrURL)
	}
	return readasonecar.NewFromURLs(pathOrURL)
}

type RawJsonClient struct {
	rpcURL     string
	httpClient *http.Client
}

func NewHTTP(
	rpcURL string,
	client *http.Client,
) *RawJsonClient {
	return &RawJsonClient{
		rpcURL:     rpcURL,
		httpClient: client,
	}
}

func IntoBatchesOf[T any](
	batchSize int,
	items []T,
) [][]T {
	if batchSize <= 0 {
		panic("batch size must be greater than 0")
	}
	if len(items) == 0 {
		return nil
	}
	batches := make([][]T, 0, (len(items)+batchSize-1)/batchSize)
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}
	return batches
}

func (c *RawJsonClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error: %s", body)
	}
	return resp, nil
}

func (c *RawJsonClient) jsonRPC(ctx context.Context, method string, params interface{}) (*http.Response, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.rpcURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *RawJsonClient) GetTransaction(
	ctx context.Context,
	format solana.EncodingType,
	txID solana.Signature,
) ([]byte, error) {
	return retry(ctx, func(ctx context.Context) ([]byte, error) {
		resp, err := c._getTransaction(ctx, format, txID)
		if err != nil {
			return nil, err
		}
		if len(resp) == 0 || bytes.Equal(resp, []byte("null")) {
			return nil, fmt.Errorf("no result found")
		}
		return resp, nil
	}, 3)
}

func retry[T any](
	ctx context.Context,
	fn func(ctx context.Context) (T, error),
	retries int,
) (T, error) {
	var result T
	var err error
	sleep := time.Second
	for i := 0; i < retries; i++ {
		result, err = fn(ctx)
		if err == nil {
			return result, nil
		}
		time.Sleep(sleep)
		sleep *= 2
		fmt.Printf("Retrying... %d/%d\n", i+1, retries)
	}
	return result, err
}

func (c *RawJsonClient) _getTransaction(
	ctx context.Context,
	format solana.EncodingType,
	txID solana.Signature,
) ([]byte, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "getTransaction",
		"params":  []interface{}{txID, map[string]interface{}{"encoding": format, "maxSupportedTransactionVersion": 0}},
		"id":      1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.rpcURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", result.Error.Message)
	}
	if result.Result == nil {
		return nil, fmt.Errorf("no result found")
	}
	return result.Result, nil
}

func (c *RawJsonClient) GetTransactionBatch(
	ctx context.Context,
	format solana.EncodingType,
	txIDs []solana.Signature,
) ([]json.RawMessage, error) {
	return retry(ctx, func(ctx context.Context) ([]json.RawMessage, error) {
		resp, err := c._getTransaction_batch(ctx, format, txIDs)
		if err != nil {
			return nil, err
		}
		if len(resp) == 0 || bytes.Equal(resp[0], []byte("null")) {
			return nil, fmt.Errorf("no result found")
		}
		return resp, nil
	}, 3)
}

func (c *RawJsonClient) _getTransaction_batch(
	ctx context.Context,
	format solana.EncodingType,
	txIDs []solana.Signature,
) ([]json.RawMessage, error) {
	reqBody := []map[string]interface{}{}
	for _, txID := range txIDs {
		reqBody = append(reqBody, map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "getTransaction",
			"params":  []interface{}{txID, map[string]interface{}{"encoding": format, "maxSupportedTransactionVersion": 0}},
			"id":      1,
		})
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.rpcURL, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var results []json.RawMessage
	for _, res := range result {
		if res.Error != nil {
			return nil, fmt.Errorf("RPC error: %s", res.Error.Message)
		}
		if res.Result == nil {
			return nil, fmt.Errorf("no result found")
		}
		results = append(results, res.Result)
	}
	return results, nil
}

func generateRange(start, end int) []int {
	// Create a slice to hold the range of integers
	rangeSlice := make([]int, end-start+1)

	// Fill the slice with the range of integers
	for i := start; i <= end; i++ {
		rangeSlice[i-start] = i
	}

	return rangeSlice
}

func isOneOf[T comparable](value T, values ...T) bool {
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}
