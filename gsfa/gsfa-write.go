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
	"github.com/rpcpool/yellowstone-faithful/gsfa/manifest"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
	"github.com/rpcpool/yellowstone-faithful/gsfa/sff"
	"github.com/rpcpool/yellowstone-faithful/store"
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
	man                       *manifest.Manifest
	lastSlot                  uint64
	firstSlotOfCurrentBatch   uint64
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
		offsets, err := offsetstore.Open(
			context.Background(),
			filepath.Join(offsetsIndexDir, "index"),
			filepath.Join(offsetsIndexDir, "data"),
			store.IndexBitSize(22),
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
	{
		man, err := manifest.NewManifest(filepath.Join(indexRootDir, "manifest"))
		if err != nil {
			return nil, err
		}
		index.man = man
	}
	return index, nil
}

func (a *GsfaWriter) Push(slot uint64, signature solana.Signature, publicKeys []solana.PublicKey) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.numCurrentBatchSignatures >= a.optAutoflushAtNumSigs && slot != a.lastSlot {
		// Flush the current batch. Only flush if the slot is different from the last one.
		// This is to avoid flushing mid-slot.
		if err := a.flush(); err != nil {
			return fmt.Errorf("error while flushing current batch: %w", err)
		}
		a.firstSlotOfCurrentBatch = slot
	}
	index, err := a.sff.Put(signature)
	for _, publicKey := range publicKeys {
		a.batch[publicKey] = append(a.batch[publicKey], index)
	}
	a.numCurrentBatchSignatures++
	a.lastSlot = slot
	if a.firstSlotOfCurrentBatch == 0 {
		a.firstSlotOfCurrentBatch = slot
	}
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
		a.man.Close(),
	)
}

func (a *GsfaWriter) flush() error {
	if err := a.sff.Flush(); err != nil {
		return err
	}
	if len(a.batch) == 0 {
		return nil
	}
	klog.Infof("Flushing %d key-to-sigs...", len(a.batch))
	startedAt := time.Now()
	defer func() {
		klog.Infof(" Flushed key-to-sigs in %s.", time.Since(startedAt))
	}()

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
		startOffset, err := a.ll.Put(
			a.batch,
			func(pk solana.PublicKey) (uint64, error) {
				got, err := a.offsets.Get(context.Background(), pk)
				if err != nil {
					if offsetstore.IsNotFound(err) {
						// This is the first time we see this account.
						// And there is no offset for the previous list.
						return 0, nil
					} else {
						return 0, fmt.Errorf("error while getting account: %w", err)
					}
				}
				return got.OffsetToLatest, nil
			},
			func(pk solana.PublicKey, offset uint64, ln uint32) error {
				return a.offsets.Put(
					context.Background(),
					pk,
					offsetstore.Locs{
						OffsetToLatest: offset, // in case this is the first time we see this account.
					})
			},
		)
		if err != nil {
			return fmt.Errorf("error while writing account lists batch to linked log: %w", err)
		}
		// Maps first slot of the batch to the offset of the batch in the linked log.
		err = a.man.Put(a.firstSlotOfCurrentBatch, startOffset)
		if err != nil {
			return fmt.Errorf("error while writing entry to manifest: %w", err)
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
