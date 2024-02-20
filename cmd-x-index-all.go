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
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	carv1 "github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

func newCmd_Index_all() *cli.Command {
	var verify bool
	var network indexes.Network
	return &cli.Command{
		Name:        "all",
		Usage:       "Create all the necessary indexes for a Solana epoch.",
		Description: "Given a CAR file containing a Solana epoch, create all the necessary indexes and save them in the specified index dir.",
		ArgsUsage:   "<car-path> <index-dir>",
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
				Value: "",
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
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexDir := c.Args().Get(1)
			tmpDir := c.String("tmp-dir")

			if carPath == "" {
				return fmt.Errorf("missing car-path argument")
			}
			if indexDir == "" {
				return fmt.Errorf("missing index-dir argument")
			}
			if ok, err := isDirectory(indexDir); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("index-dir is not a directory")
			}

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Took %s", time.Since(startedAt))
				}()
				klog.Infof("Creating all indexes for %s", carPath)
				klog.Infof("Indexes will be saved in %s", indexDir)

				indexPaths, numTotalItems, err := createAllIndexes(
					c.Context,
					network,
					tmpDir,
					carPath,
					indexDir,
				)
				if err != nil {
					return err
				}
				klog.Info("Indexes created:")
				fmt.Println(indexPaths.String())
				if verify {
					return verifyAllIndexes(
						context.Background(),
						carPath,
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

func createAllIndexes(
	ctx context.Context,
	network indexes.Network,
	tmpDir string,
	carPath string,
	indexDir string,
) (*IndexPaths, uint64, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return nil, 0, fmt.Errorf("CAR file %q does not exist", carPath)
	}

	carFile, err := os.Open(carPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open car file: %w", err)
	}
	defer carFile.Close()

	rd, err := newCarReader(carFile)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.header.Roots) != 1 {
		return nil, 0, fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.header.Roots))
	}
	// print roots:
	for _, root := range rd.header.Roots {
		klog.Infof("- Root: %s", root)
	}
	rootCID := rd.header.Roots[0]

	klog.Infof("Getting car file size")

	klog.Infof("Counting items in car file...")
	numItems, epochObject, err := carCountItemsByFirstByte(carPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count items in car file: %w", err)
	}
	if epochObject == nil {
		return nil, 0, fmt.Errorf("failed to find epoch object in the car file")
	}
	fmt.Println()
	klog.Infof("Found items in car file:")
	numTotalItems := uint64(0)
	var kinds []byte
	for kind := range numItems {
		kinds = append(kinds, kind)
	}
	for _, kind := range kinds {
		klog.Infof("  %s: %s items", iplddecoders.Kind(kind), humanize.Comma(int64(numItems[kind])))
		numTotalItems += numItems[kind]
	}
	klog.Infof("Total: %s items", humanize.Comma(int64(numTotalItems)))

	epoch := uint64(epochObject.Epoch)
	klog.Infof("This CAR file is for epoch %d and cluster %s", epoch, network)

	cid_to_offset_and_size, err := NewBuilder_CidToOffset(
		epoch,
		rootCID,
		network,
		tmpDir,
		numTotalItems,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create cid_to_offset_and_size index: %w", err)
	}
	defer cid_to_offset_and_size.Close()

	slot_to_cid, err := NewBuilder_SlotToCid(
		epoch,
		rootCID,
		network,
		tmpDir,
		numItems[byte(iplddecoders.KindBlock)],
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create slot_to_cid index: %w", err)
	}
	defer slot_to_cid.Close()

	sig_to_cid, err := NewBuilder_SignatureToCid(
		epoch,
		rootCID,
		network,
		tmpDir,
		numItems[byte(iplddecoders.KindTransaction)],
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create sig_to_cid index: %w", err)
	}
	defer sig_to_cid.Close()

	sigExistsFilepath := formatSigExistsIndexFilePath(indexDir, epoch, rootCID, network)
	sig_exists, err := bucketteer.NewWriter(
		sigExistsFilepath,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create sig_exists index: %w", err)
	}
	defer sig_exists.Close()

	totalOffset := uint64(0)
	{
		var buf bytes.Buffer
		if err = carv1.WriteHeader(rd.header, &buf); err != nil {
			return nil, 0, err
		}
		totalOffset = uint64(buf.Len())
	}

	numIndexedOffsets := uint64(0)
	numIndexedBlocks := uint64(0)
	numIndexedTransactions := uint64(0)
	lastCheckpoint := time.Now()
	klog.Infof("Indexing...")
	var eta time.Duration
	startedAt := time.Now()
	for {
		_cid, sectionLength, block, err := rd.NextNode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		err = cid_to_offset_and_size.Put(_cid, totalOffset, sectionLength)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to index cid to offset: %w", err)
		}
		numIndexedOffsets++

		kind := iplddecoders.Kind(block.RawData()[1])
		switch kind {
		case iplddecoders.KindBlock:
			{
				block, err := iplddecoders.DecodeBlock(block.RawData())
				if err != nil {
					return nil, 0, fmt.Errorf("failed to decode block: %w", err)
				}

				err = slot_to_cid.Put(uint64(block.Slot), _cid)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to index slot to cid: %w", err)
				}
				numIndexedBlocks++
			}
		case iplddecoders.KindTransaction:
			{
				txNode, err := iplddecoders.DecodeTransaction(block.RawData())
				if err != nil {
					return nil, 0, fmt.Errorf("failed to decode transaction: %w", err)
				}

				sig, err := readFirstSignature(txNode.Data.Bytes())
				if err != nil {
					return nil, 0, fmt.Errorf("failed to read signature: %w", err)
				}

				err = sig_to_cid.Put(sig, _cid)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to index signature to cid: %w", err)
				}

				sig_exists.Put(sig)

				numIndexedTransactions++
			}
		}

		totalOffset += sectionLength

		if numIndexedOffsets%1_000_000 == 0 && numIndexedOffsets > 0 {
			timeForChunk := time.Since(lastCheckpoint)
			numChunksLeft := ((numTotalItems - numIndexedOffsets) / 1_000_000) + 1
			eta = timeForChunk * time.Duration(numChunksLeft)
			lastCheckpoint = time.Now()
		}
		if numIndexedOffsets%100_000 == 0 {
			var etaString string
			if eta > 0 {
				etaString = fmt.Sprintf(" ETA: %s   ", eta.Truncate(time.Second).String())
			} else {
				etaString = ", ETA: ---   "
			}
			printToStderr(
				fmt.Sprintf("\rIndexing: %s/%s items [%s%%] %s",
					humanize.Comma(int64(numIndexedOffsets)),
					humanize.Comma(int64(numTotalItems)),
					humanize.CommafWithDigits(float64(numIndexedOffsets)/float64(numTotalItems)*100, 2),
					etaString,
				),
			)
		}
	}
	printToStderr(
		fmt.Sprintf("\rIndexed %s items in %s                           \n",
			humanize.Comma(int64(numIndexedOffsets)),
			time.Since(startedAt).Truncate(time.Second),
		),
	)
	printToStderr("\n")
	klog.Infof(
		"Indexed %s offsets, %s blocks, %s transactions",
		humanize.Comma(int64(numIndexedOffsets)),
		humanize.Comma(int64(numIndexedBlocks)),
		humanize.Comma(int64(numIndexedTransactions)),
	)

	klog.Infof("Preparing to seal indexes (DO NOT EXIT)...")

	paths := &IndexPaths{}
	paths.SignatureExists = sigExistsFilepath

	{
		wg := new(errgroup.Group)

		// seal the indexes
		wg.Go(func() error {
			klog.Infof("Sealing cid_to_offset_and_size index...")
			err = cid_to_offset_and_size.Seal(ctx, indexDir)
			if err != nil {
				return fmt.Errorf("failed to seal cid_to_offset_and_size index: %w", err)
			}
			paths.CidToOffsetAndSize = cid_to_offset_and_size.GetFilepath()
			klog.Infof("Successfully sealed cid_to_offset_and_size index: %s", paths.CidToOffsetAndSize)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing slot_to_cid index...")
			err = slot_to_cid.Seal(ctx, indexDir)
			if err != nil {
				return fmt.Errorf("failed to seal slot_to_cid index: %w", err)
			}
			paths.SlotToCid = slot_to_cid.GetFilepath()
			klog.Infof("Successfully sealed slot_to_cid index: %s", paths.SlotToCid)
			return nil
		})

		wg.Go(func() error {
			klog.Infof("Sealing sig_to_cid index...")
			err = sig_to_cid.Seal(ctx, indexDir)
			if err != nil {
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
			if _, err = sig_exists.Seal(meta); err != nil {
				return fmt.Errorf("failed to seal sig_exists index: %w", err)
			}
			klog.Infof("Successfully sealed sig_exists index: %s", paths.SignatureExists)
			return nil
		})

		if err := wg.Wait(); err != nil {
			return nil, 0, err
		}
	}

	return paths, numTotalItems, nil
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
	numItems uint64,
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
		numItems,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create slot_to_cid index: %w", err)
	}
	return index, nil
}

func verifyAllIndexes(
	ctx context.Context,
	carPath string,
	indexes *IndexPaths,
	numTotalItems uint64,
) error {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("CAR file %q does not exist", carPath)
	}

	carFile, err := os.Open(carPath)
	if err != nil {
		return fmt.Errorf("failed to open car file: %w", err)
	}
	defer carFile.Close()

	rd, err := newCarReader(carFile)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.header.Roots) != 1 {
		return fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.header.Roots))
	}

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
		sig_exists, err = bucketteer.Open(
			indexes.SignatureExists,
		)
		if err != nil {
			return fmt.Errorf("failed to open sig_exists index: %w", err)
		}
		defer sig_exists.Close()
	}

	totalOffset := uint64(0)
	{
		var buf bytes.Buffer
		if err = carv1.WriteHeader(rd.header, &buf); err != nil {
			return err
		}
		totalOffset = uint64(buf.Len())
	}

	numIndexedOffsets := uint64(0)
	numIndexedBlocks := uint64(0)
	numIndexedTransactions := uint64(0)
	klog.Infof("Verifying indexes...")
	lastCheckpoint := time.Now()
	var eta time.Duration
	startedAt := time.Now()
	for {
		_cid, sectionLength, block, err := rd.NextNode()
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
		if offset.Offset != totalOffset {
			return fmt.Errorf("offset mismatch for %s: %d != %d", _cid, offset, totalOffset)
		}
		if offset.Size != sectionLength {
			return fmt.Errorf("length mismatch for %s: %d != %d", _cid, offset, sectionLength)
		}

		numIndexedOffsets++

		kind := iplddecoders.Kind(block.RawData()[1])
		switch kind {
		case iplddecoders.KindBlock:
			{
				block, err := iplddecoders.DecodeBlock(block.RawData())
				if err != nil {
					return fmt.Errorf("failed to decode block: %w", err)
				}

				got, err := slot_to_cid.Get(uint64(block.Slot))
				if err != nil {
					return fmt.Errorf("failed to index slot to cid: %w", err)
				}
				if !got.Equals(_cid) {
					return fmt.Errorf("slot to cid mismatch for %d: expected cid %s, got %s", block.Slot, _cid, got)
				}
				numIndexedBlocks++
			}
		case iplddecoders.KindTransaction:
			{
				txNode, err := iplddecoders.DecodeTransaction(block.RawData())
				if err != nil {
					return fmt.Errorf("failed to decode transaction: %w", err)
				}

				sig, err := readFirstSignature(txNode.Data.Bytes())
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

		totalOffset += sectionLength

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
