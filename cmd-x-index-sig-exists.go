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

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_sigExists() *cli.Command {
	var verify bool
	return &cli.Command{
		Name:        "sig-exists",
		Description: "Create sig-exists index from a CAR file",
		ArgsUsage:   "<car-path> <index-dir>",
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
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the index after creating it",
				Destination: &verify,
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
			rootCID := rd.Header.Roots[0]
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

			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			klog.Infof("Creating sig-exists index for %s", carPath)
			indexFilePath := formatSigExistsIndexFilePath(indexDir, carPath, rootCID.String())
			index, err := bucketteer.NewWriter(
				indexFilePath,
			)
			if err != nil {
				return fmt.Errorf("error while opening sig-exists index writer: %w", err)
			}
			defer func() {
				if err := index.Close(); err != nil {
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
						sig := resValue.Signature
						{
							index.Put(sig)
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

			klog.Info("Sealing index...")
			sealingStartedAt := time.Now()
			_, err = index.Seal(
				map[string]string{
					"root_cid": rootCID.String(),
				},
			)
			if err != nil {
				return fmt.Errorf("error while sealing index: %w", err)
			}
			klog.Infof("Sealed index in %s", time.Since(sealingStartedAt))

			klog.Infof("Success: sig-exists index created at %s", indexFilePath)

			if verify {
				klog.Infof("Verifying index for %s located at %s", carPath, indexFilePath)
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				err := VerifyIndex_sigExists(context.TODO(), carPath, indexFilePath)
				if err != nil {
					return cli.Exit(err, 1)
				}
				klog.Info("Index verified")
				return nil
			}
			return nil
		},
	}
}

func formatSigExistsIndexFilePath(indexDir string, carPath string, rootCID string) string {
	return filepath.Join(indexDir, fmt.Sprintf("%s.%s.sig-exists.index", filepath.Base(carPath), rootCID))
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
