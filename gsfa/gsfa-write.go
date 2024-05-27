package gsfa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/gsfa/manifest"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/tidwall/hashmap"
	"k8s.io/klog/v2"
)

type GsfaWriter struct {
	mu                   sync.Mutex
	indexRootDir         string
	offsets              *hashmap.Map[solana.PublicKey, [2]uint64]
	ll                   *linkedlog.LinkedLog
	man                  *manifest.Manifest
	fullBufferWriterChan chan linkedlog.KeyToOffsetAndSizeAndBlocktime
	accum                *hashmap.Map[solana.PublicKey, []*linkedlog.OffsetAndSizeAndBlocktime]
	offsetsWriter        *indexes.PubkeyToOffsetAndSize_Writer
	ctx                  context.Context
	cancel               context.CancelFunc
	exiting              *atomic.Bool
	fullBufferWriterDone chan struct{}
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
		fullBufferWriterChan: make(chan linkedlog.KeyToOffsetAndSizeAndBlocktime, 50), // TODO: make this configurable
		offsets:              hashmap.New[solana.PublicKey, [2]uint64](int(1_000_000)),
		accum:                hashmap.New[solana.PublicKey, []*linkedlog.OffsetAndSizeAndBlocktime](int(1_000_000)),
		ctx:                  ctx,
		cancel:               cancel,
		fullBufferWriterDone: make(chan struct{}),
		indexRootDir:         indexRootDir,
		exiting:              new(atomic.Bool),
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
	numReadFromChan := uint64(0)
	howManyBuffersToFlushConcurrently := 256
	tmpBuf := make(linkedlog.KeyToOffsetAndSizeAndBlocktimeSlice, howManyBuffersToFlushConcurrently)

	for {
		// fmt.Println("numReadFromChan", numReadFromChan, "len(a.fullBufferWriterChan)", len(a.fullBufferWriterChan), "a.exiting.Load()", a.exiting.Load())
		if a.exiting.Load() {
			klog.Infof("remaining %d buffers to flush", len(a.fullBufferWriterChan))
		}
		if a.exiting.Load() && len(a.fullBufferWriterChan) == 0 {
			a.fullBufferWriterDone <- struct{}{}
			return // exit
		}
		select {
		case buffer := <-a.fullBufferWriterChan:
			{
				numReadFromChan++
				has := tmpBuf.Has(buffer.Key)
				if len(tmpBuf) == howManyBuffersToFlushConcurrently || has {
					for _, buf := range tmpBuf {
						// Write the buffer to the linked log.
						klog.V(5).Infof("Flushing %d transactions for key %s", len(buf.Values), buf.Key)
						if err := a.flushKVs(buf); err != nil {
							klog.Errorf("Error while flushing transactions for key %s: %v", buf.Key, err)
						}
					}
					tmpBuf = make(linkedlog.KeyToOffsetAndSizeAndBlocktimeSlice, howManyBuffersToFlushConcurrently)
				}
				tmpBuf = append(tmpBuf, buffer)
			}
		case <-time.After(1 * time.Second):
			klog.Infof("Read %d buffers from channel", numReadFromChan)
		}
	}
}

func (a *GsfaWriter) Push(
	offset uint64,
	length uint64,
	slot uint64,
	blocktime uint64,
	publicKeys solana.PublicKeySlice,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	oas := &linkedlog.OffsetAndSizeAndBlocktime{
		Offset:    offset,
		Size:      length,
		Blocktime: blocktime,
	}
	publicKeys = publicKeys.Dedupe()
	publicKeys.Sort()
	if slot%1000 == 0 {
		if a.accum.Len() > 130_000 {
			// flush all
			klog.Infof("Flushing all %d keys", a.accum.Len())

			var keys solana.PublicKeySlice = a.accum.Keys()
			keys.Sort()

			for iii := range keys {
				key := keys[iii]
				values, _ := a.accum.Get(key)

				if len(values) < 100 && len(values) > 0 {
					if err := a.flushKVs(linkedlog.KeyToOffsetAndSizeAndBlocktime{
						Key:    key,
						Values: values,
					}); err != nil {
						return err
					}
					a.accum.Delete(key)
				}
			}
		}
	}
	for _, publicKey := range publicKeys {
		current, ok := a.accum.Get(publicKey)
		if !ok {
			current = make([]*linkedlog.OffsetAndSizeAndBlocktime, 0)
			current = append(current, oas)
			a.accum.Set(publicKey, current)
		} else {
			current = append(current, oas)
			if len(current) >= itemsPerBatch {
				a.fullBufferWriterChan <- linkedlog.KeyToOffsetAndSizeAndBlocktime{
					Key:    publicKey,
					Values: clone(current),
				}
				clear(current)
				a.accum.Delete(publicKey)
			} else {
				a.accum.Set(publicKey, current)
			}
		}
	}
	return nil
}

func clone[T any](slice []T) []T {
	s := make([]T, len(slice))
	copy(s, slice)
	return s
}

const itemsPerBatch = 1000

// Close closes the accumulator.
func (a *GsfaWriter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.flushAccum(a.accum); err != nil {
		return err
	}
	a.exiting.Store(true)
	klog.Info("Closing linked log...")
	<-a.fullBufferWriterDone
	klog.Info("Closing full buffer writer...")
	a.cancel()
	{
		{
			keys := solana.PublicKeySlice(a.offsets.Keys())
			keys.Sort()
			klog.Infof("Writing %d starting offsets for as many pubkeys ...", len(keys))
			for _, key := range keys {
				offSize, _ := a.offsets.Get(key)
				err := a.offsetsWriter.Put(key, offSize[0], offSize[1])
				if err != nil {
					return fmt.Errorf("error while writing pubkey-to-offset-and-size: %w", err)
				}
			}
		}
		offsetsIndex := filepath.Join(a.indexRootDir, string(indexes.Kind_PubkeyToOffsetAndSize)+".index")
		klog.Info("Sealing pubkey-to-offset-and-size writer...")
		err := a.offsetsWriter.SealWithFilename(context.Background(), offsetsIndex)
		if err != nil {
			return fmt.Errorf("error while sealing pubkey-to-offset-and-size writer: %w", err)
		}
	}

	return errors.Join(
		a.offsetsWriter.Close(),
		a.ll.Close(),
		a.man.Close(),
	)
}

func (a *GsfaWriter) flushAccum(m *hashmap.Map[solana.PublicKey, []*linkedlog.OffsetAndSizeAndBlocktime]) error {
	keys := solana.PublicKeySlice(m.Keys())
	keys.Sort()
	for ii := range keys {
		key := keys[ii]
		vals, _ := m.Get(key)
		if err := a.flushKVs(linkedlog.KeyToOffsetAndSizeAndBlocktime{
			Key:    key,
			Values: vals,
		}); err != nil {
			return err
		}
		m.Delete(key)
	}
	return nil
}

func (a *GsfaWriter) flushKVs(kvs ...linkedlog.KeyToOffsetAndSizeAndBlocktime) error {
	if len(kvs) == 0 {
		return nil
	}
	startedAt := time.Now()
	defer func() {
		klog.V(5).Infof(" Flushed %d key-to-sigs in %s.", len(kvs), time.Since(startedAt))
	}()

	// Flush the linked log cache.
	// err := a.ll.Flush()
	// if err != nil {
	// 	return fmt.Errorf("error while flushing linked log cache: %w", err)
	// }
	_, err := a.ll.Put(
		func(pk solana.PublicKey) (indexes.OffsetAndSize, error) {
			got, ok := a.offsets.Get(pk)
			if !ok {
				// This is the first time we see this account.
				// And there is no offset for the previous list.
				return indexes.OffsetAndSize{}, nil
			}
			return indexes.OffsetAndSize{Offset: got[0], Size: got[1]}, nil
		},
		func(pk solana.PublicKey, offset uint64, ln uint32) error {
			a.offsets.Set(pk, [2]uint64{offset, uint64(ln)})
			return nil
		},
		kvs...,
	)
	if err != nil {
		return fmt.Errorf("error while writing account lists batch to linked log: %w", err)
	}
	return nil
}

var enableDebug = false

func debugln(args ...interface{}) {
	if enableDebug {
		klog.Infoln(args...)
	}
}
