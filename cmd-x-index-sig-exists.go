package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"github.com/valyala/bytebufferpool"
	"k8s.io/klog/v2"
)

func newCmd_Index_sigExists() *cli.Command {
	var verify bool
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "sig-exists",
		Description: "Create sig-exists index from a CAR file",
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
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the index after creating it",
				Destination: &verify,
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
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.StringFlag{
				Name:  "index-dir",
				Usage: "Directory to store the index",
			},
		},
		Action: func(c *cli.Context) error {
			carPaths := c.StringSlice("car")
			if len(carPaths) == 0 {
				klog.Exit("Please provide a CAR file")
			}

			rd, err := readasonecar.NewFromFilepaths(carPaths...)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			defer rd.Close()

			rootCID, err := rd.FindRoot()
			if err != nil {
				return fmt.Errorf("failed to find root CID: %w", err)
			}
			indexDir := c.String("index-dir")

			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			klog.Infof("Creating sig-exists index for %v", carPaths)
			indexFilePath := formatSigExistsIndexFilePath(indexDir, epoch, rootCID, network)
			index, err := bucketteer.NewWriter(
				indexFilePath,
			)
			if err != nil {
				return fmt.Errorf("error while opening sig-exists index writer: %w", err)
			}

			startedAt := time.Now()
			numTransactionsSeen := 0
			defer func() {
				klog.Infof("Finished in %s", time.Since(startedAt))
				klog.Infof("Indexed %s transactions", humanize.Comma(int64(numTransactionsSeen)))
			}()
			dotEvery := 100_000
			klog.Infof("A dot is printed every %s transactions", humanize.Comma(int64(dotEvery)))

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
				_cid, _, buf, err := rd.NextNodeBytes()
				if err != nil {
					if errors.Is(err, io.EOF) {
						fmt.Println("EOF")
						break
					}
					return err
				}
				rawData := buf.Bytes()
				kind := iplddecoders.Kind(rawData[1])

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
							_cid,
							buf,
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
			_, err = index.SealAndClose(meta)
			if err != nil {
				return fmt.Errorf("error while sealing index: %w", err)
			}
			klog.Infof("Sealed index in %s", time.Since(sealingStartedAt))

			klog.Infof("Success: sig-exists index created at %s", indexFilePath)

			if verify {
				klog.Infof("Verifying index for %v located at %v", carPaths, indexFilePath)
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				err := VerifyIndex_sigExists(context.TODO(), carPaths, indexFilePath)
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

func formatSigExistsIndexFilePath(indexDir string, epoch uint64, rootCID cid.Cid, network indexes.Network) string {
	return filepath.Join(
		indexDir,
		formatFilename_SigExists(epoch, rootCID, network),
	)
}

func formatFilename_SigExists(epoch uint64, rootCid cid.Cid, network indexes.Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"sig-exists.index",
	)
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
	_cid cid.Cid // The CID of the block (not used in this parser, but can be useful for debugging)
	done func()
	blk  *bytebufferpool.ByteBuffer
}

func newSignatureSlot(
	_cid cid.Cid,
	blk *bytebufferpool.ByteBuffer,
	done func(),
) *sigToEpochParser {
	return &sigToEpochParser{
		_cid: _cid,
		blk:  blk,
		done: done,
	}
}

func (w sigToEpochParser) Run(ctx context.Context) interface{} {
	defer func() {
		w.done()
	}()

	block := w.blk

	raw := block.Bytes()
	decoded, err := iplddecoders.DecodeTransaction(raw)
	if err != nil {
		return fmt.Errorf("error while decoding transaction from nodex %s: %w", w._cid, err)
	}
	sig, err := tooling.ReadFirstSignature(decoded.Data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}
	return SignatureAndSlot{
		Slot:      uint64(decoded.Slot),
		Signature: sig,
	}
}
