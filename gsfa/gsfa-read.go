package gsfa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/manifest"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	"github.com/rpcpool/yellowstone-faithful/gsfa/sff"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store"
)

type GsfaReader struct {
	offsets *offsetstore.OffsetStore
	ll      *linkedlog.LinkedLog
	sff     *sff.SignaturesFlatFile
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
		offsetsIndexDir := filepath.Join(indexRootDir, "offsets-index")
		offsets, err := offsetstore.OpenOffsetStore(
			context.Background(),
			filepath.Join(offsetsIndexDir, "index"),
			filepath.Join(offsetsIndexDir, "data"),
			store.IndexBitSize(22), // NOTE: if you don't specify this, the final size is smaller.
			store.GCInterval(time.Hour),
		)
		if err != nil {
			return nil, fmt.Errorf("error while opening index: %w", err)
		}
		index.offsets = offsets
		index.offsets.Start()
	}
	{
		ll, err := linkedlog.NewLinkedLog(filepath.Join(indexRootDir, "linked-log"))
		if err != nil {
			return nil, err
		}
		index.ll = ll
	}
	{
		sff, err := sff.NewSignaturesFlatFile(filepath.Join(indexRootDir, "signatures-flatfile"))
		if err != nil {
			return nil, err
		}
		index.sff = sff
	}
	{
		man, err := manifest.NewManifest(filepath.Join(indexRootDir, "manifest"))
		if err != nil {
			return nil, err
		}
		index.man = man
	}
	return index, nil
}

func (index *GsfaReader) Close() error {
	return errors.Join(
		index.offsets.Close(),
		index.ll.Close(),
		index.sff.Close(),
	)
}

func (index *GsfaReader) Get(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
) ([]solana.Signature, error) {
	locs, err := index.offsets.Get(context.Background(), pk)
	if err != nil {
		if offsetstore.IsNotFound(err) {
			return nil, offsetstore.ErrNotFound{PubKey: pk}
		}
		return nil, fmt.Errorf("error while getting initial offset: %w", err)
	}
	debugln("locs.OffsetToFirst:", locs)

	var sigs []solana.Signature
	next := locs.OffsetToLatest // Start from the latest, and go back in time.

	for {
		if next == 0 {
			break
		}
		if limit > 0 && len(sigs) >= limit {
			break
		}
		sigIndexes, newNext, err := index.ll.Read(next)
		if err != nil {
			return nil, fmt.Errorf("error while reading linked log with next=%d: %w", next, err)
		}
		debugln("sigIndexes:", sigIndexes, "newNext:", newNext)
		next = newNext
		for _, sigIndex := range sigIndexes {
			sig, err := index.sff.Get(sigIndex)
			if err != nil {
				return nil, fmt.Errorf("error while getting signature at index=%d: %w", sigIndex, err)
			}
			if limit > 0 && len(sigs) >= limit {
				break
			}
			sigs = append(sigs, sig)
		}
	}
	return sigs, nil
}

func (index *GsfaReader) GetBeforeUntil(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	before *solana.Signature, // Before this signature, exclusive (i.e. get signatures older than this signature, excluding it).
	until *solana.Signature, // Until this signature, inclusive (i.e. stop at this signature, including it).
) ([]solana.Signature, error) {
	locs, err := index.offsets.Get(context.Background(), pk)
	if err != nil {
		if offsetstore.IsNotFound(err) {
			return nil, offsetstore.ErrNotFound{PubKey: pk}
		}
		return nil, fmt.Errorf("error while getting initial offset: %w", err)
	}
	debugln("locs.OffsetToFirst:", locs)

	var sigs []solana.Signature
	next := locs.OffsetToLatest // Start from the latest, and go back in time.

	reachedBefore := false
	if before == nil {
		reachedBefore = true
	}

bigLoop:
	for {
		if next == 0 {
			break
		}
		if limit > 0 && len(sigs) >= limit {
			break
		}
		sigIndexes, newNext, err := index.ll.Read(next)
		if err != nil {
			return nil, fmt.Errorf("error while reading linked log with next=%d: %w", next, err)
		}
		debugln("sigIndexes:", sigIndexes, "newNext:", newNext)
		next = newNext
		for _, sigIndex := range sigIndexes {
			sig, err := index.sff.Get(sigIndex)
			if err != nil {
				return nil, fmt.Errorf("error while getting signature at index=%d: %w", sigIndex, err)
			}
			if !reachedBefore && sig == *before {
				reachedBefore = true
				continue
			}
			if !reachedBefore {
				continue
			}
			if limit > 0 && len(sigs) >= limit {
				break
			}
			sigs = append(sigs, sig)
			if until != nil && sig == *until {
				break bigLoop
			}
		}
	}
	return sigs, nil
}
