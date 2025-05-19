package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_gsfa() *cli.Command {
	var epoch uint64
	var network indexes.Network
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
				Usage: "temporary directory to use for storing intermediate files; WILL BE DELETED",
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
		},
		Action: func(c *cli.Context) error {
			carPaths := c.StringSlice("car")
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

			rd, err := readasonecar.NewMultiReader(carPaths...)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			defer rd.Close()

			indexDir := c.String("index-dir")
			if indexDir == "" {
				klog.Exit("Please provide an --index-dir=<dir to store the index>")
			}
			if ok, err := isDirectory(indexDir); err != nil {
				return fmt.Errorf("error checking index-dir %q: %w", indexDir, err)
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
			tmpDir = filepath.Join(tmpDir, fmt.Sprintf("yellowstone-faithful-gsfa-%d", time.Now().UnixNano()))
			if err := os.MkdirAll(tmpDir, 0o755); err != nil {
				return fmt.Errorf("failed to create tmp dir: %w", err)
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
			defer func() {
				klog.Infof("Indexed %s transactions", humanize.Comma(int64(numProcessedTransactions.Load())))
				klog.Info("Finalizing index -- this may take a while, DO NOT EXIT")
				klog.Info("Closing index")
				if err := indexW.Close(); err != nil {
					klog.Fatalf("Error while closing: %s", err)
				}
				klog.Infof("Success: gSFA index created at %s with %d transactions", gsfaIndexDir, numProcessedTransactions.Load())
				klog.Infof("Finished in %s", time.Since(startedAt))
			}()

			verifyHash := c.Bool("verify-hash")
			ipldbindcode.DisableHashVerification = !verifyHash

			epochStart, epochEnd := slottools.CalcEpochLimits(epoch)

			numSlots := uint64(0)
			numMaxObjects := uint64(0)

			lastPrintedAt := time.Now()

			numMissingMetadata := new(atomic.Int64)
			numMissingMetadata.Store(0)

			requireTxMetadata := c.Bool("require-tx-metadata")
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
					transactions, err := accum.ObjectsToTransactionsAndMetadata(block, children)
					if err != nil {
						return fmt.Errorf("error while converting objects to transactions: %w", err)
					}
					defer accum.PutTransactionWithSlotSlice(transactions)

					for ii := range transactions {
						txWithInfo := transactions[ii]
						numProcessedTransactions.Add(1)
						accountKeys := txWithInfo.Transaction.Message.AccountKeys
						if txWithInfo.Metadata != nil && txWithInfo.Metadata.IsProtobuf() {
							meta := txWithInfo.Metadata.GetProtobuf()
							accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedReadonlyAddresses)...)
							accountKeys = append(accountKeys, byteSlicesToKeySlice(meta.LoadedWritableAddresses)...)
						}
						hasMeta := txWithInfo.Metadata != nil // We include this to know whether isSuccess is valid.
						if txWithInfo.Metadata == nil || !txWithInfo.Metadata.HasMeta() {
							numMissingMetadata.Add(1)
							if requireTxMetadata {
								klog.Errorf("Transaction %s has no metadata", txWithInfo.Transaction.Signatures[0])
								spew.Dump(txWithInfo.Error, txWithInfo.IsMetaParseError())
								spew.Dump(txWithInfo)
								panic("metadata error")
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

						isVote := IsVote(&txWithInfo.Transaction)

						err = indexW.Push(
							txWithInfo.Offset,
							txWithInfo.Length,
							txWithInfo.Slot,
							accountKeys,
							hasMeta,
							isSuccess,
							isVote,
						)
						if err != nil {
							klog.Exitf("Error while pushing to gsfa index: %s", err)
						}

						if time.Since(lastPrintedAt) > time.Second {
							percentDone := float64(txWithInfo.Slot-epochStart) / float64(epochEnd-epochStart) * 100
							// clear line, then print progress
							msg := fmt.Sprintf(
								"\rCreating gSFA index for epoch %d - %s | %s | %.2f%% | slot %s | tx %s",
								epoch,
								time.Now().Format("2006-01-02 15:04:05"),
								time.Since(startedAt).Truncate(time.Second),
								percentDone,
								humanize.Comma(int64(txWithInfo.Slot)),
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
					return nil
				},
				// Ignore these kinds in the accumulator (only need Transactions and DataFrames):
				iplddecoders.KindEntry,
				iplddecoders.KindRewards,
			)

			if err := accum.Run(context.Background()); err != nil {
				return fmt.Errorf("error while accumulating objects: %w", err)
			}

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
