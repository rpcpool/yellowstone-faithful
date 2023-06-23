package gsfa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	"github.com/rpcpool/yellowstone-faithful/gsfa/sff"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store"
	"k8s.io/klog"
)

type GsfaWriter struct {
	sff                       *sff.SignaturesFlatFile
	batch                     map[solana.PublicKey][]uint64
	numCurrentBatchSignatures uint64
	optAutoflushAtNumSigs     uint64
	mu                        sync.Mutex
	offsets                   *offsetstore.OffsetStore
	ll                        *linkedlog.LinkedLog
}

// NewGsfaWriter creates or opens an existing index in WRITE mode.
func NewGsfaWriter(
	indexRootDir string,
	flushEveryXSigs uint64,
) (*GsfaWriter, error) {
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
	if flushEveryXSigs == 0 {
		return nil, fmt.Errorf("flushAt must be greater than 0")
	}
	index := &GsfaWriter{
		batch:                 make(map[solana.PublicKey][]uint64),
		optAutoflushAtNumSigs: flushEveryXSigs,
	}
	{
		offsetsIndexDir := filepath.Join(indexRootDir, "offsets-index")
		if err := os.MkdirAll(offsetsIndexDir, 0o755); err != nil {
			return nil, err
		}
		offsets, err := offsetstore.OpenOffsetStore(
			context.Background(),
			filepath.Join(offsetsIndexDir, "index"),
			filepath.Join(offsetsIndexDir, "data"),
			store.IndexBitSize(22), // NOTE: if you don't specify this, the final size is smaller.
			store.GCInterval(time.Hour),
		)
		if err != nil {
			return nil, fmt.Errorf("error while opening offset index: %w", err)
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

func (a *GsfaWriter) Push(signature solana.Signature, publicKeys []solana.PublicKey) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.numCurrentBatchSignatures >= a.optAutoflushAtNumSigs {
		if err := a.flush(); err != nil {
			return fmt.Errorf("error while flushing current batch: %w", err)
		}
	}
	index, err := a.sff.Put(signature)
	for _, publicKey := range publicKeys {
		a.batch[publicKey] = append(a.batch[publicKey], index)
	}
	a.numCurrentBatchSignatures++
	return err
}

// Flush forces a flush of the current batch to disk.
func (a *GsfaWriter) Flush() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.flush()
}

// Close closes the accumulator.
func (a *GsfaWriter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.flush(); err != nil {
		return err
	}
	return errors.Join(
		a.sff.Close(),
		a.offsets.Close(),
		a.ll.Close(),
	)
}

func (a *GsfaWriter) flush() error {
	klog.Infof("Flushing %d key-to-sigs...", len(a.batch))
	startedAt := time.Now()
	defer func() {
		klog.Infof(" Flushed key-to-sigs in %s.", time.Since(startedAt))
	}()

	if err := a.sff.Flush(); err != nil {
		return err
	}
	if len(a.batch) == 0 {
		return nil
	}
	// Flush the offsets store.
	err := a.offsets.Flush()
	if err != nil {
		return fmt.Errorf("error while flushing account store: %w", err)
	}
	{
		// Flush the linked log cache.
		err = a.ll.Flush()
		if err != nil {
			return fmt.Errorf("error while flushing linked log cache: %w", err)
		}
		debugf("Writing %d account batches to linked log...", len(a.batch))
		err := a.ll.Write(
			a.batch,
			func(pk solana.PublicKey, offset uint64, ln uint32) error {
				startOfLast8Bytes := offset + uint64(ln) - 8

				got, err := a.offsets.Get(context.Background(), pk)
				if err == nil {
					debugf(
						"Offsets for %s already exists, overwriting `next` of previous with %d...",
						pk,
						offset,
					)
					// overwrite the next offset of the previous batch of this pubkey:
					err = a.ll.OverwriteNextOffset_NoMutex(got.OffsetToLastNext, offset)
					if err != nil {
						return fmt.Errorf("error while overwriting next offset: %w", err)
					}
				} else {
					if !offsetstore.IsNotFound(err) {
						return fmt.Errorf("error while getting account: %w", err)
					}
				}

				return a.offsets.Put(
					context.Background(),
					pk,
					offsetstore.Locs{
						OffsetToFirst:    offset, // in case this is the first time we see this account.
						OffsetToLastNext: startOfLast8Bytes,
					})
			},
		)
		if err != nil {
			return err
		}
	}
	a.batch = make(map[solana.PublicKey][]uint64)
	a.numCurrentBatchSignatures = 0
	return nil
}

var enableDebug = false

func debugf(format string, args ...interface{}) {
	if enableDebug {
		klog.Infof(format, args...)
	}
}

func debugln(args ...interface{}) {
	if enableDebug {
		klog.Infoln(args...)
	}
}

func debugln_(c func() []any) {
	if enableDebug {
		klog.Infoln(c()...)
	}
}
