package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readahead"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_gsfa() *cli.Command {
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "gsfa",
		Description: "Create GSFA index from a CAR file",
		ArgsUsage:   "<car-path> <index-dir>",
		Before: func(c *cli.Context) error {
			if network == "" {
				network = indexes.NetworkMainnet
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:  "flush-every",
				Usage: "flush every N transactions",
				Value: 1_000_000,
			},
			// verify hash of transactions:
			&cli.BoolFlag{
				Name:  "verify-hash",
				Usage: "verify hash of transactions",
				Value: false,
			},
			// w number of workers:
			&cli.UintFlag{
				Name:  "w",
				Usage: "number of workers",
				Value: uint(runtime.NumCPU()) * 3,
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
		},
		Action: func(c *cli.Context) error {
			carPath := c.Args().First()
			var file fs.File
			var err error
			if carPath == "-" {
				file = os.Stdin
			} else {
				file, err = os.Open(carPath)
				if err != nil {
					klog.Exit(err.Error())
				}
				defer file.Close()
			}

			cachingReader, err := readahead.NewCachingReaderFromReader(file, readahead.DefaultChunkSize)
			if err != nil {
				klog.Exitf("Failed to create caching reader: %s", err)
			}
			rd, err := car.NewCarReader(cachingReader)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			{
				// print roots:
				roots := rd.Header.Roots
				klog.Infof("Roots: %d", len(roots))
				for i, root := range roots {
					if i == 0 && len(roots) == 1 {
						klog.Infof("- %s (Epoch CID)", root.String())
					} else {
						klog.Infof("- %s", root.String())
					}
				}
			}

			indexDir := c.Args().Get(1)
			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			rootCID := rd.Header.Roots[0]

			// Use the car file name and root CID to name the gsfa index dir:
			gsfaIndexDir := filepath.Join(indexDir, formatIndexDirname_gsfa(
				epoch,
				rootCID,
				network,
			))
			klog.Infof("Creating gsfa index dir at %s", gsfaIndexDir)
			err = os.Mkdir(gsfaIndexDir, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create index dir: %w", err)
			}

			flushEvery := c.Uint64("flush-every")
			if flushEvery == 0 {
				return fmt.Errorf("flush-every must be > 0")
			}
			klog.Infof("Will flush to index every %s transactions", humanize.Comma(int64(flushEvery)))

			meta := indexmeta.Meta{}
			if err := meta.AddUint64(indexmeta.MetadataKey_Epoch, epoch); err != nil {
				return fmt.Errorf("failed to add epoch to sig_exists index metadata: %w", err)
			}
			if err := meta.AddCid(indexmeta.MetadataKey_RootCid, rootCID); err != nil {
				return fmt.Errorf("failed to add root cid to sig_exists index metadata: %w", err)
			}
			if err := meta.AddString(indexmeta.MetadataKey_Network, string(network)); err != nil {
				return fmt.Errorf("failed to add network to sig_exists index metadata: %w", err)
			}
			accu, err := gsfa.NewGsfaWriter(
				gsfaIndexDir,
				flushEvery,
				meta,
			)
			if err != nil {
				return fmt.Errorf("error while opening gsfa index writer: %w", err)
			}
			defer func() {
				if err := accu.Flush(); err != nil {
					klog.Errorf("Error while flushing: %s", err)
				}
				if err := accu.Close(); err != nil {
					klog.Errorf("Error while closing: %s", err)
				}
			}()

			startedAt := time.Now()
			numTransactionsSeen := 0
			defer func() {
				klog.Infof("Finished in %s", time.Since(startedAt))
				klog.Infof("Indexed %s transactions", humanize.Comma(int64(numTransactionsSeen)))
			}()
			dotEvery := 100_000
			klog.Infof("A dot is printed every %s transactions", humanize.Comma(int64(dotEvery)))

			verifyHash = c.Bool("verify-hash")
			numWorkers := c.Uint("w")

			if numWorkers == 0 {
				numWorkers = uint(runtime.NumCPU())
			}
			workerInputChan := make(chan concurrently.WorkFunction, numWorkers)
			waitExecuted := new(sync.WaitGroup)
			waitResultsReceived := new(sync.WaitGroup)
			numReceivedAtomic := new(atomic.Int64)

			outputChan := concurrently.Process(
				context.Background(),
				workerInputChan,
				&concurrently.Options{PoolSize: int(numWorkers), OutChannelBuffer: int(numWorkers)},
			)
			go func() {
				// process the results from the workers
				for result := range outputChan {
					switch resValue := result.Value.(type) {
					case error:
						panic(resValue)
					case TransactionWithSlot:
						tx := resValue.Transaction
						slot := resValue.Slot
						sig := tx.Signatures[0]
						err = accu.Push(slot, sig, tx.Message.AccountKeys)
						if err != nil {
							klog.Exitf("Error while pushing to gsfa index: %s", err)
						}
						waitResultsReceived.Done()
						numReceivedAtomic.Add(-1)
					default:
						panic(fmt.Errorf("unexpected result type: %T", result.Value))
					}
				}
			}()

			for {
				block, err := rd.Next()
				if err != nil {
					if errors.Is(err, io.EOF) {
						fmt.Println("EOF")
						break
					}
					return err
				}
				kind := iplddecoders.Kind(block.RawData()[1])

				switch kind {
				case iplddecoders.KindTransaction:
					numTransactionsSeen++
					if numTransactionsSeen%dotEvery == 0 {
						fmt.Print(".")
					}
					{
						waitExecuted.Add(1)
						waitResultsReceived.Add(1)
						numReceivedAtomic.Add(1)
						workerInputChan <- newTxParserWorker(
							block,
							func() {
								waitExecuted.Done()
							},
						)
					}
				default:
					continue
				}
			}

			{
				klog.Infof("Waiting for all transactions to be parsed...")
				waitExecuted.Wait()
				klog.Infof("All transactions parsed.")

				klog.Infof("Waiting to receive all results...")
				close(workerInputChan)
				waitResultsReceived.Wait()
				klog.Infof("All results received")
			}
			klog.Infof("Success: GSFA index created at %s", gsfaIndexDir)
			return nil
		},
	}
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

type TransactionWithSlot struct {
	Slot        uint64
	Transaction solana.Transaction
}

type txParserWorker struct {
	blk  blocks.Block
	done func()
}

func newTxParserWorker(
	blk blocks.Block,
	done func(),
) *txParserWorker {
	return &txParserWorker{
		blk:  blk,
		done: done,
	}
}

var verifyHash bool

func (w txParserWorker) Run(ctx context.Context) interface{} {
	defer func() {
		w.done()
	}()

	block := w.blk

	decoded, err := iplddecoders.DecodeTransaction(block.RawData())
	if err != nil {
		return fmt.Errorf("error while decoding transaction from nodex %s: %w", block.Cid(), err)
	}
	{
		if total, ok := decoded.Data.GetTotal(); !ok || total == 1 {
			completeData := decoded.Data.Bytes()
			if verifyHash {
				// verify hash (if present)
				if ha, ok := decoded.Data.GetHash(); ok {
					err := ipldbindcode.VerifyHash(completeData, ha)
					if err != nil {
						klog.Exitf("Error while verifying hash for %s: %s", block.Cid(), err)
					}
				}
			}
			var tx solana.Transaction
			if err := bin.UnmarshalBin(&tx, completeData); err != nil {
				klog.Exitf("Error while unmarshaling transaction from nodex %s: %s", block.Cid(), err)
			} else if len(tx.Signatures) == 0 {
				klog.Exitf("Error while unmarshaling transaction from nodex %s: no signatures", block.Cid())
			}
			return TransactionWithSlot{
				Slot:        uint64(decoded.Slot),
				Transaction: tx,
			}
		} else {
			klog.Warningf("Transaction data is split into multiple objects for %s; skipping", block.Cid())
		}
	}
	return nil
}
