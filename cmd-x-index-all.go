package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	carv1 "github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/compactindex"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"github.com/urfave/cli/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"k8s.io/klog/v2"
)

func newCmd_Index_all() *cli.Command {
	var verify bool
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
		},
		Subcommands: []*cli.Command{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexDir := c.Args().Get(1)
			tmpDir := c.String("tmp-dir")

			{
				startedAt := time.Now()
				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
				}()
				klog.Infof("Creating all indexes for %s", carPath)
				indexPaths, err := createAllIndexes(context.Background(), tmpDir, carPath, indexDir)
				if err != nil {
					return err
				}
				spew.Dump(indexPaths)
				klog.Info("Index created")
				if verify {
					panic("not implemented")
				}
			}
			return nil
		},
	}
}

func createAllIndexes(
	ctx context.Context,
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

	klog.Infof("Getting car file size")
	targetFileSize, err := getFileSize(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get car file size: %w", err)
	}

	// TODO: use another way to precisely count the number of solana Blocks in the CAR file.
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
		klog.Infof("  %s: %d items", iplddecoders.Kind(kind), numItems[kind])
		numTotalItems += numItems[kind]
	}

	cid_to_offset, err := NewBuilder_CidToOffset(
		tmpDir,
		indexDir,
		numTotalItems,
		targetFileSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cid_to_offset index: %w", err)
	}
	defer cid_to_offset.Close()

	slot_to_cid, err := NewBuilder_SlotToCid(
		tmpDir,
		indexDir,
		numItems[byte(iplddecoders.KindBlock)],
		targetFileSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create slot_to_cid index: %w", err)
	}
	defer slot_to_cid.Close()

	sig_to_cid, err := NewBuilder_SignatureToCid(
		tmpDir,
		indexDir,
		numItems[byte(iplddecoders.KindTransaction)],
		targetFileSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sig_to_cid index: %w", err)
	}
	defer sig_to_cid.Close()

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
	klog.Infof("Indexing...")
	for {
		_cid, sectionLength, block, err := rd.NextNode()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// klog.Infof("key: %s, offset: %d", bin.FormatByteSlice(c.Bytes()), totalOffset)

		err = cid_to_offset.Put(_cid, totalOffset)
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

				var tx solana.Transaction
				txBuffer := new(bytes.Buffer)
				txBuffer.Write(txNode.Data.Data)
				if txNode.Data.Total > 1 {
					// TODO: handle this case
					continue
				}
				if err := bin.UnmarshalBin(&tx, txBuffer.Bytes()); err != nil {
					return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
				} else if len(tx.Signatures) == 0 {
					panic("no signatures")
				}
				sig := tx.Signatures[0]

				err = sig_to_cid.Put(sig, _cid)
				if err != nil {
					return nil, fmt.Errorf("failed to index signature to cid: %w", err)
				}
				numIndexedTransactions++
			}
		}

		totalOffset += sectionLength

		numIndexedOffsets++
		if numIndexedOffsets%100_000 == 0 {
			printToStderr(".")
		}
	}
	klog.Infof("Indexed %d offsets, %d blocks, %d transactions", numIndexedOffsets, numIndexedBlocks, numIndexedTransactions)

	rootCID := rd.header.Roots[0]
	paths := &IndexPaths{}

	klog.Infof("Root CID: %s", rootCID)

	{
		// seal the indexes
		klog.Infof("Sealing cid_to_offset index...")
		paths.CidToOffset, err = cid_to_offset.Seal(ctx, carPath, rootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to seal cid_to_offset index: %w", err)
		}
		klog.Infof("Sealed cid_to_offset index: %s", paths.CidToOffset)

		klog.Infof("Sealing slot_to_cid index...")
		paths.SlotToCid, err = slot_to_cid.Seal(ctx, carPath, rootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to seal slot_to_cid index: %w", err)
		}
		klog.Infof("Sealed slot_to_cid index: %s", paths.SlotToCid)

		klog.Infof("Sealing sig_to_cid index...")
		paths.SignatureToCid, err = sig_to_cid.Seal(ctx, carPath, rootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to seal sig_to_cid index: %w", err)
		}
		klog.Infof("Sealed sig_to_cid index: %s", paths.SignatureToCid)
	}

	return paths, nil
}

type IndexPaths struct {
	CidToOffset    string
	SlotToCid      string
	SignatureToCid string
}

type Builder_CidToOffset struct {
	tmpDir   string
	indexDir string
	carPath  string
	index    *compactindex.Builder
}

func NewBuilder_CidToOffset(
	tmpDir string,
	indexDir string,
	numItems uint64,
	targetFileSize uint64,
) (*Builder_CidToOffset, error) {
	tmpDir = filepath.Join(tmpDir, "index-cid-to-offset-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}
	index, err := compactindex.NewBuilder(
		tmpDir,
		uint(numItems),
		(targetFileSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	return &Builder_CidToOffset{
		tmpDir:   tmpDir,
		indexDir: indexDir,
		index:    index,
	}, nil
}

func (b *Builder_CidToOffset) Put(c cid.Cid, offset uint64) error {
	return b.index.Insert(c.Bytes(), offset)
}

func (b *Builder_CidToOffset) Close() error {
	return b.index.Close()
}

func (b *Builder_CidToOffset) Seal(ctx context.Context, carPath string, rootCid cid.Cid) (string, error) {
	indexFilePath := filepath.Join(b.indexDir, fmt.Sprintf("%s.%s.cid-to-offset.index", filepath.Base(carPath), rootCid.String()))
	klog.Infof("Creating index file at %s", indexFilePath)
	targetFile, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create index file: %w", err)
	}
	defer targetFile.Close()

	klog.Infof("Sealing index...")
	if err = b.index.Seal(ctx, targetFile); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	return indexFilePath, nil
}

type Builder_SignatureToCid struct {
	tmpDir   string
	indexDir string
	carPath  string
	index    *compactindex36.Builder
}

func NewBuilder_SignatureToCid(
	tmpDir string,
	indexDir string,
	numItems uint64,
	targetFileSize uint64,
) (*Builder_SignatureToCid, error) {
	tmpDir = filepath.Join(tmpDir, "index-sig-to-cid-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}
	index, err := compactindex36.NewBuilder(
		tmpDir,
		uint(numItems),
		(targetFileSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	return &Builder_SignatureToCid{
		tmpDir:   tmpDir,
		indexDir: indexDir,
		index:    index,
	}, nil
}

func (b *Builder_SignatureToCid) Put(signature solana.Signature, cid cid.Cid) error {
	var buf [36]byte
	copy(buf[:], cid.Bytes()[:36])
	return b.index.Insert(signature[:], buf)
}

func (b *Builder_SignatureToCid) Close() error {
	return b.index.Close()
}

func (b *Builder_SignatureToCid) Seal(ctx context.Context, carPath string, rootCid cid.Cid) (string, error) {
	indexFilePath := filepath.Join(b.indexDir, fmt.Sprintf("%s.%s.sig-to-cid.index", filepath.Base(carPath), rootCid.String()))
	klog.Infof("Creating index file at %s", indexFilePath)
	targetFile, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create index file: %w", err)
	}
	defer targetFile.Close()

	klog.Infof("Sealing index...")
	if err = b.index.Seal(ctx, targetFile); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	return indexFilePath, nil
}

type Builder_SlotToCid struct {
	tmpDir   string
	indexDir string
	carPath  string
	index    *compactindex36.Builder
}

func NewBuilder_SlotToCid(
	tmpDir string,
	indexDir string,
	numItems uint64,
	targetFileSize uint64,
) (*Builder_SlotToCid, error) {
	tmpDir = filepath.Join(tmpDir, "index-slot-to-cid-"+time.Now().Format("20060102-150405.000000000")+fmt.Sprintf("-%d", rand.Int63()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}
	index, err := compactindex36.NewBuilder(
		tmpDir,
		uint(numItems),
		(targetFileSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	return &Builder_SlotToCid{
		tmpDir:   tmpDir,
		indexDir: indexDir,
		index:    index,
	}, nil
}

func (b *Builder_SlotToCid) Put(slot uint64, cid cid.Cid) error {
	var buf [36]byte
	copy(buf[:], cid.Bytes()[:36])
	return b.index.Insert(uint64ToLeBytes(slot), buf)
}

func (b *Builder_SlotToCid) Close() error {
	return b.index.Close()
}

func (b *Builder_SlotToCid) Seal(ctx context.Context, carPath string, rootCid cid.Cid) (string, error) {
	indexFilePath := filepath.Join(b.indexDir, fmt.Sprintf("%s.%s.slot-to-cid.index", filepath.Base(carPath), rootCid.String()))
	klog.Infof("Creating index file at %s", indexFilePath)
	targetFile, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create index file: %w", err)
	}
	defer targetFile.Close()

	klog.Infof("Sealing index...")
	if err = b.index.Seal(ctx, targetFile); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	return indexFilePath, nil
}