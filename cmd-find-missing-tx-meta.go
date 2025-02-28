package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_find_missing_tx_metadata() *cli.Command {
	return &cli.Command{
		Name:        "find-missing-tx-metadata",
		Description: "Find missing transaction metadata in a CAR file.",
		ArgsUsage:   "<car-paths>",
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
			// destination dir for the output files
			&cli.StringFlag{
				Name:  "output-dir",
				Usage: "Destination directory for the output files",
				Value: "",
			},
			// skip is the number of objects (any kind) to skip before starting to process.
			&cli.Uint64Flag{
				Name:  "skip",
				Usage: "Number of objects to skip before starting to process",
				Value: 0,
			},
			// dont' save missing metadata
			&cli.BoolFlag{
				Name:  "no-save-missing-metadata",
				Usage: "Do not save the signatures of transactions that are missing metadata",
				Value: false,
			},
			// dont' save metadata parsing errors
			&cli.BoolFlag{
				Name:  "no-save-meta-parsing-errors",
				Usage: "Do not save the errors that occurred while parsing the metadata",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			carPaths := c.Args().Slice()
			var file fs.File
			var err error
			if len(carPaths) == 0 {
				klog.Exit("Please provide a CAR file")
			}
			if carPaths[0] == "-" {
				file = os.Stdin
			} else {
				file, err = os.Open(carPaths[0])
				if err != nil {
					klog.Exit(err.Error())
				}
				defer file.Close()
			}

			silent := c.Bool("silent")

			if silent {
				klog.Infoln("Silent mode is ON: will not print progress")
			}

			noSaveMissingMetadata := c.Bool("no-save-missing-metadata")
			if noSaveMissingMetadata {
				klog.Infoln("Will not save missing metadata")
			}

			noSaveMetaParsingErrors := c.Bool("no-save-meta-parsing-errors")
			if noSaveMetaParsingErrors {
				klog.Infoln("Will not save metadata parsing errors")
			}

			rd, err := readasonecar.NewMultiReader(carPaths...)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			defer rd.Close()

			outputDir := c.String("output-dir")
			if outputDir == "" {
				outputDir = filepath.Dir(carPaths[0])
			} else {
				if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
					klog.Exitf("Failed to create output directory: %s", err)
				}
			}

			// In the same directory as the CAR file, create a file where we will write the signatures of the transactions that are missing metadata.
			missingTxPath := filepath.Join(outputDir, filepath.Base(carPaths[0])+".missing-tx-metadata.txt")
			fileMissingMetadata, err := tooling.NewBufferedWritableFile(missingTxPath)
			if err != nil {
				klog.Exitf("Failed to create file for missing metadata: %s", err)
			}

			// In the same directory as the CAR file, create a file where we will write the errors that occurred while parsing the metadata.
			txMetaParseErrorPath := filepath.Join(outputDir, filepath.Base(carPaths[0])+".tx-meta-parsing-error.txt")
			fileTxMetaParsingError, err := tooling.NewBufferedWritableFile(txMetaParseErrorPath)
			if err != nil {
				klog.Exitf("Failed to create file for tx meta parsing error: %s", err)
			}

			numProcessedTransactions := new(atomic.Int64)
			startedAt := time.Now()

			slotCounter := uint64(0)
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
			numTransactionsWithMetaParsingError := new(atomic.Uint64)
			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(parent *accum.ObjectWithMetadata, children []accum.ObjectWithMetadata) error {
					slotCounter++
					numObjects := len(children) + 1
					if numObjects > int(numMaxObjects) {
						numMaxObjects = uint64(numObjects)
					}

					if parent == nil {
						transactions, err := accum.ObjectsToTransactionsAndMetadata(&ipldbindcode.Block{
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
					if slotCounter%etaSampleSlots == 0 {
						tookToDo1kSlots = time.Since(lastTimeDid1kSlots)
						lastTimeDid1kSlots = time.Now()
					}
					if firstSlot == nil {
						slot := uint64(block.Slot)
						firstSlot = &slot
						// determine epoch:
						epoch := slottools.CalcEpochForSlot(slot)
						epochStart, epochEnd = slottools.CalcEpochLimits(epoch)
					}
					if tookToDo1kSlots > 0 {
						remainingSlots := epochEnd - uint64(block.Slot)
						if epochEnd < uint64(block.Slot) {
							remainingSlots = 0
						}
						eta = time.Duration(float64(tookToDo1kSlots) / float64(etaSampleSlots) * float64(remainingSlots))
					}
					transactions, err := accum.ObjectsToTransactionsAndMetadata(block, children)
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
							if txWithInfo.IsMetaNotFound() {
								numTransactionsWithMissingMetadata.Add(1)
								if !noSaveMissingMetadata {
									err := fileMissingMetadata.WriteString(txWithInfo.Transaction.Signatures[0].String() + "\n")
									if err != nil {
										return fmt.Errorf("error while writing to file: %w", err)
									}
								}
							}
							if txWithInfo.IsMetaParseError() {
								numTransactionsWithMetaParsingError.Add(1)
								quotedError := fmt.Sprintf("%q", txWithInfo.Error)
								if !noSaveMetaParsingErrors {
									err := fileTxMetaParsingError.WriteString(quotedError + "\n")
									if err != nil {
										return fmt.Errorf("error while writing to file: %w", err)
									}
								}
							}
						}

						if time.Since(lastPrintedAt) > time.Millisecond*500 {
							percentDone := float64(txWithInfo.Slot-epochStart) / float64(epochEnd-epochStart) * 100
							// clear line, then print progress
							msg := fmt.Sprintf(
								"\rChecking tx meta - %s missing, %s parse err - %s | %s | %.2f%% | slot %s | tx %s",
								humanize.Comma(int64(numTransactionsWithMissingMetadata.Load())),
								humanize.Comma(int64(numTransactionsWithMetaParsingError.Load())),
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

			if skip := c.Uint64("skip"); skip > 0 {
				klog.Infof("Skipping %s objects", humanize.Comma(int64(skip)))
				accum.SetSkip(skip)
			}

			if err := accum.Run(context.Background()); err != nil {
				return fmt.Errorf("error while accumulating objects: %w", err)
			}

			fileMissingMetadata.Close()
			fileTxMetaParsingError.Close()

			klog.Infof("Checked %s transactions", humanize.Comma(int64(numProcessedTransactions.Load())))
			klog.Infof("Finished in %s", time.Since(startedAt))

			klog.Infof("Transactions with missing metadata: %d", numTransactionsWithMissingMetadata.Load())
			klog.Infof("Transactions with metadata parsing error: %d", numTransactionsWithMetaParsingError.Load())

			// NOTE: if there are parsing errors, THEY WILL BE IGNORED.
			if numTransactionsWithMissingMetadata.Load() > 0 {
				file.Close()
				os.Exit(1)
			}
			os.Exit(0)
			return nil
		},
	}
}
