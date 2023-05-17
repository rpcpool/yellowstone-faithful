package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/compactindex36"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/ipld/ipldbindcode"
	"k8s.io/klog/v2"
)

// CreateIndex_sig2cid creates an index file that maps transaction signatures to CIDs.
func CreateIndex_sig2cid(ctx context.Context, carPath string, indexDir string) (string, error) {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("CAR file %q does not exist", carPath)
	}

	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to open CAR file: %w", err)
	}

	// check it has 1 root
	roots, err := cr.Roots()
	if err != nil {
		return "", fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return "", fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}

	// TODO: use another way to precisely count the number of solana Blocks in the CAR file.
	klog.Infof("Counting items in car file...")
	numItems, err := carCountItems(carPath)
	if err != nil {
		return "", fmt.Errorf("failed to count items in car file: %w", err)
	}
	klog.Infof("Found %d items in car file", numItems)

	klog.Infof("Creating builder with %d items", numItems)
	c2o, err := compactindex36.NewBuilder(
		"",
		uint(numItems), // TODO: what if the number of real items is less than this?
		(0),
	)
	if err != nil {
		return "", fmt.Errorf("failed to open index store: %w", err)
	}
	defer c2o.Close()

	numItemsIndexed := uint64(0)
	klog.Infof("Indexing...")

	dr, err := cr.DataReader()
	if err != nil {
		return "", fmt.Errorf("failed to get data reader: %w", err)
	}

	// Iterate over all Transactions in the CAR file and put them into the index,
	// using the transaction signature as the key and the CID as the value.
	err = FindTransactions(
		ctx,
		dr,
		func(c cid.Cid, txNode *ipldbindcode.Transaction) error {
			var tx solana.Transaction
			if err := bin.UnmarshalBin(&tx, txNode.Data); err != nil {
				return fmt.Errorf("failed to unmarshal transaction: %w", err)
			} else if len(tx.Signatures) == 0 {
				panic("no signatures")
			}
			sig := tx.Signatures[0]

			var buf [36]byte
			copy(buf[:], c.Bytes()[:36])

			err = c2o.Insert(sig[:], buf)
			if err != nil {
				return fmt.Errorf("failed to put cid to offset: %w", err)
			}

			numItemsIndexed++
			if numItemsIndexed%10_000 == 0 {
				printToStderr(".")
			}
			return nil
		})
	if err != nil {
		return "", fmt.Errorf("failed to index; error while iterating over blocks: %w", err)
	}

	rootCID := roots[0]

	// Use the car file name and root CID to name the index file:
	indexFilePath := filepath.Join(indexDir, fmt.Sprintf("%s.%s.sig-to-cid.index", filepath.Base(carPath), rootCID.String()))

	klog.Infof("Creating index file at %s", indexFilePath)
	targetFile, err := os.Create(indexFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create index file: %w", err)
	}
	defer targetFile.Close()

	klog.Infof("Sealing index...")
	if err = c2o.Seal(ctx, targetFile); err != nil {
		return "", fmt.Errorf("failed to seal index: %w", err)
	}
	klog.Infof("Index created; %d items indexed", numItemsIndexed)
	return indexFilePath, nil
}

// VerifyIndex_sig2cid verifies that the index file is correct for the given car file.
// It does this by reading the car file and comparing the offsets in the index
// file to the offsets in the car file.
func VerifyIndex_sig2cid(ctx context.Context, carPath string, indexFilePath string) error {
	// Check if the CAR file exists:
	exists, err := fileExists(carPath)
	if err != nil {
		return fmt.Errorf("failed to check if CAR file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("CAR file %s does not exist", carPath)
	}

	// Check if the index file exists:
	exists, err = fileExists(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if index file exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index file %s does not exist", indexFilePath)
	}

	cr, err := carv2.OpenReader(carPath)
	if err != nil {
		return fmt.Errorf("failed to open CAR file: %w", err)
	}

	// check it has 1 root
	roots, err := cr.Roots()
	if err != nil {
		return fmt.Errorf("failed to get roots: %w", err)
	}
	// There should be only one root CID in the CAR file.
	if len(roots) != 1 {
		return fmt.Errorf("CAR file has %d roots, expected 1", len(roots))
	}

	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer indexFile.Close()

	c2o, err := compactindex36.Open(indexFile)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}

	dr, err := cr.DataReader()
	if err != nil {
		return fmt.Errorf("failed to get data reader: %w", err)
	}

	numItems := uint64(0)
	err = FindTransactions(
		ctx,
		dr,
		func(c cid.Cid, txNode *ipldbindcode.Transaction) error {
			var tx solana.Transaction
			if err := bin.UnmarshalBin(&tx, txNode.Data); err != nil {
				return fmt.Errorf("failed to unmarshal transaction: %w", err)
			} else if len(tx.Signatures) == 0 {
				panic("no signatures")
			}
			sig := tx.Signatures[0]

			got, err := findCidFromSignature(c2o, sig)
			if err != nil {
				return fmt.Errorf("failed to put cid to offset: %w", err)
			}

			if !got.Equals(c) {
				return fmt.Errorf("sig %s: expected cid %s, got %s", sig, c, got)
			}

			numItems++
			if numItems%10_000 == 0 {
				printToStderr(".")
			}

			return nil
		})
	if err != nil {
		return fmt.Errorf("failed to verify index; error while iterating over blocks: %w", err)
	}
	return nil
}

func findCidFromSignature(db *compactindex36.DB, sig solana.Signature) (cid.Cid, error) {
	bucket, err := db.LookupBucket(sig[:])
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to lookup bucket for %s: %w", sig, err)
	}
	got, err := bucket.Lookup(sig[:])
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to lookup value for %s: %w", sig, err)
	}
	l, c, err := cid.CidFromBytes(got[:])
	if err != nil {
		return cid.Cid{}, fmt.Errorf("failed to parse cid from bytes: %w", err)
	}
	if l != 36 {
		return cid.Cid{}, fmt.Errorf("unexpected cid length %d", l)
	}
	return c, nil
}
