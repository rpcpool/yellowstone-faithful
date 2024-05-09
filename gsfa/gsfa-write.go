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
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/manifest"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/rpcpool/yellowstone-faithful/store"
	"github.com/tidwall/hashmap"
	"k8s.io/klog"
)

type GsfaWriter struct {
	mu                   sync.Mutex
	offsetsIndexDir      string
	offsets              *hashmap.Map[solana.PublicKey, [2]uint64]
	ll                   *linkedlog.LinkedLog
	man                  *manifest.Manifest
	fullBufferWriterChan chan keyWithTxLocations
	accum                *hashmap.Map[solana.PublicKey, []indexes.OffsetAndSize]
	offsetsWriter        *indexes.PubkeyToOffsetAndSize_Writer
	ctx                  context.Context
	cancel               context.CancelFunc
	fullBufferWriterDone chan struct{}
}

type keyWithTxLocations struct {
	Key       solana.PublicKey
	Locations []indexes.OffsetAndSize
}

var offsetstoreOptions = []store.Option{
	store.IndexBitSize(22),
	store.GCInterval(time.Hour),
}

// NewGsfaWriter creates or opens an existing index in WRITE mode.
func NewGsfaWriter(
	indexRootDir string,
	meta indexmeta.Meta,
	epoch uint64,
	rootCid cid.Cid,
	network indexes.Network,
	tmpDir string,
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
	ctx, cancel := context.WithCancel(context.Background())
	index := &GsfaWriter{
		fullBufferWriterChan: make(chan keyWithTxLocations, 1000), // TODO: make this configurable
		offsets:              hashmap.New[solana.PublicKey, [2]uint64](int(1_000_000)),
		accum:                hashmap.New[solana.PublicKey, []indexes.OffsetAndSize](int(10000)),
		ctx:                  ctx,
		cancel:               cancel,
		fullBufferWriterDone: make(chan struct{}),
	}
	{
		index.offsetsIndexDir = indexRootDir
	}
	{
		ll, err := linkedlog.NewLinkedLog(filepath.Join(indexRootDir, "linked-log"))
		if err != nil {
			return nil, fmt.Errorf("error while opening linked log: %w", err)
		}
		index.ll = ll
	}
	{
		man, err := manifest.NewManifest(filepath.Join(indexRootDir, "manifest"), meta)
		if err != nil {
			return nil, fmt.Errorf("error while opening manifest: %w", err)
		}
		index.man = man
	}
	{
		offsetsWriter, err := indexes.NewWriter_PubkeyToOffsetAndSize(
			epoch,
			rootCid,
			network,
			tmpDir,
		)
		if err != nil {
			return nil, fmt.Errorf("error while opening pubkey-to-offset-and-size writer: %w", err)
		}
		index.offsetsWriter = offsetsWriter
	}
	go index.fullBufferWriter()
	return index, nil
}

func (a *GsfaWriter) fullBufferWriter() {
	bufSize := 100
	tmpBuf := make(map[solana.PublicKey]keyWithTxLocations, bufSize)

	for {
		select {
		case <-a.ctx.Done():
			for _, buf := range tmpBuf {
				// Write the buffer to the linked log.
				klog.V(5).Infof("Flushing %d transactions for key %s", len(buf.Locations), buf.Key)
				// TODO: write to linked log
				if err := a.flushSingle(buf.Key, buf.Locations); err != nil {
					klog.Errorf("Error while flushing transactions for key %s: %v", buf.Key, err)
				}
			}
			a.fullBufferWriterDone <- struct{}{}
			return
		case buffer := <-a.fullBufferWriterChan:
			_, has := tmpBuf[buffer.Key]
			if len(tmpBuf) == bufSize || has {
				for _, buf := range tmpBuf {
					// Write the buffer to the linked log.
					klog.V(5).Infof("Flushing %d transactions for key %s", len(buf.Locations), buf.Key)
					// TODO: write to linked log
					if err := a.flushSingle(buf.Key, buf.Locations); err != nil {
						klog.Errorf("Error while flushing transactions for key %s: %v", buf.Key, err)
					}
				}
				tmpBuf = make(map[solana.PublicKey]keyWithTxLocations, 10)
			}
			tmpBuf[buffer.Key] = buffer
		}
	}
}

func (a *GsfaWriter) Push(
	offset uint64,
	length uint64,
	slot uint64,
	publicKeys []solana.PublicKey,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	oas := indexes.OffsetAndSize{
		Offset: offset,
		Size:   length,
	}
	for _, publicKey := range publicKeys {
		current, ok := a.accum.Get(publicKey)
		if !ok {
			current = make([]indexes.OffsetAndSize, 0, itemsPerBatch)
			current = append(current, oas)
		} else {
			current = append(current, oas)
			if len(current) >= itemsPerBatch {
				a.fullBufferWriterChan <- keyWithTxLocations{
					Key:       publicKey,
					Locations: clone(current),
				}
				clear(current)
				current = make([]indexes.OffsetAndSize, 0, itemsPerBatch)
			}
		}
		a.accum.Set(publicKey, current)
	}
	return nil
}

func clone[T any](slice []T) []T {
	s := make([]T, len(slice))
	copy(s, slice)
	return s
}

const itemsPerBatch = 1000

// Flush forces a flush of the current batch to disk.
func (a *GsfaWriter) Flush() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.flushAll(a.accum)
}

// Close closes the accumulator.
func (a *GsfaWriter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.flushAll(a.accum); err != nil {
		return err
	}
	a.cancel()
	{
		{
			keys := a.offsets.Keys()
			for _, key := range keys {
				offSize, _ := a.offsets.Get(key)
				err := a.offsetsWriter.Put(key, offSize[0], offSize[1])
				if err != nil {
					return fmt.Errorf("error while writing pubkey-to-offset-and-size: %w", err)
				}
			}
		}
		offsetsIndex := filepath.Join(a.offsetsIndexDir, string(indexes.Kind_PubkeyToOffsetAndSize)+".index")
		err := a.offsetsWriter.SealWithFilename(context.Background(), offsetsIndex)
		if err != nil {
			return fmt.Errorf("error while sealing pubkey-to-offset-and-size writer: %w", err)
		}
	}
	<-a.fullBufferWriterDone
	return errors.Join(
		a.offsetsWriter.Close(),
		a.ll.Close(),
		a.man.Close(),
	)
}

func (a *GsfaWriter) flushAll(accum *hashmap.Map[solana.PublicKey, []indexes.OffsetAndSize]) error {
	if accum.Len() == 0 {
		return nil
	}
	startedAt := time.Now()
	defer func() {
		klog.Infof(" Flushed key-to-sigs in %s.", time.Since(startedAt))
	}()

	{
		// Flush the linked log cache.
		err := a.ll.Flush()
		if err != nil {
			return fmt.Errorf("error while flushing linked log cache: %w", err)
		}
		klog.V(5).Infof("Writing %d account batches to linked log...", accum.Len())
		_, err = a.ll.Put(
			accum,
			func(pk solana.PublicKey) (uint64, error) {
				got, ok := a.offsets.Get(pk)
				if !ok {
					// This is the first time we see this account.
					// And there is no offset for the previous list.
					return 0, nil
				}
				return got[0], nil
			},
			func(pk solana.PublicKey, offset uint64, ln uint32) error {
				a.offsets.Set(pk, [2]uint64{offset, uint64(ln)})
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("error while writing account lists batch to linked log: %w", err)
		}
	}

	a.accum = hashmap.New[solana.PublicKey, []indexes.OffsetAndSize](int(10000))
	return nil
}

func (a *GsfaWriter) flushSingle(key solana.PublicKey, values []indexes.OffsetAndSize) error {
	if len(values) == 0 {
		return nil
	}
	startedAt := time.Now()
	defer func() {
		klog.V(5).Infof(" Flushed %v keys for %s in %s.", len(values), key, time.Since(startedAt))
	}()

	batch := hashmap.New[solana.PublicKey, []indexes.OffsetAndSize](1)
	batch.Set(key, values)
	{
		// Flush the linked log cache.
		err := a.ll.Flush()
		if err != nil {
			return fmt.Errorf("error while flushing linked log cache: %w", err)
		}
		_, err = a.ll.Put(
			batch,
			func(pk solana.PublicKey) (uint64, error) {
				got, ok := a.offsets.Get(pk)
				if !ok {
					// This is the first time we see this account.
					// And there is no offset for the previous list.
					return 0, nil
				}
				return got[0], nil
			},
			func(pk solana.PublicKey, offset uint64, ln uint32) error {
				a.offsets.Set(pk, [2]uint64{offset, uint64(ln)})
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("error while writing account lists batch to linked log: %w", err)
		}
	}
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
