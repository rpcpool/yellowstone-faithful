package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/downloader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	txpool "github.com/rpcpool/yellowstone-faithful/tx-pool"
	"github.com/rpcpool/yellowstone-faithful/uri"
)

type MultiCounter struct {
	mu       sync.Mutex
	counters map[solana.PK]*atomic.Uint64
}

func NewMultiCounter() *MultiCounter {
	return &MultiCounter{
		counters: make(map[solana.PK]*atomic.Uint64),
	}
}

func (mc *MultiCounter) Add(key solana.PK, delta uint64) uint64 {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.counters == nil {
		mc.counters = make(map[solana.PK]*atomic.Uint64)
	}
	counter, exists := mc.counters[key]
	if !exists {
		counter = new(atomic.Uint64)
		mc.counters[key] = counter
	}
	return counter.Add(delta)
}

func (mc *MultiCounter) Load(key solana.PK) uint64 {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.counters == nil {
		return 0
	}
	counter, exists := mc.counters[key]
	if !exists {
		return 0
	}
	return counter.Load()
}

// getall key value pairs
func (mc *MultiCounter) GetAll() map[solana.PK]uint64 {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	result := make(map[solana.PK]uint64)
	for key, counter := range mc.counters {
		result[key] = counter.Load()
	}
	return result
}

func main() {
	var carpath string
	var jsonResults bool
	flag.StringVar(&carpath, "car", "", "Path to the CAR file")
	flag.BoolVar(&jsonResults, "json", false, "Output results as JSON lines")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -car <path-to-car> <pubkey1> [<pubkey2> ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if carpath == "" {
		flag.Usage()
		return
	}
	pubkeyStrings := flag.Args()
	if len(pubkeyStrings) == 0 {
		fmt.Println("Please provide at least one Solana public key to filter for.")
		flag.Usage()
		return
	}
	var pubkeys solana.PublicKeySlice
	for _, pkStr := range pubkeyStrings {
		pk, err := solana.PublicKeyFromBase58(pkStr)
		if err != nil {
			fmt.Printf("Invalid Solana public key: %s\n", pkStr)
			return
		}
		pubkeys = append(pubkeys, pk)
	}
	if len(pubkeys) == 0 {
		fmt.Println("No valid Solana public keys provided.")
		return
	}
	fmt.Printf("Filtering for %d public keys:\n", len(pubkeys))
	for _, pk := range pubkeys {
		fmt.Printf(" - %s\n", pk.String())
	}

	// default slog to stderr
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	slog.Info("Going to walk each block in the CAR file and print as JSON line",
		"car", carpath,
	)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	highestSlotCar := new(atomic.Uint64)
	numBlocksReadCar := new(atomic.Uint64)
	startedAt := time.Now()
	counterTx := new(atomic.Uint64)

	mcSuccess := NewMultiCounter()
	mcFail := NewMultiCounter()
	finished := new(atomic.Bool)
	defer func() {
		slog.Info("Final stats",

			"duration", time.Since(startedAt).Round(time.Second),
			"blocksRead", humanize.Comma(int64(numBlocksReadCar.Load())),
			"highestSlot", humanize.Comma(int64(highestSlotCar.Load())),
			"speed", fmt.Sprintf("%v/s", (uint64(float64(numBlocksReadCar.Load())/time.Since(startedAt).Seconds()))),
			"car", carpath,
			"counterTx", humanize.Comma(int64(counterTx.Load())),
		)
		if jsonResults {
			type Result struct {
				Pubkey          string `json:"pubkey"`
				CountTxSuccess  uint64 `json:"count_tx_success"`
				CountTxReverted uint64 `json:"count_tx_reverted"`
			}
			var results []Result
			allSuccess := mcSuccess.GetAll()
			allFail := mcFail.GetAll()
			keysMap := make(map[solana.PK]struct{})
			for k := range allSuccess {
				keysMap[k] = struct{}{}
			}
			for k := range allFail {
				keysMap[k] = struct{}{}
			}
			var keys []solana.PK
			for k := range keysMap {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return keys[i].String() < keys[j].String()
			})
			for _, pk := range keys {
				results = append(results, Result{
					Pubkey:          pk.String(),
					CountTxSuccess:  allSuccess[pk],
					CountTxReverted: allFail[pk],
				})
			}
			// print one object with all results
			output := map[string]any{
				"success":                  finished.Load(),
				"epoch":                    slottools.CalcEpochForSlot(highestSlotCar.Load()),
				"num_blocks_checked":       numBlocksReadCar.Load(),
				"num_transactions_checked": counterTx.Load(),
				"keys_searched":            pubkeys,
				"results":                  results,
			}
			b, err := json.Marshal(output)
			if err != nil {
				slog.Error("Failed to marshal results to JSON", "error", err)
				return
			}
			fmt.Println(string(b))
		} else {
			fmt.Println("Success/revert by key")
			fmt.Println("Keys considered:")
			for _, pk := range pubkeys {
				fmt.Printf(" - %s\n", pk.String())
			}
			fmt.Println("Epoch surveyed:", slottools.CalcEpochForSlot(highestSlotCar.Load()))
			fmt.Println("Number of slots surveyed:", numBlocksReadCar.Load())
			fmt.Println("Number of transactions surveyed:", counterTx.Load())
			fmt.Println("Finished:", finished.Load())
			fmt.Println("Results:")

			// aggregate success and failure
			type keyStat struct {
				Key     solana.PK
				Success uint64
				Fail    uint64
			}
			var stats []keyStat
			seenKeys := make(map[solana.PK]struct{})
			for key := range mcSuccess.GetAll() {
				seenKeys[key] = struct{}{}
			}
			for key := range mcFail.GetAll() {
				seenKeys[key] = struct{}{}
			}
			for key := range seenKeys {
				stats = append(stats, keyStat{
					Key:     key,
					Success: mcSuccess.Load(key),
					Fail:    mcFail.Load(key),
				})
			}
			// sort by total count desc
			sort.Slice(stats, func(i, j int) bool {
				return (stats[i].Success + stats[i].Fail) > (stats[j].Success + stats[j].Fail)
			})
			for _, stat := range stats {
				fmt.Printf(" - %s: success=%s revert=%s total=%s\n",
					stat.Key.String(),
					humanize.Comma(int64(stat.Success)),
					humanize.Comma(int64(stat.Fail)),
					humanize.Comma(int64(stat.Success+stat.Fail)),
				)
			}
		}
	}()

	reader, bytecounter, err := openURI(carpath)
	if err != nil {
		slog.Error("Failed to open CAR file", "error", err, "carpath", carpath)
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(startedAt).Round(time.Second)
				blocks := numBlocksReadCar.Load()
				etaFor432000Blocks := time.Duration(0)
				if blocks > 0 {
					etaFor432000Blocks = time.Duration(float64(elapsed) * (432000.0 / float64(blocks))).Round(time.Second)
				}
				if etaFor432000Blocks > 0 {
					etaFor432000Blocks = etaFor432000Blocks - elapsed
				}
				if etaFor432000Blocks < 0 {
					etaFor432000Blocks = 0
				}
				slog.Info(
					"Progress report",
					"blocksRead", humanize.Comma(int64(blocks)),
					"highestSlot", humanize.Comma(int64(highestSlotCar.Load())),
					"elapsed", elapsed,
					"etaFor432000Blocks", etaFor432000Blocks,
					"speed", fmt.Sprintf("%v/s", (uint64(float64(blocks)/elapsed.Seconds()))),
					"speedDL", fmt.Sprintf("%v/s", humanize.Bytes(uint64(float64(bytecounter.Load())/elapsed.Seconds()))),
					"car", carpath,
					"readBytes", func() string {
						if bytecounter != nil {
							return humanize.Bytes(bytecounter.Load())
						}
						return "N/A"
					}(),
					"txChecked", humanize.Comma(int64(counterTx.Load())),
				)
			case <-ctx.Done():
				slog.Info("Stopping progress reporting")
				signal.Reset(os.Interrupt)
				return
			}
		}
	}()

	walker, err := NewBlockWalker(reader, func(singleBlockDag *nodetools.DataAndCidSlice) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		defer singleBlockDag.Put() // ensure resources are released
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
		_ = parentSlot
		_ = epochNumber

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

		{
			transactions := parsedNodes.SortedTransactions()
			for _, transactionNode := range transactions {
				tx, meta, err := nodetools.GetTransactionAndMeta(parsedNodes, transactionNode)
				if err != nil {
					slog.Error("Failed to get transaction and meta from node", "error", err, "carpath", carpath)
					panic(fmt.Sprintf("Fatal: Failed to get transaction and meta from node: %v", err))
				}
				{
					accountKeys := tx.Message.AccountKeys
					if meta != nil && meta.IsProtobuf() {
						meta := meta.GetProtobuf()
						accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedReadonlyAddresses)...)
						accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedWritableAddresses)...)
					}
					accountKeys = accountKeys.Dedupe()
					for _, pk := range accountKeys {
						if pubkeys.Contains(pk) {
							// found a matching pubkey
							counterTx.Add(1)
							if meta.IsErr() {
								mcFail.Add(pk, 1)
							} else {
								mcSuccess.Add(pk, 1)
							}
						}
					}

				}
				{

					meta.Put()
					txpool.Put(tx) // return the transaction to the pool
				}
			}
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
			finished.Store(true)
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
		// client := NewClient(nil)
		// stream, err := NewResilientStream(client, pathOrURL, 3, time.Second*5)
		// if err != nil {
		// 	return nil, nil, fmt.Errorf("failed to get stream from %q: %w", pathOrURL, err)
		// }
		downloader, err := downloader.NewDownloader(pathOrURL, runtime.NumCPU(), 4*MiB)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create downloader from %q: %w", pathOrURL, err)
		}
		reader, err := downloader.Download()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to start download from %q: %w", pathOrURL, err)
		}
		bytecounter := new(atomic.Uint64)
		countingReader := NewCountingReader(reader, bytecounter)
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

func byteSlicesToKeySlice(keys [][]byte) []solana.PublicKey {
	var out []solana.PublicKey
	for _, key := range keys {
		var k solana.PublicKey
		copy(k[:], key)
		out = append(out, k)
	}
	return out
}
