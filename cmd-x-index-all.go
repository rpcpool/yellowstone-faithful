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

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	carv1 "github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_all() *cli.Command {
	var verify bool
	var epoch uint64
	var network indexes.Network
	return &cli.Command{
		Name:        "all",
		Description: "Given a CAR file containing a Solana epoch, create all the necessary indexes and save them in the specified index dir.",
		ArgsUsage:   "<car-path> <index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "verify",
				Usage:       "verify the index after creating it",
				Destination: &verify,
			},
			&cli.StringFlag{
				Name:  "tmp-dir",
				Usage: "temporary directory to use for storing intermediate files",
				Value: "",
			},
			&cli.Uint64Flag{
				Name:        "epoch",
				Usage:       "the epoch of the CAR file",
				Destination: &epoch,
				Required:    true,
			},
			&cli.StringFlag{
				Name:  "network",
				Usage: "the cluster of the epoch; one of: mainnet, testnet, devnet",
				Action: func(c *cli.Context, s string) error {
					network = indexes.Network(s)
					if !indexes.IsValidNetwork(network) {
						return fmt.Errorf("invalid network: %s", network)
					}
					return nil
				},
				Required: true,
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
				indexPaths, err := createAllIndexes(
					c.Context,
					epoch,
					network,
					tmpDir,
					carPath,
					indexDir,
				)
				if err != nil {
					return err
				}
				klog.Info("Indexes created:")
				veryPlainSdumpConfig.Dump(indexPaths)
				if verify {
					return verifyAllIndexes(context.Background(), carPath, indexPaths)
				}
				klog.Info("Skipping verification.")
			}
			return nil
		},
	}
}

var veryPlainSdumpConfig = spew.ConfigState{
	Indent:                  "  ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	DisableMethods:          true,
	DisablePointerMethods:   true,
	ContinueOnMethod:        true,
	SortKeys:                true,
}

func createAllIndexes(
	ctx context.Context,
	epoch uint64,
	network indexes.Network,
	tmpDir string,
	carPath string,
	indexDir string,
) (*IndexPaths, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("CAR file %q does not exist", carPath)
	}

	carFile, err := os.Open(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open car file: %w", err)
	}
	defer carFile.Close()

	rd, err := newCarReader(carFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create car reader: %w", err)
	}
	// check it has 1 root
	if len(rd.header.Roots) != 1 {
		return nil, fmt.Errorf("car file must have exactly 1 root, but has %d", len(rd.header.Roots))
	}
	// print roots:
	for _, root := range rd.header.Roots {
		klog.Infof("- Root: %s", root)
	}
	rootCID := rd.header.Roots[0]

	klog.Infof("Getting car file size")

	klog.Infof("Counting items in car file...")
	numItems, err := carCountItemsByFirstByte(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to count items in car file: %w", err)
	}
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

	cid_to_offset_and_size, err := NewBuilder_CidToOffset(
		epoch,
		rootCID,
		network,
		tmpDir,
		numTotalItems,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cid_to_offset index: %w", err)
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
		return nil, fmt.Errorf("failed to create slot_to_cid index: %w", err)
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
		return nil, fmt.Errorf("failed to create sig_to_cid index: %w", err)
	}
	defer sig_to_cid.Close()

	sigExistsFilepath := formatSigExistsIndexFilePath(indexDir, carPath, rootCID.String())
	sig_exists, err := bucketteer.NewWriter(
		sigExistsFilepath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sig_exists index: %w", err)
	}
	defer sig_exists.Close()

	totalOffset := uint64(0)
	{
		var buf bytes.Buffer
		if err = carv1.WriteHeader(rd.header, &buf); err != nil {
			return nil, err
		}
		totalOffset = uint64(buf.Len())
	}

	numIndexedOffsets := uint64(0)
	numIndexedBlocks := uint64(0)
	numIndexedTransactions := uint64(0)
	lastCheckpoint := time.Now()
	klog.Infof("Indexing...")
	for {
		_cid, sectionLength, block, err := rd.NextNode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		err = cid_to_offset_and_size.Put(_cid, totalOffset, sectionLength)
		if err != nil {
			return nil, fmt.Errorf("failed to index cid to offset: %w", err)
		}
		numIndexedOffsets++

		kind := iplddecoders.Kind(block.RawData()[1])
		switch kind {
		case iplddecoders.KindBlock:
			{
				block, err := iplddecoders.DecodeBlock(block.RawData())
				if err != nil {
					return nil, fmt.Errorf("failed to decode block: %w", err)
				}

				err = slot_to_cid.Put(uint64(block.Slot), _cid)
				if err != nil {
					return nil, fmt.Errorf("failed to index slot to cid: %w", err)
				}
				numIndexedBlocks++
			}
		case iplddecoders.KindTransaction:
			{
				txNode, err := iplddecoders.DecodeTransaction(block.RawData())
				if err != nil {
					return nil, fmt.Errorf("failed to decode transaction: %w", err)
				}

				sig, err := readFirstSignature(txNode.Data.Bytes())
				if err != nil {
					return nil, fmt.Errorf("failed to read signature: %w", err)
				}

				err = sig_to_cid.Put(sig, _cid)
				if err != nil {
					return nil, fmt.Errorf("failed to index signature to cid: %w", err)
				}

				sig_exists.Put(sig)

				numIndexedTransactions++
			}
		}

		totalOffset += sectionLength

		if numIndexedOffsets%100_000 == 0 {
			printToStderr(".")
		}
		if numIndexedOffsets%10_000_000 == 0 {
			timeFor10_000_000 := time.Since(lastCheckpoint)
			howMany10_000_000 := ((numTotalItems - numIndexedOffsets) / 10_000_000) + 1
			eta := timeFor10_000_000 * time.Duration(howMany10_000_000)

			printToStderr(
				"\n" + greenBackground(
					fmt.Sprintf(" %s (%s) ",
						humanize.Comma(int64(numIndexedOffsets)),
						time.Since(lastCheckpoint),
					),
				) + "ETA: " + eta.String() + "\n",
			)
			lastCheckpoint = time.Now()
		}
	}
	printToStderr("\n")
	klog.Infof(
		"Indexed %s offsets, %s blocks, %s transactions",
		humanize.Comma(int64(numIndexedOffsets)),
		humanize.Comma(int64(numIndexedBlocks)),
		humanize.Comma(int64(numIndexedTransactions)),
	)

	klog.Infof("Preparing to seal indexes...")

	paths := &IndexPaths{}
	paths.SignatureExists = sigExistsFilepath

	klog.Infof("Root CID: %s", rootCID)

	{
		// seal the indexes
		{
			klog.Infof("Sealing cid_to_offset_and_size index...")
			err = cid_to_offset_and_size.Seal(ctx, indexDir)
			if err != nil {
				return nil, fmt.Errorf("failed to seal cid_to_offset index: %w", err)
			}
			paths.CidToOffsetAndSize = cid_to_offset_and_size.GetFilepath()
			klog.Infof("Successfully sealed cid_to_offset_and_size index: %s", paths.CidToOffsetAndSize)
		}

		{
			klog.Infof("Sealing slot_to_cid index...")
			err = slot_to_cid.Seal(ctx, indexDir)
			if err != nil {
				return nil, fmt.Errorf("failed to seal slot_to_cid index: %w", err)
			}
			paths.SlotToCid = slot_to_cid.GetFilepath()
			klog.Infof("Successfully sealed slot_to_cid index: %s", paths.SlotToCid)
		}

		{
			klog.Infof("Sealing sig_to_cid index...")
			err = sig_to_cid.Seal(ctx, indexDir)
			if err != nil {
				return nil, fmt.Errorf("failed to seal sig_to_cid index: %w", err)
			}
			paths.SignatureToCid = sig_to_cid.GetFilepath()
			klog.Infof("Successfully sealed sig_to_cid index: %s", paths.SignatureToCid)
		}

		{
			klog.Infof("Sealing sig_exists index...")
			meta := map[string]string{
				"root_cid": rootCID.String(),
			}
			if _, err = sig_exists.Seal(meta); err != nil {
				return nil, fmt.Errorf("failed to seal sig_exists index: %w", err)
			}
			klog.Infof("Successfully sealed sig_exists index: %s", paths.SignatureExists)
		}
	}

	return paths, nil
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

func NewBuilder_CidToOffset(
	epoch uint64,
	rootCid cid.Cid,
	network indexes.Network,
	tmpDir string,
	numItems uint64,
) (*indexes.CidToOffsetAndSize_Writer, error) {
	tmpDir = filepath.Join(tmpDir, "index-cid-to-offset-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cid_to_offset tmp dir: %w", err)
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

	cid_to_offset, err := OpenIndex_CidToOffset(
		indexes.CidToOffsetAndSize,
	)
	if err != nil {
		return fmt.Errorf("failed to open cid_to_offset index: %w", err)
	}
	defer cid_to_offset.Close()

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
	for {
		_cid, sectionLength, block, err := rd.NextNode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		offset, err := cid_to_offset.Get(_cid)
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

		if numIndexedOffsets%100_000 == 0 {
			printToStderr(".")
		}
		if numIndexedOffsets%1_000_000 == 0 {
			printToStderr(
				"\n" + greenBackground(
					fmt.Sprintf(" %s (%s) ",
						humanize.Comma(int64(numIndexedOffsets)),
						time.Since(lastCheckpoint),
					),
				) + "\n",
			)
			lastCheckpoint = time.Now()
		}
	}
	printToStderr("\n")
	klog.Infof(
		"Verified %s offsets, %s blocks, %s transactions",
		humanize.Comma(int64(numIndexedOffsets)),
		humanize.Comma(int64(numIndexedBlocks)),
		humanize.Comma(int64(numIndexedTransactions)),
	)

	return nil
}

func OpenIndex_CidToOffset(
	indexFilePath string,
) (*indexes.CidToOffsetAndSize_Reader, error) {
	index, err := indexes.Open_CidToOffsetAndSize(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	return index, nil
}

func OpenIndex_SlotToCid(
	indexFilePath string,
) (*indexes.SlotToCid_Reader, error) {
	index, err := indexes.Open_SlotToCid(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	return index, nil
}

func OpenIndex_SigToCid(
	indexFilePath string,
) (*indexes.SigToCid_Reader, error) {
	index, err := indexes.Open_SigToCid(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	return index, nil
}
