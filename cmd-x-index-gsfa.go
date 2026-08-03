package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"

	"github.com/gagliardetto/solana-go"
)

func newCmd_Index_gsfa() *cli.Command {
	var epoch uint64
	var network indexes.Network
	var pubkeysExclude solana.PublicKeySlice
	return &cli.Command{
		Name:        "gsfa",
		Description: "Create GSFA index from a CAR file",
		ArgsUsage:   "--index-dir=<index-dir> --car=<car-path>",
		Before: func(c *cli.Context) error {
			if network == "" {
				network = indexes.NetworkMainnet
			}
			return nil
		},
		Flags: []cli.Flag{
			// verify hash of transactions:
			&cli.BoolFlag{
				Name:  "verify-hash",
				Usage: "verify hash of transactions",
				Value: false,
			},
			// w number of workers:
			&cli.UintFlag{
				Name:    "workers",
				Aliases: []string{"w"},
				Usage:   "number of workers",
				Value:   uint(runtime.NumCPU()) * 3,
			},
			&cli.Uint64Flag{
				Name:        "epoch",
				Usage:       "epoch",
				Destination: &epoch,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "network",
				Usage:       "network",
				Destination: (*string)(&network),
				Action: func(c *cli.Context, v string) error {
					if !indexes.IsValidNetwork(indexes.Network(v)) {
						return fmt.Errorf("invalid network: %s", v)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: os.TempDir(),
			},
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.StringFlag{
				Name:  "index-dir",
				Usage: "Destination directory for the output files",
			},
			&cli.BoolFlag{
				Name:  "require-tx-metadata",
				Usage: "Require transaction metadata to be present in the CAR file",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "sigverify",
				Usage: "Verify signatures of transactions",
				Value: true,
			},
			&cli.StringSliceFlag{
				Name:  "exclude-pubkey",
				Usage: "Exclude transactions that contain these public keys in their account keys",
				Action: func(c *cli.Context, v []string) error {
					for _, pk := range v {
						parsed, err := solana.PublicKeyFromBase58(pk)
						if err != nil {
							return fmt.Errorf("failed to parse public key %q: %w", pk, err)
						}
						pubkeysExclude = append(pubkeysExclude, parsed)
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "cpuprofile",
				Usage: "write a CPU profile to this file; the profile is flushed on normal completion and also on Ctrl-C (SIGINT/SIGTERM), so you can interrupt a long run and still get a valid profile",
			},
		},
		Action: func(c *cli.Context) error {
			if cpuProfilePath := c.String("cpuprofile"); cpuProfilePath != "" {
				stop, err := startCPUProfile(cpuProfilePath)
				if err != nil {
					return fmt.Errorf("failed to start CPU profile: %w", err)
				}
				// Flush on normal completion.
				defer stop()
				// Flush on Ctrl-C, since a full epoch run is unlikely to finish
				// during a profiling session.
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
				go func() {
					<-sigChan
					klog.Info("Received interrupt; flushing CPU profile and exiting...")
					stop()
					os.Exit(130)
				}()
			}

			carPaths := c.StringSlice("car")
			if len(carPaths) == 0 {
				klog.Exit("Please provide a CAR file")
			}

			rd, err := readasonecar.NewFromFilepaths(carPaths...)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			defer rd.Close()

			indexDir := c.String("index-dir")
			if indexDir == "" {
				klog.Exit("Please provide an --index-dir=<dir to store the index>")
			}
			if ok, err := isDirectory(indexDir); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if err := os.MkdirAll(indexDir, 0o755); err != nil {
						return fmt.Errorf("failed to create index-dir: %w", err)
					} else {
						klog.Infof("Created index-dir: %s", indexDir)
					}
				} else {
					return err
				}
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			rootCID, err := rd.FindRoot()
			if err != nil {
				return fmt.Errorf("failed to find root CID: %w", err)
			}

			// Use the car file name and root CID to name the gsfa index dir:
			gsfaIndexDir := filepath.Join(indexDir, formatIndexDirname_gsfa(
				epoch,
				rootCID,
				network,
			))
			klog.Infof("Creating gsfa index dir at %s", gsfaIndexDir)
			if fileExists, err := isFileOrDirExists(gsfaIndexDir); err != nil {
				return fmt.Errorf("failed to check if gsfa index dir exists: %w", err)
			} else if fileExists {
				if isNonEmpty, err := isDirNonEmpty(gsfaIndexDir); err != nil {
					return fmt.Errorf("failed to check if gsfa index dir is non-empty: %w", err)
				} else if isNonEmpty {
					return fmt.Errorf("gsfa index dir already exists and is not empty: %s", gsfaIndexDir)
				}
			} else {
				err = os.Mkdir(gsfaIndexDir, 0o755)
				if err != nil {
					return fmt.Errorf("failed to create gsfa index dir: %w", err)
				}
			}

			meta := indexmeta.Meta{}
			{
				if err := meta.AddUint64(indexmeta.MetadataKey_Epoch, epoch); err != nil {
					return fmt.Errorf("failed to add epoch to sig_exists index metadata: %w", err)
				}
				if err := meta.AddCid(indexmeta.MetadataKey_RootCid, rootCID); err != nil {
					return fmt.Errorf("failed to add root cid to sig_exists index metadata: %w", err)
				}
				if err := meta.AddString(indexmeta.MetadataKey_Network, string(network)); err != nil {
					return fmt.Errorf("failed to add network to sig_exists index metadata: %w", err)
				}
			}
			tmpDir := c.String("tmp-dir")
			tmpDir, err = os.MkdirTemp(tmpDir, "gsfa_indexer_*")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			indexW, err := gsfa.NewGsfaWriter(
				gsfaIndexDir,
				meta,
				epoch,
				rootCID,
				network,
				tmpDir,
			)
			if err != nil {
				return fmt.Errorf("error while opening gsfa index writer: %w", err)
			}
			numProcessedTransactions := new(atomic.Int64)
			startedAt := time.Now()

			verifyHash := c.Bool("verify-hash")
			sigverify := c.Bool("sigverify")
			ipldbindcode.DisableHashVerification = !verifyHash

			epochStart, epochEnd := slottools.CalcEpochLimits(epoch)

			lastPrintedAt := time.Now()

			numMissingMetadata := new(atomic.Int64)
			numMissingMetadata.Store(0)

			requireTxMetadata := c.Bool("require-tx-metadata")

			if len(pubkeysExclude) > 0 {
				slog.Info("Excluding transactions with the following public keys:", "pubkeys", pubkeysExclude)
			}

			numTransactionsWithMoreThanOnePieceForMetadata := new(atomic.Uint64)
			defer func() {
				n := numTransactionsWithMoreThanOnePieceForMetadata.Load()
				if n > 0 {
					slog.Info(
						"num transactions with more than one piece for metadata",
						"num", numTransactionsWithMoreThanOnePieceForMetadata.Load(),
					)
				}
			}()

			numWorkers := c.Uint("workers")
			if numWorkers == 0 {
				numWorkers = uint(runtime.NumCPU())
			}

			// Decoding each block group (decode block, reassemble metadata, verify
			// signatures) is CPU-bound. Blocks are read sequentially by the
			// accumulator, fanned out to a pool of workers for the heavy work, and
			// their results are applied to the index on this single consumer
			// goroutine. ordered-concurrently preserves input order, so blocks reach
			// indexW.Push in slot order, keeping the writer's slot-based flush
			// heuristic intact.
			workerCtx, cancelWorkers := context.WithCancel(context.Background())
			defer cancelWorkers()

			workerInputChan := make(chan concurrently.WorkFunction, numWorkers)
			outputChan := concurrently.Process(
				workerCtx,
				workerInputChan,
				&concurrently.Options{PoolSize: int(numWorkers), OutChannelBuffer: int(numWorkers)},
			)

			var consumeErr error
			consumerDone := make(chan struct{})
			go func() {
				defer close(consumerDone)
				fail := func(err error) {
					if consumeErr == nil {
						consumeErr = err
						cancelWorkers()
					}
				}
				for result := range outputChan {
					if consumeErr != nil {
						continue // drain remaining results to unblock workers
					}
					switch res := result.Value.(type) {
					case error:
						fail(res)
						continue
					case *gsfaBlockResult:
						for ri := range res.records {
							rec := res.records[ri]
							numProcessedTransactions.Add(1)
							if err := indexW.Push(
								rec.Offset,
								rec.Length,
								rec.Slot,
								rec.AccountKeys,
								rec.HasMeta,
								rec.IsSuccess,
								rec.IsVote,
							); err != nil {
								fail(fmt.Errorf("error while pushing to gsfa index: %w", err))
								break
							}

							if time.Since(lastPrintedAt) > time.Second {
								percentDone := float64(rec.Slot-epochStart) / float64(epochEnd-epochStart) * 100
								// clear line, then print progress
								msg := fmt.Sprintf(
									"\rCreating gSFA index for epoch %d - %s | %s | %.2f%% | slot %s | tx %s",
									epoch,
									time.Now().Format("2006-01-02 15:04:05"),
									time.Since(startedAt).Truncate(time.Second),
									percentDone,
									humanize.Comma(int64(rec.Slot)),
									humanize.Comma(int64(numProcessedTransactions.Load())),
								)
								var eta time.Duration
								timePast := time.Since(startedAt).Truncate(time.Second).Round(time.Second)
								if percentDone > 0 && timePast > 0 {
									// it took timePast to get percentDone done
									remainingPercent := 100 - percentDone
									msForOnePercent := float64(timePast.Milliseconds()) / percentDone
									eta = time.Millisecond * time.Duration(msForOnePercent*remainingPercent)
									eta = eta.Truncate(time.Second).Round(time.Second)
								}
								if eta > 0 {
									msg += fmt.Sprintf(" | ETA %s", eta.Truncate(time.Second))
								}
								fmt.Print(msg)
								lastPrintedAt = time.Now()
							}
						}
					default:
						fail(fmt.Errorf("unexpected result type: %T", result.Value))
					}
				}
			}()

			objAccum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				accum.IgnoreKinds(
					// Ignore these kinds in the accumulator (only need Transactions and DataFrames):
					iplddecoders.KindEntry,
					iplddecoders.KindRewards,
				),
				func(parent *accum.ObjectWithMetadata, children accum.ObjectsWithMetadata) error {
					if workerCtx.Err() != nil {
						// The consumer hit an error and asked us to stop; release the
						// buffers we were handed since no worker will, then stop.
						if parent != nil {
							carreader.PutBuffer(parent.ObjectData)
						}
						children.Put()
						return accum.ErrStop
					}
					workerInputChan <- &gsfaBlockParser{
						parent:                parent,
						children:              children,
						sigverify:             sigverify,
						requireTxMetadata:     requireTxMetadata,
						pubkeysExclude:        pubkeysExclude,
						numMissingMetadata:    numMissingMetadata,
						numMultiPieceMetadata: numTransactionsWithMoreThanOnePieceForMetadata,
					}
					return nil
				},
			)

			runErr := objAccum.Run(context.Background())
			close(workerInputChan)
			<-consumerDone
			if consumeErr != nil {
				return fmt.Errorf("error while accumulating objects: %w", consumeErr)
			}
			if runErr != nil {
				return fmt.Errorf("error while accumulating objects: %w", runErr)
			}

			{
				klog.Infof("Indexed %s transactions", humanize.Comma(int64(numProcessedTransactions.Load())))
				klog.Info("Finalizing index -- this may take a while, DO NOT EXIT")
				klog.Info("Closing index")
				if err := indexW.Close(); err != nil {
					klog.Fatalf("Error while closing: %s", err)
				}
				klog.Infof("Success: gSFA index created at %s with %s transactions", gsfaIndexDir, humanize.Comma(int64(numProcessedTransactions.Load())))
				klog.Infof("Finished in %s", time.Since(startedAt))
			}

			return nil
		},
	}
}

// gsfaRecord is a single transaction's contribution to the gsfa index, fully
// detached from any pooled/reused memory so it can be pushed later on the
// consumer goroutine.
type gsfaRecord struct {
	Offset      uint64
	Length      uint64
	Slot        uint64
	AccountKeys solana.PublicKeySlice
	HasMeta     bool
	IsSuccess   bool
	IsVote      bool
}

// gsfaBlockResult holds the per-transaction records decoded from one block group.
type gsfaBlockResult struct {
	records []gsfaRecord
}

// gsfaBlockParser is the unit of parallel work for the gsfa indexer: it decodes
// one block group (block + its transactions + metadata), verifies signatures,
// and produces the records to push. It is a concurrently.WorkFunction.
type gsfaBlockParser struct {
	parent   *accum.ObjectWithMetadata
	children accum.ObjectsWithMetadata

	sigverify         bool
	requireTxMetadata bool
	pubkeysExclude    solana.PublicKeySlice

	numMissingMetadata    *atomic.Int64
	numMultiPieceMetadata *atomic.Uint64
}

func (w *gsfaBlockParser) Run(ctx context.Context) interface{} {
	parent := w.parent
	children := w.children
	defer func() {
		if parent != nil {
			carreader.PutBuffer(parent.ObjectData)
		}
		children.Put()
	}()

	if parent == nil {
		transactions, err := accum.ObjectsToTransactionsAndMetadata(
			&ipldbindcode.Block{
				Meta: ipldbindcode.SlotMeta{
					Blocktime: 0,
				},
			}, children)
		if err != nil {
			return fmt.Errorf("error while converting objects to transactions: %w", err)
		}
		if len(transactions) == 0 {
			accum.PutTransactionWithSlotSlice(transactions)
			return &gsfaBlockResult{}
		}
		spew.Dump(parent, transactions, len(children))
	}

	// decode the block:
	block, err := iplddecoders.DecodeBlock(parent.ObjectData.Bytes())
	if err != nil {
		return fmt.Errorf("error while decoding block: %w", err)
	}
	defer iplddecoders.PutBlock(block)
	transactions, err := accum.ObjectsToTransactionsAndMetadata(block, children)
	if err != nil {
		return fmt.Errorf("error while converting objects to transactions: %w", err)
	}
	defer accum.PutTransactionWithSlotSlice(transactions)

	if w.sigverify {
		for ii := range transactions {
			txWithInfo := transactions[ii]
			if err := txWithInfo.Transaction.VerifySignatures(); err != nil {
				return fmt.Errorf(
					"error while verifying signatures for transaction %s: %w",
					txWithInfo.Transaction.Signatures[0],
					err,
				)
			}
			if len(txWithInfo.MetadataPieces) > 0 {
				w.numMultiPieceMetadata.Add(1)
			}
		}
	}

	records := make([]gsfaRecord, 0, len(transactions))
	for ii := range transactions {
		txWithInfo := transactions[ii]
		// Build a fresh, independent copy of the account keys so we don't hold a
		// reference into pooled transaction memory after this worker returns
		// (append of []PublicKey copies the [32]byte values).
		srcKeys := txWithInfo.Transaction.Message.AccountKeys
		accountKeys := make(solana.PublicKeySlice, 0, len(srcKeys)+8)
		accountKeys = append(accountKeys, srcKeys...)
		if txWithInfo.Metadata != nil && txWithInfo.Metadata.IsProtobuf() {
			meta := txWithInfo.Metadata.GetProtobuf()
			accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedReadonlyAddresses)...)
			accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedWritableAddresses)...)
		}
		hasMeta := txWithInfo.Metadata != nil // We include this to know whether isSuccess is valid.
		if txWithInfo.Metadata == nil || !txWithInfo.Metadata.HasMeta() {
			w.numMissingMetadata.Add(1)
			if w.requireTxMetadata {
				klog.Errorf("Transaction %s has no metadata", txWithInfo.Transaction.Signatures[0])
				spew.Dump(txWithInfo.Error, txWithInfo.IsMetaParseError())
				panic("Transaction has no metadata, but --require-tx-metadata=true")
			}
		}
		isSuccess := func() bool {
			// check if the transaction is a success:
			if txWithInfo.Metadata == nil {
				// NOTE: if there is no metadata, we have NO WAY of knowing if the transaction was successful.
				return false
			}
			if txWithInfo.Metadata.IsProtobuf() {
				meta := txWithInfo.Metadata.GetProtobuf()
				if meta.Err == nil {
					return true
				}
			}
			if txWithInfo.Metadata.IsSerde() {
				meta := txWithInfo.Metadata.GetSerde()
				_, ok := meta.Status.(*serde_agave.Result__Ok)
				if ok {
					return true
				}
			}
			return false
		}()

		isVote := IsVote(txWithInfo.Transaction)

		// v2:
		if len(w.pubkeysExclude) > 0 {
			accountKeys = slices.DeleteFunc(
				accountKeys,
				func(pk solana.PublicKey) bool {
					return slices.Contains(w.pubkeysExclude, pk)
				},
			)
		}
		// Dedupe here, in the parallel worker, rather than in GsfaWriter.Push on
		// the single consumer goroutine (Dedupe sorts internally, and profiling
		// showed that sort dominating the serialized writer). Dedupe returns a
		// new sorted, duplicate-free slice.
		accountKeys = accountKeys.Dedupe()
		records = append(records, gsfaRecord{
			Offset:      txWithInfo.Offset,
			Length:      txWithInfo.Length,
			Slot:        txWithInfo.Slot,
			AccountKeys: accountKeys,
			HasMeta:     hasMeta,
			IsSuccess:   isSuccess,
			IsVote:      isVote,
		})
	}
	return &gsfaBlockResult{records: records}
}

// startCPUProfile begins writing a CPU profile to the given path and returns a
// stop function that stops profiling and closes the file. The stop function is
// safe to call more than once (e.g. from both a deferred call and a signal
// handler).
func startCPUProfile(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("could not create CPU profile file %q: %w", path, err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("could not start CPU profile: %w", err)
	}
	klog.Infof("Writing CPU profile to %s", path)
	var once sync.Once
	return func() {
		once.Do(func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				klog.Errorf("error closing CPU profile file: %s", err)
			}
			klog.Infof("CPU profile written to %s", path)
		})
	}, nil
}

func formatIndexDirname_gsfa(epoch uint64, rootCid cid.Cid, network indexes.Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"gsfa.indexdir",
	)
}

func isFileOrDirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File or directory does not exist
		}
		return false, err // Other error occurred
	}
	return info.IsDir() || info.Mode().IsRegular(), nil // Return true if it's a directory or a regular file
}

func isDirNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Directory does not exist
		}
		return false, err // Other error occurred
	}
	return len(entries) > 0, nil // Return true if there are any entries in the directory
}
