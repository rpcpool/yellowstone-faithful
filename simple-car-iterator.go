package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/ipld/ipldbindcode"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"go.firedancer.io/radiance/pkg/compactindex"
	"k8s.io/klog/v2"
)

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("path %s is a directory", path)
	}
	return true, nil
}

func openCarReaderWithCidIndex(carPath string, indexFilePath string) (*SimpleIterator, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("CAR file %s does not exist", carPath)
	}

	// Check if the index file exists:
	exists, err = fileExists(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	// Open CAR file:
	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}

	// Get root CIDs in the CARv1 file.
	roots, err := cr.Roots()
	if err != nil {
		return nil, fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return nil, fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}

	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}

	klog.Infof("Reading index from %s", indexFilePath)
	c2o, err := compactindex.Open(indexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	klog.Infof("Done reading index from %s", indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}

	iter := &SimpleIterator{
		c2o:       c2o,
		cr:        cr,
		indexFile: indexFile,
	}

	// Try finding the root CID in the index;
	// if it's not there, then the index is not for this CAR file.
	for _, root := range roots {
		node, err := getRawNodeFromCarByCid(
			newOffsetFinderFunc(iter.c2o),
			iter.cr,
			root,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get root node: %w", err)
		}
		if node == nil {
			return nil, fmt.Errorf("root node is nil")
		}
		klog.Infof("Root CID: %s", root)
		if !node.Cid().Equals(root) {
			return nil, fmt.Errorf("root CID %s does not match %s", root, node.Cid())
		}
	}
	return iter, nil
}

type SimpleIterator struct {
	c2o       *compactindex.DB // index from cid to offset in the CAR file
	cr        *carv2.Reader    // the CAR file
	indexFile *os.File         // the index file
}

func NewSimpleCarIterator(carPath string, indexFilePath string) (*SimpleIterator, error) {
	return openCarReaderWithCidIndex(carPath, indexFilePath)
}

// Close closes the underlying resources.
func (t *SimpleIterator) Close() error {
	t.indexFile.Close()
	return t.cr.Close()
}

var ErrNotFound = errors.New("not found")

// Get returns the block with the given CID.
func (t *SimpleIterator) Get(ctx context.Context, c cid.Cid) (*blocks.BasicBlock, error) {
	node, err := getRawNodeFromCarByCid(
		newOffsetFinderFunc(t.c2o),
		t.cr,
		c,
	)
	return node, err
}

func newOffsetFinderFunc(c2o *compactindex.DB) func(ctx context.Context, c cid.Cid) (uint64, error) {
	return func(ctx context.Context, c cid.Cid) (uint64, error) {
		bucket, err := c2o.LookupBucket(c.Bytes())
		if err != nil {
			if err == compactindex.ErrNotFound {
				return 0, ErrNotFound
			}
			return 0, fmt.Errorf("failed to lookup bucket: %w", err)
		}
		offset, err := bucket.Lookup(c.Bytes())
		if err != nil {
			if err == compactindex.ErrNotFound {
				return 0, ErrNotFound
			}
			return 0, fmt.Errorf("failed to lookup offset: %w", err)
		}
		return offset, nil
	}
}

// GetEpoch returns the Epoch root.
func (t *SimpleIterator) GetEpoch(ctx context.Context) (*ipldbindcode.Epoch, error) {
	roots, err := t.cr.Roots()
	if err != nil {
		return nil, fmt.Errorf("failed to get roots: %w", err)
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("expected 1 root, got %d", len(roots))
	}
	epochRaw, err := t.Get(ctx, roots[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get Epoch root: %w", err)
	}
	epoch, err := iplddecoders.DecodeEpoch(epochRaw.RawData())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Epoch root object: %w", err)
	}
	return epoch, nil
}

// GetSubset returns the Subset with the given CID.
func (t *SimpleIterator) GetSubset(ctx context.Context, c cid.Cid) (*ipldbindcode.Subset, error) {
	node, err := getRawNodeFromCarByCid(
		newOffsetFinderFunc(t.c2o),
		t.cr,
		c,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Subset: %w", err)
	}
	subset, err := iplddecoders.DecodeSubset(node.RawData())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Subset: %w", err)
	}
	return subset, nil
}

// GetBlock returns the Block with the given CID.
func (t *SimpleIterator) GetBlock(ctx context.Context, c cid.Cid) (*ipldbindcode.Block, error) {
	node, err := getRawNodeFromCarByCid(
		newOffsetFinderFunc(t.c2o),
		t.cr,
		c,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Block: %w", err)
	}
	block, err := iplddecoders.DecodeBlock(node.RawData())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Block: %w", err)
	}
	return block, nil
}

// GetEntry returns the Entry with the given CID.
func (t *SimpleIterator) GetEntry(ctx context.Context, c cid.Cid) (*ipldbindcode.Entry, error) {
	node, err := getRawNodeFromCarByCid(
		newOffsetFinderFunc(t.c2o),
		t.cr,
		c,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Entry: %w", err)
	}
	entry, err := iplddecoders.DecodeEntry(node.RawData())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Entry: %w", err)
	}
	return entry, nil
}

// GetTransaction returns the Transaction with the given CID.
func (t *SimpleIterator) GetTransaction(ctx context.Context, c cid.Cid) (*ipldbindcode.Transaction, error) {
	node, err := getRawNodeFromCarByCid(
		newOffsetFinderFunc(t.c2o),
		t.cr,
		c,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Transaction: %w", err)
	}
	tx, err := iplddecoders.DecodeTransaction(node.RawData())
	if err != nil {
		return nil, fmt.Errorf("failed to decode Transaction: %w", err)
	}
	return tx, nil
}

// FindSubsets calls the callback for each Subset in the CAR file.
// It stops iterating if the callback returns an error.
// It works by iterating over all objects in the CAR file and
// calling the callback for each object that is a Subset.
func (t *SimpleIterator) FindSubsets(ctx context.Context, callback func(*ipldbindcode.Subset) error) error {
	dr, err := t.cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}
	rd, err := car.NewCarReader(dr)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	for {
		block, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		{
			if block.RawData()[1] != byte(iplddecoders.KindSubset) {
				continue
			}
			decoded, err := iplddecoders.DecodeSubset(block.RawData())
			if err != nil {
				// TODO: log error, or return error?
				continue
			}
			err = callback(decoded)
			if err != nil {
				if err == ErrStopIteration {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

var ErrStopIteration = errors.New("stop iteration")

// FindBlocks calls the callback for each solana Block in the CAR file.
// It stops iterating if the callback returns an error.
// It works by iterating over all objects in the CAR file and
// calling the callback for each object that is a Block.
func (t *SimpleIterator) FindBlocks(ctx context.Context, callback func(*ipldbindcode.Block) error) error {
	dr, err := t.cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}
	rd, err := car.NewCarReader(dr)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	for {
		block, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		{
			if block.RawData()[1] != byte(iplddecoders.KindBlock) {
				continue
			}
			decoded, err := iplddecoders.DecodeBlock(block.RawData())
			if err != nil {
				continue
			}
			err = callback(decoded)
			if err != nil {
				if err == ErrStopIteration {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

// FindEntries calls the callback for each solana Entry in the CAR file.
// It stops iterating if the callback returns an error.
// It works by iterating over all objects in the CAR file and
// calling the callback for each object that is an Entry.
func (t *SimpleIterator) FindEntries(ctx context.Context, callback func(*ipldbindcode.Entry) error) error {
	dr, err := t.cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}
	rd, err := car.NewCarReader(dr)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	for {
		block, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		{
			if block.RawData()[1] != byte(iplddecoders.KindEntry) {
				continue
			}
			decoded, err := iplddecoders.DecodeEntry(block.RawData())
			if err != nil {
				continue
			}
			err = callback(decoded)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// FindTransactions calls the callback for each solana Transaction in the CAR file.
// It stops iterating if the callback returns an error.
// It works by iterating over all objects in the CAR file and
// calling the callback for each object that is a Transaction.
func (t *SimpleIterator) FindTransactions(ctx context.Context, callback func(*ipldbindcode.Transaction) error) error {
	dr, err := t.cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}
	rd, err := car.NewCarReader(dr)
	if err != nil {
		return fmt.Errorf("failed to create car reader: %w", err)
	}
	for {
		block, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		{
			if block.RawData()[1] != byte(iplddecoders.KindTransaction) {
				continue
			}
			decoded, err := iplddecoders.DecodeTransaction(block.RawData())
			if err != nil {
				continue
			}
			err = callback(decoded)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type offsetFinderFunc func(ctx context.Context, c cid.Cid) (uint64, error)

func getRawNodeFromCarByCid(offsetFinder offsetFinderFunc, cr *carv2.Reader, c cid.Cid) (*blocks.BasicBlock, error) {
	offset, err := offsetFinder(context.Background(), c)
	if err != nil {
		return nil, fmt.Errorf("failed to find object: %w", err)
	}
	dr, err := cr.DataReader()
	if err != nil {
		return nil, fmt.Errorf("failed to get data reader: %w", err)
	}
	// Seek to the offset.
	dr.Seek(int64(offset), io.SeekStart)
	br := bufio.NewReader(dr)

	// sectionLen, err := varint.ReadUvarint(br)
	// if err != nil {
	// 	return nil, err
	// }
	// // Read the CID.
	// cidLen, gotCid, err := cid.CidFromReader(br)
	// if err != nil {
	// 	return nil, err
	// }
	// remainingSectionLen := int64(sectionLen) - int64(cidLen)
	// // Read the data.
	// data := make([]byte, remainingSectionLen)
	// if _, err := io.ReadFull(br, data); err != nil {
	// 	return nil, err
	// }
	// Read node.
	gotCid, data, err := util.ReadNode(br)
	if err != nil {
		return nil, err
	}
	// Verify that the CID we read matches the one we expected.
	if !gotCid.Equals(c) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", c, gotCid)
	}
	// Create the block.
	bl, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}
	return bl, nil
}
