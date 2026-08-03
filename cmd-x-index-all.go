package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/blocktimeindex"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/preindex"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	concurrently "github.com/tejzpr/ordered-concurrently/v3"
	"github.com/urfave/cli/v2"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

func newCmd_Index_all() *cli.Command {
	var verify bool
	var network indexes.Network
	var doPreIndexTxDedup bool
	return &cli.Command{
		Name:        "all",
		Usage:       "Create all the necessary indexes for a Solana epoch.",
		Description: "Given a CAR file containing a Solana epoch, create all the necessary indexes and save them in the specified index dir.",
		ArgsUsage:   "<index-dir>",
		Before: func(c *cli.Context) error {
			if network == "" {
				network = indexes.NetworkMainnet
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the indexes after creating them",
				Destination: &verify,
			},
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: os.TempDir(),
			},
			&cli.StringFlag{
				Name:  "network",
				Usage: "the cluster of the epoch; one of: mainnet, testnet, devnet",
				Action: func(c *cli.Context, s string) error {
					network = indexes.Network(s)
					if !indexes.IsValidNetwork(network) {
						return fmt.Errorf("invalid network: %q", network)
					}
					return nil
				},
			},
			&cli.StringSliceFlag{
				Name:  "car",
				Usage: "Path to a CAR file containing a single Solana epoch, or multiple split CAR files (in order) containing a single Solana epoch",
			},
			&cli.Uint64Flag{
				Name:     "epoch",
				Usage:    "the epoch number",
				Required: true,
			},
			&cli.BoolFlag{
				Name:        "dedup-txs",
				Usage:       "do a preliminary sig index to weed out duplicate signatures before creating the final indexes; NOTE: this requires extra disk space and RAM (40-70 GB RAM extra), and time.",
				Destination: &doPreIndexTxDedup,
			},
			&cli.UintFlag{
				Name:    "workers",
				Aliases: []string{"w"},
				Usage:   "number of workers used to decode CAR nodes in parallel",
				Value:   uint(runtime.NumCPU()) * 3,
			},
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			indexDir := c.Args().Get(0)
			tmpDir := c.String("tmp-dir")

			carPaths := c.StringSlice("car")
			if len(carPaths) == 0 {
				return fmt.Errorf("missing --car flag")
			}
			if indexDir == "" {
				return fmt.Errorf("missing index-dir argument")
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

			epoch := c.Uint64("epoch")

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Took %s", time.Since(startedAt))
				}()
				klog.Infof("Creating all indexes for %v", carPaths)
				klog.Infof("Indexes will be saved in %s", indexDir)

				var dedupReader *preindex.PreIndexReader
				if doPreIndexTxDedup {
					klog.Info("Doing preliminary sig deduplication pre-index...")
					preindexDir := filepath.Join(tmpDir, "preindex-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
					klog.Info("Creating preindex in ", preindexDir)
					pre, err := preindex.NewPreIndexWriter(preindexDir, 256, preindex.WithTotalBufferCap(GiB*20))
					if err != nil {
						return fmt.Errorf("failed to create preindex writer: %w", err)
					}
					rd, err := readasonecar.NewFromFilepaths(carPaths...)
					if err != nil {
						return fmt.Errorf("failed to create car reader: %w", err)
					}
					defer rd.Close()

					rootCID, err := rd.FindRoot()
					if err != nil {
						return fmt.Errorf("failed to find root CID: %w", err)
					}
					klog.Infof("Root CID: %s", rootCID)
					totalSize := rd.TotalSize()
					numIndexedBlocks := uint64(0)
					var eta time.Duration
					numIndexedOffsets := uint64(0)
					numIndexedTxs := uint64(0)
					for {
						totalOffset, ok := rd.GetGlobalOffsetForNextRead()
						if !ok {
							break
						}

						_, _, buf, err := rd.NextNodeBytes()
						if err != nil {
							if errors.Is(err, io.EOF) {
								break
							}
							return err
						}
						numIndexedOffsets++

						rawData := buf.Bytes()
						kind := iplddecoders.Kind(rawData[1])
						switch kind {
						case iplddecoders.KindBlock:
							{
								numIndexedBlocks++
							}
						case iplddecoders.KindTransaction:
							{
								numIndexedTxs++
								txNode, err := iplddecoders.DecodeTransaction(rawData)
								if err != nil {
									return fmt.Errorf("failed to decode transaction: %w", err)
								}

								sig, err := tooling.ReadFirstSignature(txNode.Data.Bytes())
								if err != nil {
									return fmt.Errorf("failed to read signature: %w", err)
								}
								err = pre.Push(preindex.Key(sig), preindex.Value(txNode.Slot))
								if err != nil {
									return fmt.Errorf("failed to push to preindex: %w", err)
								}
							}
						}
						percentDone := calcPercentDone(
							totalSize,
							totalOffset,
						)
						if percentDone > 0 {
							tookSoFar := time.Since(startedAt)
							msPerOnePercent := float64(tookSoFar.Milliseconds()) / (percentDone)
							eta = time.Duration(int64(msPerOnePercent)*int64(100-percentDone)) * time.Millisecond
						}
						if numIndexedOffsets%100_000 == 0 {
							var etaString string
							if eta > 0 {
								etaString = fmt.Sprintf(" ETA: %s   ", eta.Truncate(time.Second).String())
							} else {
								etaString = ", ETA: ---   "
							}
							printToStderr(
								fmt.Sprintf("\rTx-deduplication: %s txs [%s%%] %s",
									humanize.Comma(int64(numIndexedTxs)),
									humanize.CommafWithDigits(float64(percentDone), 2),
									etaString,
								),
							)
						}
					}
					printToStderr(
						fmt.Sprintf("\rPre-indexed %s txs in %s                           \n",
							humanize.Comma(int64(numIndexedTxs)),
							time.Since(startedAt).Truncate(time.Second),
						),
					)
					printToStderr("\n")
					klog.Infof(
						"Pre-indexed %s offsets, %s blocks, %s transactions in %s",
						humanize.Comma(int64(numIndexedOffsets)),
						humanize.Comma(int64(numIndexedBlocks)),
						humanize.Comma(int64(numIndexedTxs)),
						time.Since(startedAt).Truncate(time.Second),
					)
					if err := pre.Build(); err != nil {
						return fmt.Errorf("failed to seal preindex: %w", err)
					}
					klog.Info("Pre-indexing complete.")
					dedupReader, err = preindex.NewPreIndexReader(preindexDir, 256)
					if err != nil {
						return fmt.Errorf("failed to create preindex reader: %w", err)
					}
					klog.Info("Created dedup reader from preindex.")
					err = dedupReader.Load()
					if err != nil {
						return fmt.Errorf("failed to load dedup reader: %w", err)
					}
					klog.Info("Loaded dedup reader from preindex.")
				}

				numWorkers := c.Uint("workers")
				if numWorkers == 0 {
					numWorkers = uint(runtime.NumCPU())
				}
				indexPaths, numTotalItems, err := createAllIndexes(
					c.Context,
					network,
					epoch,
					tmpDir,
					carPaths,
					indexDir,
					dedupReader,
					numWorkers,
				)
				if err != nil {
					return err
				}
				klog.Info("Indexes created:")
				fmt.Println(indexPaths.String())
				if verify {
					return verifyAllIndexes(
						context.Background(),
						carPaths,
						indexPaths,
						numTotalItems,
					)
				}
				klog.Info("Skipping verification.")
			}
			return nil
		},
	}
}

// indexAllNodeItem is the decoded result of a single CAR node, carrying
// everything the consumer needs to write to the indexes without touching the
// (pooled) raw bytes again.
type indexAllNodeItem struct {
	cid           cid.Cid
	offset        uint64
	sectionLength uint64
	kind          iplddecoders.Kind
	// block-only fields:
	slot      uint64
	blocktime int64
	// transaction-only fields (slot is also set for transactions):
	sig solana.Signature
}

// indexAllNodeParser is the unit of parallel work: it decodes one CAR node's
// bytes into an *indexAllNodeItem. It is a concurrently.WorkFunction.
type indexAllNodeParser struct {
	cid           cid.Cid
	offset        uint64
	sectionLength uint64
	buf           *bytebufferpool.ByteBuffer
}

func (w *indexAllNodeParser) Run(ctx context.Context) interface{} {
	// Return the pooled buffer once we're done decoding it.
	defer carreader.PutBuffer(w.buf)
	rawData := w.buf.Bytes()
	item := &indexAllNodeItem{
		cid:           w.cid,
		offset:        w.offset,
		sectionLength: w.sectionLength,
		kind:          iplddecoders.Kind(rawData[1]),
	}
	switch item.kind {
	case iplddecoders.KindBlock:
		block, err := iplddecoders.DecodeBlock(rawData)
		if err != nil {
			return fmt.Errorf("failed to decode block: %w", err)
		}
		item.slot = uint64(block.Slot)
		item.blocktime = int64(block.Meta.Blocktime)
	case iplddecoders.KindTransaction:
		txNode, err := iplddecoders.DecodeTransaction(rawData)
		if err != nil {
			return fmt.Errorf("failed to decode transaction: %w", err)
		}
		sig, err := tooling.ReadFirstSignature(txNode.Data.Bytes())
		if err != nil {
			return fmt.Errorf("failed to read signature: %w", err)
		}
		item.sig = sig
		item.slot = uint64(txNode.Slot)
	}
	return item
}

func createAllIndexes(
	ctx context.Context,
	network indexes.Network,
	epoch uint64,
	tmpDir string,
	carPaths []string,
	indexDir string,
	dedupReader *preindex.PreIndexReader,
	numWorkers uint,
) (*IndexPaths, uint64, error) {
	err := allFilesExist(carPaths...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check if CAR file exists: %w", err)
	}

	rd, err := readasonecar.NewFromFilepaths(carPaths...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create car reader: %w", err)
	}
	defer rd.Close()

	rootCID, err := rd.FindRoot()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find root CID: %w", err)
	}
	klog.Infof("Root CID: %s", rootCID)

	klog.Infof("This CAR file is for epoch %d and cluster %s", epoch, network)

	hardcodedNumTotalItems := uint64(3_000_000_000)
	cid_to_offset_and_size, err := NewBuilder_CidToOffset(
		epoch,
		rootCID,
		network,
		tmpDir,
		hardcodedNumTotalItems,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create cid_to_offset_and_size index: %w", err)
	}

	slot_to_cid, err := NewBuilder_SlotToCid(
		epoch,
		rootCID,
		network,
		tmpDir,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create slot_to_cid index: %w", err)
	}

	hardcodedNumTransactions := uint64(2_000_000_000) // THis is used to determine the number of buckets in the index
	sig_to_cid, err := NewBuilder_SignatureToCid(
		epoch,
		rootCID,
		network,
		tmpDir,
		hardcodedNumTransactions,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create sig_to_cid index: %w", err)
	}

	sigExistsFilepath := formatSigExistsIndexFilePath(indexDir, epoch, rootCID, network)
	sig_exists, err := bucketteer.NewWriter(
		sigExistsFilepath,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create sig_exists index: %w", err)
	}

	slot_to_blocktime := blocktimeindex.NewForEpoch(epoch)

	numIndexedOffsets := uint64(0)
	numIndexedBlocks := uint64(0)
	numIndexedTransactions := uint64(0)
	klog.Infof("Indexing with %d workers...", numWorkers)
	var eta time.Duration
	startedAt := time.Now()
	totalSize := rd.TotalSize()

	// The CAR file is read sequentially (CARv1 is a length-prefixed stream and
	// cannot be seeked), but decoding each node is CPU-bound. We read the raw
	// node bytes on a single producer goroutine and fan the decode work out to a
	// pool of workers, then apply the results on this (single) consumer goroutine.
	// ordered-concurrently guarantees results arrive in the same order as the
	// input, which keeps the index output deterministic and preserves the
	// order-sensitive dedup check below.
	producerCtx, cancelProducer := context.WithCancel(ctx)
	defer cancelProducer()

	workerInputChan := make(chan concurrently.WorkFunction, numWorkers)
	outputChan := concurrently.Process(
		producerCtx,
		workerInputChan,
		&concurrently.Options{PoolSize: int(numWorkers), OutChannelBuffer: int(numWorkers)},
	)

	var produceErr error
	go func() {
		defer close(workerInputChan)
		for {
			if producerCtx.Err() != nil {
				return
			}
			totalOffset, ok := rd.GetGlobalOffsetForNextRead()
			if !ok {
				return
			}
			_cid, sectionLength, buf, err := rd.NextNodeBytes()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				produceErr = err
				cancelProducer()
				return
			}
			workerInputChan <- &indexAllNodeParser{
				cid:           _cid,
				offset:        totalOffset,
				sectionLength: sectionLength,
				buf:           buf,
			}
		}
	}()

	var consumeErr error
	fail := func(err error) {
		if consumeErr == nil {
			consumeErr = err
			cancelProducer()
		}
	}
	for result := range outputChan {
		// On error we keep ranging (discarding results) so the workers never
		// block on a full output channel and the pipeline can shut down cleanly.
		if consumeErr != nil {
			continue
		}
		switch item := result.Value.(type) {
		case error:
			fail(item)
			continue
		case *indexAllNodeItem:
			if err := cid_to_offset_and_size.Put(item.cid, item.offset, item.sectionLength); err != nil {
				fail(fmt.Errorf("failed to index cid to offset: %w", err))
				continue
			}
			numIndexedOffsets++

			switch item.kind {
			case iplddecoders.KindBlock:
				if err := slot_to_cid.Put(item.slot, item.cid); err != nil {
					fail(fmt.Errorf("failed to index slot to cid: %w", err))
					continue
				}
				if err := slot_to_blocktime.Set(item.slot, item.blocktime); err != nil {
					fail(fmt.Errorf("failed to index slot to blocktime: %w", err))
					continue
				}
				numIndexedBlocks++
			case iplddecoders.KindTransaction:
				if dedupReader != nil {
					last, err := dedupReader.IsLastMustFind(preindex.Key(item.sig), preindex.Value(item.slot))
					if err != nil {
						fail(fmt.Errorf("failed to check dedup preindex: %w", err))
						continue
					}
					if !last {
						klog.InfoS(
							"Skipping duplicate signature",
							"signature", item.sig.String(),
							"offset", item.offset,
							"slot", item.slot,
						)
						continue // skip duplicate signature
					}
				}

				if err := sig_to_cid.Put(item.sig, item.cid); err != nil {
					fail(fmt.Errorf("failed to index signature to cid: %w", err))
					continue
				}

				sig_exists.Put(item.sig)

				numIndexedTransactions++
			}

			percentDone := calcPercentDone(
				totalSize,
				item.offset,
			)
			if percentDone > 0 {
				tookSoFar := time.Since(startedAt)
				msPerOnePercent := float64(tookSoFar.Milliseconds()) / (percentDone)
				eta = time.Duration(int64(msPerOnePercent)*int64(100-percentDone)) * time.Millisecond
			}
			if numIndexedOffsets%100_000 == 0 {
				var etaString string
				if eta > 0 {
					etaString = fmt.Sprintf(" ETA: %s   ", eta.Truncate(time.Second).String())
				} else {
					etaString = ", ETA: ---   "
				}
				printToStderr(
					fmt.Sprintf("\rIndexing: %s items [%s%%] %s",
						humanize.Comma(int64(numIndexedOffsets)),
						humanize.CommafWithDigits(float64(percentDone), 2),
						etaString,
					),
				)
			}
		default:
			fail(fmt.Errorf("unexpected result type: %T", result.Value))
		}
	}
	if consumeErr != nil {
		return nil, 0, consumeErr
	}
	if produceErr != nil {
		return nil, 0, produceErr
	}
	printToStderr(
		fmt.Sprintf("\rIndexed %s items in %s                           \n",
			humanize.Comma(int64(numIndexedOffsets)),
			time.Since(startedAt).Truncate(time.Second),
		),
	)
	printToStderr("\n")
	klog.Infof(
		"Indexed %s offsets, %s blocks, %s transactions in %s",
		humanize.Comma(int64(numIndexedOffsets)),
		humanize.Comma(int64(numIndexedBlocks)),
		humanize.Comma(int64(numIndexedTransactions)),
		time.Since(startedAt).Truncate(time.Second),
	)
	if dedupReader != nil {
		if err := dedupReader.Close(); err != nil {
			return nil, 0, fmt.Errorf("failed to close dedup reader: %w", err)
		}
	}

	klog.Infof("Preparing to seal indexes (DO NOT EXIT)...")

	paths := &IndexPaths{}
	paths.SignatureExists = sigExistsFilepath

	{
		wg := new(errgroup.Group)

		// seal the indexes
		wg.Go(func() error {
			klog.Infof("Sealing cid_to_offset_and_size index...")
			if err := cid_to_offset_and_size.SealAndClose(ctx, indexDir); err != nil {
				return fmt.Errorf("failed to seal cid_to_offset_and_size index: %w", err)
			}
			paths.CidToOffsetAndSize = cid_to_offset_and_size.GetFilepath()
			klog.Infof("Successfully sealed cid_to_offset_and_size index: %s", paths.CidToOffsetAndSize)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing slot_to_cid index...")
			if err := slot_to_cid.SealAndClose(ctx, indexDir); err != nil {
				return fmt.Errorf("failed to seal slot_to_cid index: %w", err)
			}
			paths.SlotToCid = slot_to_cid.GetFilepath()
			klog.Infof("Successfully sealed slot_to_cid index: %s", paths.SlotToCid)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing sig_to_cid index...")
			if err := sig_to_cid.SealAndClose(ctx, indexDir); err != nil {
				return fmt.Errorf("failed to seal sig_to_cid index: %w", err)
			}
			paths.SignatureToCid = sig_to_cid.GetFilepath()
			klog.Infof("Successfully sealed sig_to_cid index: %s", paths.SignatureToCid)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing sig_exists index...")
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
			if _, err := sig_exists.SealAndClose(meta); err != nil {
				return fmt.Errorf("failed to seal sig_exists index: %w", err)
			}
			klog.Infof("Successfully sealed sig_exists index: %s", paths.SignatureExists)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing slot_to_blocktime index...")

			path := filepath.Join(indexDir, blocktimeindex.FormatFilename(epoch, rootCID, network))
			file, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("failed to create slot_to_blocktime index file: %w", err)
			}
			defer file.Close()

			if _, err := slot_to_blocktime.WriteTo(file); err != nil {
				return fmt.Errorf("failed to write slot_to_blocktime index: %w", err)
			}
			paths.SlotToBlocktime = path
			klog.Infof("Successfully sealed slot_to_blocktime index: %s", paths.SlotToBlocktime)
			return nil
		})

		if err := wg.Wait(); err != nil {
			return nil, 0, err
		}
	}

	return paths, hardcodedNumTotalItems, nil
}

func calcPercentDone(
	total uint64,
	done uint64,
) float64 {
	if total == 0 {
		return 0
	}
	if done == 0 {
		return 0
	}
	return float64(done) / float64(total) * 100
}

func greenBackground(s string) string {
	return blackText(fmt.Sprintf("\x1b[42m%s\x1b[0m", s))
}

func blackText(s string) string {
	return fmt.Sprintf("\x1b[30m%s\x1b[0m", s)
}

type IndexPaths struct {
	CidToOffsetAndSize string
	SlotToCid          string
	SignatureToCid     string
	SignatureExists    string
	SlotToBlocktime    string
}

// IndexPaths.String
func (p *IndexPaths) String() string {
	var builder bytes.Buffer
	builder.WriteString("  cid_to_offset_and_size:\n    uri: ")
	builder.WriteString(quoteSingle(p.CidToOffsetAndSize))
	builder.WriteString("\n")
	builder.WriteString("  slot_to_cid:\n    uri: ")
	builder.WriteString(quoteSingle(p.SlotToCid))
	builder.WriteString("\n")
	builder.WriteString("  sig_to_cid:\n    uri: ")
	builder.WriteString(quoteSingle(p.SignatureToCid))
	builder.WriteString("\n")
	builder.WriteString("  sig_exists:\n    uri: ")
	builder.WriteString(quoteSingle(p.SignatureExists))
	builder.WriteString("\n")
	builder.WriteString("  slot_to_blocktime:\n    uri: ")
	builder.WriteString(quoteSingle(p.SlotToBlocktime))
	builder.WriteString("\n")
	return builder.String()
}

func quoteSingle(s string) string {
	return fmt.Sprintf("'%s'", s)
}

func NewBuilder_CidToOffset(
	epoch uint64,
	rootCid cid.Cid,
	network indexes.Network,
	tmpDir string,
	numItems uint64,
) (*indexes.CidToOffsetAndSize_Writer, error) {
	tmpDir = filepath.Join(tmpDir, "index-cid-to-offset-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cid_to_offset_and_size tmp dir: %w", err)
	}
	index, err := indexes.NewWriter_CidToOffsetAndSize(
		epoch,
		rootCid,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cid-to-offset-and-size index: %w", err)
	}
	return index, nil
}

func NewBuilder_SignatureToCid(
	epoch uint64,
	rootCid cid.Cid,
	network indexes.Network,
	tmpDir string,
	numItems uint64,
) (*indexes.SigToCid_Writer, error) {
	tmpDir = filepath.Join(tmpDir, "index-sig-to-cid-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sig_to_cid tmp dir: %w", err)
	}
	index, err := indexes.NewWriter_SigToCid(
		epoch,
		rootCid,
		network,
		tmpDir,
		numItems,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sig_to_cid index: %w", err)
	}
	return index, nil
}

func NewBuilder_SlotToCid(
	epoch uint64,
	rootCid cid.Cid,
	network indexes.Network,
	tmpDir string,
) (*indexes.SlotToCid_Writer, error) {
	tmpDir = filepath.Join(tmpDir, "index-slot-to-cid-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create slot_to_cid tmp dir: %w", err)
	}
	index, err := indexes.NewWriter_SlotToCid(
		epoch,
		rootCid,
		network,
		tmpDir,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create slot_to_cid index: %w", err)
	}
	return index, nil
}

func verifyAllIndexes(
	ctx context.Context,
	carPaths []string,
	indexes *IndexPaths,
	numTotalItems uint64,
) error {
	// Check if the CAR file exists:
	err := allFilesExist(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}

	rd, err := readasonecar.NewFromFilepaths(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	defer rd.Close()

	cid_to_offset_and_size, err := OpenIndex_CidToOffset(
		indexes.CidToOffsetAndSize,
	)
	if err != nil {
		return fmt.Errorf("failed to open cid_to_offset_and_size index: %w", err)
	}
	defer cid_to_offset_and_size.Close()

	slot_to_cid, err := OpenIndex_SlotToCid(
		indexes.SlotToCid,
	)
	if err != nil {
		return fmt.Errorf("failed to open slot_to_cid index: %w", err)
	}
	defer slot_to_cid.Close()

	sig_to_cid, err := OpenIndex_SigToCid(
		indexes.SignatureToCid,
	)
	if err != nil {
		return fmt.Errorf("failed to open sig_to_cid index: %w", err)
	}
	defer sig_to_cid.Close()

	var sig_exists *bucketteer.Reader
	if indexes.SignatureExists != "" {
		sig_exists, err = bucketteer.OpenMMAP(
			indexes.SignatureExists,
		)
		if err != nil {
			return fmt.Errorf("failed to open sig_exists index: %w", err)
		}
		defer sig_exists.Close()
	}

	slot_to_blocktime, err := OpenIndex_SlotToBlocktime(
		indexes.SlotToBlocktime,
	)
	if err != nil {
		return fmt.Errorf("failed to open slot_to_blocktime index: %w", err)
	}

	numIndexedOffsets := uint64(0)
	numIndexedBlocks := uint64(0)
	numIndexedTransactions := uint64(0)
	klog.Infof("Verifying indexes...")
	lastCheckpoint := time.Now()
	var eta time.Duration
	startedAt := time.Now()
	for {
		sectionOffset, ok := rd.GetGlobalOffsetForNextRead()
		if !ok {
			break
		}
		_cid, sectionLength, buf, err := rd.NextNodeBytes()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		offset, err := cid_to_offset_and_size.Get(_cid)
		if err != nil {
			return fmt.Errorf("failed to lookup offset for %s: %w", _cid, err)
		}
		if offset.Offset != sectionOffset {
			return fmt.Errorf("offset mismatch for %s: %d != %d", _cid, offset, sectionOffset)
		}
		if offset.Size != sectionLength {
			return fmt.Errorf("length mismatch for %s: %d != %d", _cid, offset, sectionLength)
		}

		numIndexedOffsets++

		rawData := buf.Bytes()
		kind := iplddecoders.Kind(rawData[1])
		switch kind {
		case iplddecoders.KindBlock:
			{
				block, err := iplddecoders.DecodeBlock(rawData)
				if err != nil {
					return fmt.Errorf("failed to decode block: %w", err)
				}

				{
					got, err := slot_to_cid.Get(uint64(block.Slot))
					if err != nil {
						return fmt.Errorf("failed to index slot to cid: %w", err)
					}
					if !got.Equals(_cid) {
						return fmt.Errorf("slot to cid mismatch for %d: expected cid %s, got %s", block.Slot, _cid, got)
					}
				}

				{
					blocktime, err := slot_to_blocktime.Get(uint64(block.Slot))
					if err != nil {
						return fmt.Errorf("failed to index slot to blocktime: %w", err)
					}
					if blocktime != int64(block.Meta.Blocktime) {
						return fmt.Errorf("blocktime mismatch for %d: expected %d, got %d", block.Slot, block.Meta.Blocktime, blocktime)
					}
				}
				numIndexedBlocks++
			}
		case iplddecoders.KindTransaction:
			{
				txNode, err := iplddecoders.DecodeTransaction(rawData)
				if err != nil {
					return fmt.Errorf("failed to decode transaction: %w", err)
				}

				sig, err := tooling.ReadFirstSignature(txNode.Data.Bytes())
				if err != nil {
					return fmt.Errorf("failed to read signature: %w", err)
				}

				got, err := sig_to_cid.Get(sig)
				if err != nil {
					return fmt.Errorf("failed to index signature to cid: %w", err)
				}
				if !got.Equals(_cid) {
					return fmt.Errorf("sig to cid mismatch for sig %s: expected cid %s, got %s", sig, _cid, got)
				}

				if sig_exists != nil {
					if has, err := sig_exists.Has(sig); err != nil {
						return fmt.Errorf("failed to check if sig exists in sig_exists index: %w", err)
					} else if !has {
						return fmt.Errorf("sig %s does not exist in sig_exists index", sig)
					}
				}
				numIndexedTransactions++
			}
		}

		if numIndexedOffsets%1_000_000 == 0 && numIndexedOffsets > 0 && numTotalItems > 0 {
			timeForChunk := time.Since(lastCheckpoint)
			numChunksLeft := ((numTotalItems - numIndexedOffsets) / 1_000_000) + 1
			eta = timeForChunk * time.Duration(numChunksLeft)
			lastCheckpoint = time.Now()
		}
		if numIndexedOffsets%100_000 == 0 {
			if numTotalItems > 0 {
				var etaString string
				if eta > 0 {
					etaString = fmt.Sprintf(", ETA: %s   ", eta.Truncate(time.Second).String())
				} else {
					etaString = ", ETA: ---   "
				}
				printToStderr(
					fmt.Sprintf("\rVerifying index: %s/%s items [%s%%] %s",
						humanize.Comma(int64(numIndexedOffsets)),
						humanize.Comma(int64(numTotalItems)),
						humanize.CommafWithDigits(float64(numIndexedOffsets)/float64(numTotalItems)*100, 2),
						etaString,
					),
				)
			} else {
				printToStderr(
					fmt.Sprintf("\rVerifying index: %s items",
						humanize.Comma(int64(numIndexedOffsets)),
					),
				)
			}
		}
	}

	printToStderr(
		fmt.Sprintf(
			"\rVerified %s offsets, %s blocks, %s transactions in %s\n",
			humanize.Comma(int64(numIndexedOffsets)),
			humanize.Comma(int64(numIndexedBlocks)),
			humanize.Comma(int64(numIndexedTransactions)),
			time.Since(startedAt).Truncate(time.Second),
		))

	return nil
}

func OpenIndex_CidToOffset(
	indexFilePath string,
) (*indexes.CidToOffsetAndSize_Reader, error) {
	index, err := indexes.Open_CidToOffsetAndSize(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cid_to_offset_and_size index: %w", err)
	}
	return index, nil
}

func OpenIndex_SlotToCid(
	indexFilePath string,
) (*indexes.SlotToCid_Reader, error) {
	index, err := indexes.Open_SlotToCid(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open slot_to_cid index: %w", err)
	}
	return index, nil
}

func OpenIndex_SigToCid(
	indexFilePath string,
) (*indexes.SigToCid_Reader, error) {
	index, err := indexes.Open_SigToCid(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sig_to_cid index: %w", err)
	}
	return index, nil
}

func OpenIndex_SlotToBlocktime(
	indexFilePath string,
) (*blocktimeindex.Index, error) {
	index, err := blocktimeindex.FromFile(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open slot_to_cid index: %w", err)
	}
	return index, nil
}
