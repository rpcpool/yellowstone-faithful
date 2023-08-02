package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	sigtoepoch "github.com/rpcpool/yellowstone-faithful/sig-to-epoch"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_sigToEpoch() *cli.Command {
	return &cli.Command{
		Name:        "sig-to-epoch",
		Description: "Create or append to a sig-to-epoch index from a CAR file.",
		ArgsUsage:   "<car-path> <sig-to-epoch-index-dir>",
		Before: func(c *cli.Context) error {
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

			rd, err := car.NewCarReader(file)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			{
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
			existed := true
			if ok, err := isDirectory(indexDir); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					existed = false
					if err := os.MkdirAll(indexDir, 0o755); err != nil {
						return fmt.Errorf("failed to create index-dir: %w", err)
					} else {
						klog.Infof("Created index-dir at %s", indexDir)
					}
				} else {
					return err
				}
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			sigToEpochIndexDir := indexDir
			if !existed {
				klog.Infof("Creating NEW sig-to-epoch index at %s", sigToEpochIndexDir)
			} else {
				klog.Infof("Will APPEND to existing sig-to-epoch index at %s", sigToEpochIndexDir)
			}
			index, err := sigtoepoch.NewIndex(
				sigToEpochIndexDir,
			)
			if err != nil {
				return fmt.Errorf("error while opening sig-to-epoch index writer: %w", err)
			}
			defer func() {
				if err := index.Flush(); err != nil {
					klog.Errorf("Error while flushing: %s", err)
				}
				if err := index.Close(); err != nil {
					klog.Errorf("Error while closing: %s", err)
				}
			}()
			if existed {
				epochs, err := index.Epochs()
				if err != nil {
					return fmt.Errorf("error while getting epochs: %w", err)
				}
				klog.Infof("Index (currently) contains %d epochs", len(epochs))
				for _, epoch := range epochs {
					klog.Infof("- Epoch #%d", epoch)
				}
			}

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
				c.Context,
				workerInputChan,
				&concurrently.Options{PoolSize: int(numWorkers), OutChannelBuffer: int(numWorkers)},
			)
			go func() {
				// process the results from the workers
				for result := range outputChan {
					switch resValue := result.Value.(type) {
					case error:
						panic(resValue)
					case SignatureAndSlot:
						slot := resValue.Slot
						sig := resValue.Signature
						{
							err = index.Push(c.Context, sig, uint16(CalcEpochForSlot(slot)))
							if err != nil {
								classicSpewConfig.Dump(err)
								klog.Exitf("Error while pushing to sig-to-epoch index: %s", err)
							}
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
				if errors.Is(err, io.EOF) {
					fmt.Println("EOF")
					break
				}
				kind := iplddecoders.Kind(block.RawData()[1])

				switch kind {
				case iplddecoders.KindTransaction:
					numTransactionsSeen++
					if numTransactionsSeen%dotEvery == 0 {
						fmt.Print(".")
					}
					if numTransactionsSeen%10_000_000 == 0 {
						fmt.Println(humanize.Comma(int64(numTransactionsSeen)))
					}
					{
						waitExecuted.Add(1)
						waitResultsReceived.Add(1)
						numReceivedAtomic.Add(1)
						workerInputChan <- newSignatureSlot(
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
				klog.Infof("Waiting for all transactions to be processed...")
				waitExecuted.Wait()
				klog.Infof("All transactions processed.")

				klog.Infof("Waiting to receive all results...")
				close(workerInputChan)
				waitResultsReceived.Wait()
				klog.Infof("All results received")
			}

			if existed {
				klog.Infof("Success: sig-to-epoch index appended at %s", sigToEpochIndexDir)
				epochs, err := index.Epochs()
				if err != nil {
					return fmt.Errorf("error while getting epochs: %w", err)
				}
				klog.Infof("Index (now) contains %d epochs", len(epochs))
				for _, epoch := range epochs {
					klog.Infof("- Epoch #%d", epoch)
				}
			} else {
				klog.Infof("Success: sig-to-epoch index created at %s", sigToEpochIndexDir)
			}
			return nil
		},
	}
}

var classicSpewConfig = spew.ConfigState{
	Indent:                  " ",
	DisableMethods:          true,
	DisablePointerMethods:   true,
	DisablePointerAddresses: true,
}

type SignatureAndSlot struct {
	Slot      uint64
	Signature solana.Signature
}

type sigToEpochParser struct {
	blk  blocks.Block
	done func()
}

func newSignatureSlot(
	blk blocks.Block,
	done func(),
) *sigToEpochParser {
	return &sigToEpochParser{
		blk:  blk,
		done: done,
	}
}

func (w sigToEpochParser) Run(ctx context.Context) interface{} {
	defer func() {
		w.done()
	}()

	block := w.blk

	decoded, err := iplddecoders.DecodeTransaction(block.RawData())
	if err != nil {
		return fmt.Errorf("error while decoding transaction from nodex %s: %w", block.Cid(), err)
	}
	sig, err := readFirstSignature(decoded.Data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}
	return SignatureAndSlot{
		Slot:      uint64(decoded.Slot),
		Signature: sig,
	}
}
