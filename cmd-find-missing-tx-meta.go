package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_find_missing_tx_metadata() *cli.Command {
	return &cli.Command{
		Name:        "find-missing-tx-metadata",
		Description: "Find missing transaction metadata in a CAR file.",
		ArgsUsage:   "<car-path>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "silent",
				Usage: "Do not print progress",
				Value: false,
			},
			&cli.StringSliceFlag{
				Name:  "watch",
				Usage: "Watch for these transactions; provide a base58-encoded signature; can be repeated; will print the transaction if found",
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

			silent := c.Bool("silent")

			if silent {
				klog.Infoln("Silent mode is ON: will not print progress")
			}

			rd, err := carreader.New(file)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}

			// In the same directory as the CAR file, create a file where we will write the signatures of the transactions that are missing metadata.
			fileMissingMetadata, err := os.Create(carPath + ".missing-tx-metadata.txt")
			if err != nil {
				klog.Exitf("Failed to create file for missing metadata: %s", err)
			}

			numProcessedTransactions := new(atomic.Int64)
			startedAt := time.Now()

			numSlots := uint64(0)
			numMaxObjects := uint64(0)

			lastPrintedAt := time.Now()
			lastTimeDid1kSlots := time.Now()
			var eta time.Duration
			etaSampleSlots := uint64(2_000)
			var tookToDo1kSlots time.Duration

			var firstSlot *uint64
			var epochStart, epochEnd uint64

			watch := []solana.Signature{}
			for _, sigStr := range c.StringSlice("watch") {
				sig, err := solana.SignatureFromBase58(sigStr)
				if err != nil {
					klog.Exitf("Invalid signature: %s", sigStr)
				}
				watch = append(watch, sig)
			}
			if len(watch) > 0 {
				klog.Infof("Watching for %d transactions", len(watch))
			}
			numTransactionsWithMissingMetadata := new(atomic.Uint64)
			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(parent *accum.ObjectWithMetadata, children []accum.ObjectWithMetadata) error {
					numSlots++
					numObjects := len(children) + 1
					if numObjects > int(numMaxObjects) {
						numMaxObjects = uint64(numObjects)
					}

					if parent == nil {
						transactions, err := objectsToTransactionsAndMetadata(&ipldbindcode.Block{
							Meta: ipldbindcode.SlotMeta{
								Blocktime: 0,
							},
						}, children)
						if err != nil {
							return fmt.Errorf("error while converting objects to transactions: %w", err)
						}
						if len(transactions) == 0 {
							return nil
						}
						spew.Dump(parent, transactions, len(children))
					}

					// decode the block:
					block, err := iplddecoders.DecodeBlock(parent.ObjectData)
					if err != nil {
						return fmt.Errorf("error while decoding block: %w", err)
					}
					if numSlots%etaSampleSlots == 0 {
						tookToDo1kSlots = time.Since(lastTimeDid1kSlots)
						lastTimeDid1kSlots = time.Now()
					}
					if firstSlot == nil {
						slot := uint64(block.Slot)
						firstSlot = &slot
						// determine epoch:
						epoch := CalcEpochForSlot(slot)
						epochStart, epochEnd = CalcEpochLimits(epoch)
					}
					if tookToDo1kSlots > 0 {
						eta = time.Duration(float64(tookToDo1kSlots) / float64(etaSampleSlots) * float64(epochEnd-epochStart-numSlots))
					}
					transactions, err := objectsToTransactionsAndMetadata(block, children)
					if err != nil {
						return fmt.Errorf("error while converting objects to transactions: %w", err)
					}
					for ii := range transactions {
						txWithInfo := transactions[ii]
						numProcessedTransactions.Add(1)

						if len(watch) > 0 {
							for _, watchSig := range watch {
								if watchSig == txWithInfo.Transaction.Signatures[0] {
									spew.Dump(txWithInfo)
								}
							}
						}

						if txWithInfo.Metadata == nil {
							numTransactionsWithMissingMetadata.Add(1)
							_, err := fileMissingMetadata.WriteString(txWithInfo.Transaction.Signatures[0].String() + "\n")
							if err != nil {
								return fmt.Errorf("error while writing to file: %w", err)
							}
						}

						if time.Since(lastPrintedAt) > time.Millisecond*500 {
							percentDone := float64(txWithInfo.Slot-epochStart) / float64(epochEnd-epochStart) * 100
							// clear line, then print progress
							msg := fmt.Sprintf(
								"\rChecking missing tx meta - %s missing - %s | %s | %.2f%% | slot %s | tx %s",
								humanize.Comma(int64(numTransactionsWithMissingMetadata.Load())),
								time.Now().Format("2006-01-02 15:04:05"),
								time.Since(startedAt).Truncate(time.Second),
								percentDone,
								humanize.Comma(int64(txWithInfo.Slot)),
								humanize.Comma(int64(numProcessedTransactions.Load())),
							)
							if eta > 0 {
								msg += fmt.Sprintf(" | ETA %s", eta.Truncate(time.Second))
							}
							if !silent {
								fmt.Print(msg)
							}
							lastPrintedAt = time.Now()
						}
					}
					return nil
				},
				// Ignore these kinds in the accumulator:
				iplddecoders.KindEntry,
				iplddecoders.KindRewards,
			)

			if err := accum.Run(context.Background()); err != nil {
				return fmt.Errorf("error while accumulating objects: %w", err)
			}

			fileMissingMetadata.Close()

			klog.Infof("Checked %s transactions", humanize.Comma(int64(numProcessedTransactions.Load())))
			klog.Infof("Finished in %s", time.Since(startedAt))

			klog.Infof("Transactions with missing metadata: %d", numTransactionsWithMissingMetadata.Load())
			if numTransactionsWithMissingMetadata.Load() > 0 {
				file.Close()
				os.Exit(1)
			}
			os.Exit(0)
			return nil
		},
	}
}
