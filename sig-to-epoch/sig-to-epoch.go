package sigtoepoch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/sig-to-epoch/epochlist"
	"github.com/rpcpool/yellowstone-faithful/sig-to-epoch/sig2epochstore"
	"github.com/rpcpool/yellowstone-faithful/store"
)

type Index struct {
	index    *sig2epochstore.Store
	epochlst *epochlist.List
}

func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// NewIndex creates or opens an existing index in WRITE mode.
func NewIndex(
	indexRootDir string,
) (*Index, error) {
	// if exists and is dir, open.
	// if exists and is not dir, error.
	// if not exists, create.
	if ok, err := isDir(indexRootDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(indexRootDir, 0o755); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if !ok {
		return nil, fmt.Errorf("provided path is not a directory: %s", indexRootDir)
	}
	index := &Index{}
	{
		indexDir := indexRootDir
		if err := os.MkdirAll(indexDir, 0o755); err != nil {
			return nil, err
		}
		indexStore, err := sig2epochstore.Open(
			context.Background(),
			filepath.Join(indexDir, "index"),
			filepath.Join(indexDir, "data"),
			store.IndexBitSize(28),
			// 26 bits,         , 30m (15  GB per 144 million signatures)
			// 28 bits,  5GB ram, 11m (15.4GB per 144 million signatures)
			// 30 bits, 19GB ram, 14m (21.4GB per 144 million signatures)
			store.GCInterval(time.Hour),
		)
		if err != nil {
			return nil, fmt.Errorf("error while opening offset index: %w", err)
		}
		index.index = indexStore
		index.index.Start()
	}
	{
		epochListFile := filepath.Join(indexRootDir, "epoch-list")
		epochlst, err := epochlist.New(epochListFile)
		if err != nil {
			return nil, fmt.Errorf("error while opening epoch list: %w", err)
		}
		index.epochlst = epochlst
	}
	return index, nil
}

// Push pushes a new entry to the index.
func (index *Index) Push(ctx context.Context, sig solana.Signature, epoch uint16) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.Join(
		index.index.Put(ctx, sig, sig2epochstore.Epoch{Epoch: epoch}),
		func() error {
			_, e := index.epochlst.HasOrPut(epoch)
			return e
		}(),
	)
}

// Get returns the epoch for the given signature.
func (index *Index) Get(ctx context.Context, sig solana.Signature) (uint16, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	epoch, err := index.index.Get(ctx, sig)
	if err != nil {
		return 0, err
	}
	return epoch.Epoch, nil
}

// Has returns true if the given signature is in the index.
func (index *Index) Has(ctx context.Context, sig solana.Signature) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return index.index.Has(ctx, sig)
}

func (index *Index) Epochs() (epochlist.Values, error) {
	list, err := index.epochlst.Load()
	if err != nil {
		return nil, fmt.Errorf("error while loading epoch list: %w", err)
	}
	return list.Unique(), nil
}

// Close closes the index.
func (index *Index) Close() error {
	return errors.Join(
		index.index.Flush(),
		index.index.Close(),
	)
}

// Flush flushes the index.
func (index *Index) Flush() error {
	return index.index.Flush()
}

func IsNotFound(err error) bool {
	return sig2epochstore.IsNotFound(err)
}
