package gsfa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/manifest"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
)

type GsfaReader struct {
	epoch   *uint64
	offsets *indexes.PubkeyToOffsetAndSize_Reader
	ll      *linkedlog.LinkedLog
	man     *manifest.Manifest
}

func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// NewGsfaReader opens an existing index in READ-ONLY mode.
func NewGsfaReader(indexRootDir string) (*GsfaReader, error) {
	if ok, err := isDir(indexRootDir); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("provided path is not a directory: %s", indexRootDir)
	}
	index := &GsfaReader{}
	{
		offsetsIndex := filepath.Join(indexRootDir, string(indexes.Kind_PubkeyToOffsetAndSize)+".index")
		offsets, err := indexes.Open_PubkeyToOffsetAndSize(offsetsIndex)
		if err != nil {
			return nil, fmt.Errorf("error while opening offsets index: %w", err)
		}
		index.offsets = offsets
	}
	{
		ll, err := linkedlog.NewLinkedLog(filepath.Join(indexRootDir, "linked-log"))
		if err != nil {
			return nil, err
		}
		index.ll = ll
	}
	{
		man, err := manifest.NewManifest(filepath.Join(indexRootDir, "manifest"), indexmeta.Meta{})
		if err != nil {
			return nil, err
		}
		index.man = man
	}
	return index, nil
}

func (index *GsfaReader) SetEpoch(epoch uint64) {
	index.epoch = &epoch
}

func (index *GsfaReader) GetEpoch() (uint64, bool) {
	if index.epoch == nil {
		return 0, false
	}
	return *index.epoch, true
}

func (index *GsfaReader) Close() error {
	return errors.Join(
		index.offsets.Close(),
		index.ll.Close(),
	)
}

func (index *GsfaReader) Meta() indexmeta.Meta {
	return index.man.Meta()
}

func (index *GsfaReader) Version() uint64 {
	return index.man.Version()
}

func (index *GsfaReader) Get(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
) ([]linkedlog.OffsetAndSizeAndBlocktime, error) {
	if limit <= 0 {
		return []linkedlog.OffsetAndSizeAndBlocktime{}, nil
	}
	lastOffset, err := index.offsets.Get(pk)
	if err != nil {
		if compactindexsized.IsNotFound(err) {
			return nil, fmt.Errorf("pubkey %s not found: %w", pk, err)
		}
		return nil, fmt.Errorf("error while getting initial offset: %w", err)
	}
	debugln("locs.OffsetToFirst:", lastOffset)

	var allTransactionLocations []linkedlog.OffsetAndSizeAndBlocktime
	next := lastOffset // Start from the latest, and go back in time.

	for {
		if next == nil || next.IsZero() { // no previous.
			break
		}
		if limit > 0 && len(allTransactionLocations) >= limit {
			break
		}
		locations, newNext, err := index.ll.ReadWithSize(next.Offset, next.Size)
		if err != nil {
			return nil, fmt.Errorf("error while reading linked log with next=%d: %w", next, err)
		}
		debugln("sigIndexes:", locations, "newNext:", newNext)
		next = &newNext
		for _, sigIndex := range locations {
			if limit > 0 && len(allTransactionLocations) >= limit {
				break
			}
			allTransactionLocations = append(allTransactionLocations, sigIndex)
		}
	}
	return allTransactionLocations, nil
}

func (index *GsfaReader) GetBeforeUntil(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	before *solana.Signature, // Before this signature, exclusive (i.e. get signatures older than this signature, excluding it).
	until *solana.Signature, // Until this signature, inclusive (i.e. stop at this signature, including it).
	fetcher func(sigIndex linkedlog.OffsetAndSizeAndBlocktime) (solana.Signature, error),
) ([]linkedlog.OffsetAndSizeAndBlocktime, error) {
	if limit <= 0 {
		return []linkedlog.OffsetAndSizeAndBlocktime{}, nil
	}
	locs, err := index.offsets.Get(pk)
	if err != nil {
		if compactindexsized.IsNotFound(err) {
			return nil, fmt.Errorf("pubkey %s not found: %w", pk, err)
		}
		return nil, fmt.Errorf("error while getting initial offset: %w", err)
	}
	debugln("locs.OffsetToFirst:", locs)

	var allTransactionLocations []linkedlog.OffsetAndSizeAndBlocktime
	next := locs // Start from the latest, and go back in time.

	reachedBefore := false
	if before == nil {
		reachedBefore = true
	}

bigLoop:
	for {
		if next == nil || next.IsZero() { // no previous.
			break
		}
		if limit > 0 && len(allTransactionLocations) >= limit {
			break
		}
		locations, newNext, err := index.ll.ReadWithSize(next.Offset, next.Size)
		if err != nil {
			return nil, fmt.Errorf("error while reading linked log with next=%v: %w", next, err)
		}
		debugln("sigIndexes:", locations, "newNext:", newNext)
		next = &newNext
		for _, txLoc := range locations {
			sig, err := fetcher(txLoc)
			if err != nil {
				return nil, fmt.Errorf("error while getting signature at index=%v: %w", txLoc, err)
			}
			if !reachedBefore && sig == *before {
				reachedBefore = true
				continue
			}
			if !reachedBefore {
				continue
			}
			if limit > 0 && len(allTransactionLocations) >= limit {
				break
			}
			allTransactionLocations = append(allTransactionLocations, txLoc)
			if until != nil && sig == *until {
				break bigLoop
			}
		}
	}
	return allTransactionLocations, nil
}
