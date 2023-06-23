package gsfa

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	"github.com/rpcpool/yellowstone-faithful/gsfa/sff"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store"
)

type GsfaReader struct {
	offsets *offsetstore.OffsetStore
	ll      *linkedlog.LinkedLog
	sff     *sff.SignaturesFlatFile
}

// NewGsfaReader opens an existing index in READ-ONLY mode.
func NewGsfaReader(indexRootDir string) (*GsfaReader, error) {
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
			return nil, offsetstore.ErrNotFound{pk}
		}
		return nil, fmt.Errorf("error while getting initial offset: %w", err)
	}
	debugln("locs.OffsetToFirst:", locs)

	var sigs []solana.Signature
	next := locs.OffsetToFirst

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
