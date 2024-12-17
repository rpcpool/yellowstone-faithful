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
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
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
				Usage: "temporary directory to use for storing intermediate files; WILL BE DELETED",
				Value: os.TempDir(),
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

			rd, err := carreader.New(file)
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
					klog.Errorf("Error while closing: %s", err)
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
			lastTimeDid1kSlots := time.Now()
			var eta time.Duration
			etaSampleSlots := uint64(2_000)
			var tookToDo1kSlots time.Duration
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
					if numSlots%etaSampleSlots == 0 {
						tookToDo1kSlots = time.Since(lastTimeDid1kSlots)
						lastTimeDid1kSlots = time.Now()
					}
					if tookToDo1kSlots > 0 {
						eta = time.Duration(float64(tookToDo1kSlots) / float64(etaSampleSlots) * float64(epochEnd-epochStart-numSlots))
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
						err = indexW.Push(
							txWithInfo.Offset,
							txWithInfo.Length,
							txWithInfo.Slot,
							accountKeys,
						)
						if err != nil {
							klog.Exitf("Error while pushing to gsfa index: %s", err)
						}

						if time.Since(lastPrintedAt) > time.Millisecond*500 {
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
