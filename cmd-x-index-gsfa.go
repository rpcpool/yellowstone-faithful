package main

import (
	"bytes"
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
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	carv1 "github.com/ipld/go-car"
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
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: "",
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
			rd, err := newCarReader(cachingReader)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			{
				// print roots:
				roots := rd.header.Roots
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

			rootCID := rd.header.Roots[0]

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
			tmpDir := c.String("tmp-dir")
			accu, err := gsfa.NewGsfaWriter(
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

			verifyHash := c.Bool("verify-hash")
			ipldbindcode.DisableHashVerification = !verifyHash
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
			epochStart, epochEnd := CalcEpochLimits(epoch)
			go func() {
				// process the results from the workers
				lastSeenSlot := uint64(0)
				numTransactionsProcessed := 0
				lastPrintedAt := time.Now()
				numSlotsDone := uint64(0)
				lastTimeDid1kSlots := time.Now()
				for result := range outputChan {
					switch resValue := result.Value.(type) {
					case error:
						panic(resValue)
					case TransactionWithSlot:
						numTransactionsProcessed++
						tx := resValue.Transaction
						slot := resValue.Slot
						off := resValue.Offset
						length := resValue.Length
						err = accu.Push(
							off,
							length,
							slot,
							tx.Message.AccountKeys,
						)
						if err != nil {
							klog.Exitf("Error while pushing to gsfa index: %s", err)
						}
						waitResultsReceived.Done()
						numReceivedAtomic.Add(-1)
						var eta time.Duration
						if slot != lastSeenSlot {
							lastSeenSlot = slot
							numSlotsDone++
							if numSlotsDone%1000 == 0 {
								tookToDo1kSlots := time.Since(lastTimeDid1kSlots)
								lastTimeDid1kSlots = time.Now()
								eta = time.Duration(float64(tookToDo1kSlots) / 1000 * float64(epochEnd-epochStart-numSlotsDone))
							}
						}
						if time.Since(lastPrintedAt) > time.Millisecond*500 {
							percentDone := float64(slot-epochStart) / float64(epochEnd-epochStart) * 100
							// clear line, then print progress
							msg := fmt.Sprintf(
								"\rCreating gSFA index for epoch %d - %s | %s | %.2f%% | slot %d | tx %s",
								epoch,
								time.Now().Format("2006-01-02 15:04:05"),
								time.Since(startedAt).Truncate(time.Second),
								percentDone,
								slot,
								humanize.Comma(int64(numTransactionsProcessed)),
							)
							if eta > 0 {
								msg += fmt.Sprintf(" | ETA %s", eta.Truncate(time.Second))
							}
							fmt.Print(msg)
							lastPrintedAt = time.Now()
						}
					default:
						panic(fmt.Errorf("unexpected result type: %T", result.Value))
					}
				}
			}()

			totalOffset := uint64(0)
			{
				var buf bytes.Buffer
				if err = carv1.WriteHeader(rd.header, &buf); err != nil {
					return err
				}
				totalOffset = uint64(buf.Len())
			}
			for {
				_, sectionLength, block, err := rd.NextNode()
				if err != nil {
					if errors.Is(err, io.EOF) {
						fmt.Println("EOF")
						break
					}
					return err
				}
				kind := iplddecoders.Kind(block.RawData()[1])

				currentOffset := totalOffset
				totalOffset += sectionLength

				switch kind {
				case iplddecoders.KindTransaction:
					numTransactionsSeen++
					{
						waitExecuted.Add(1)
						waitResultsReceived.Add(1)
						numReceivedAtomic.Add(1)
						workerInputChan <- newTxParserWorker(
							currentOffset,
							sectionLength,
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
			klog.Infof("Success: gSFA index created at %s", gsfaIndexDir)
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
	Offset      uint64
	Length      uint64
	Slot        uint64
	Transaction solana.Transaction
}

type txParserWorker struct {
	offset uint64
	length uint64 // section length
	object blocks.Block
	done   func()
}

func newTxParserWorker(
	offset uint64,
	length uint64,
	object blocks.Block,
	done func(),
) *txParserWorker {
	return &txParserWorker{
		offset: offset,
		length: length,
		object: object,
		done:   done,
	}
}

func (w txParserWorker) Run(ctx context.Context) interface{} {
	defer func() {
		w.done()
	}()

	object := w.object

	decoded, err := iplddecoders.DecodeTransaction(object.RawData())
	if err != nil {
		return fmt.Errorf("error while decoding transaction from nodex %s: %w", object.Cid(), err)
	}
	tx, err := decoded.GetSolanaTransaction()
	if err != nil {
		return fmt.Errorf("error while getting solana transaction from object %s: %w", object.Cid(), err)
	}

	return TransactionWithSlot{
		Offset:      w.offset,
		Length:      w.length,
		Slot:        uint64(decoded.Slot),
		Transaction: *tx,
	}
}
