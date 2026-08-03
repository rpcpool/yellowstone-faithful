package gsfa

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	popRank              *rollingRankOfTopPerformers // top pubkeys by flush count
	offsets              *hashmap.Map[solana.PublicKey, [2]uint64]
	ll                   *linkedlog.LinkedLog
	man                  *manifest.Manifest
	fullBufferWriterChan chan linkedlog.KeyToOffsetAndSizeAndBlocktime
	accum                *hashmap.Map[solana.PublicKey, []linkedlog.OffsetAndSizeAndSlot]
	offsetsWriter        *indexes.PubkeyToOffsetAndSize_Writer
	ctx                  context.Context
	cancel               context.CancelFunc
	exiting              *atomic.Bool
	fullBufferWriterDone chan struct{}
	// writerErr holds the first error encountered by the fullBufferWriter
	// goroutine (the sole writer to the linked log). It is written only by that
	// goroutine before it signals fullBufferWriterDone, and read by Close after
	// receiving that signal, so no additional synchronization is required.
	writerErr error
	// periodicFlushThreshold is the accumulator size above which the periodic
	// drain runs. Configurable so tests can exercise the drain path cheaply.
	periodicFlushThreshold int

	// meta
	epoch   uint64
	rootCid cid.Cid
	network indexes.Network
	tmpDir  string
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
		fullBufferWriterChan:   make(chan linkedlog.KeyToOffsetAndSizeAndBlocktime, 50), // TODO: make this configurable
		popRank:                newRollingRankOfTopPerformers(10_000),
		offsets:                hashmap.New[solana.PublicKey, [2]uint64](int(1_000_000)),
		accum:                  hashmap.New[solana.PublicKey, []linkedlog.OffsetAndSizeAndSlot](int(1_000_000)),
		ctx:                    ctx,
		cancel:                 cancel,
		fullBufferWriterDone:   make(chan struct{}),
		indexRootDir:           indexRootDir,
		exiting:                new(atomic.Bool),
		periodicFlushThreshold: 100_000,

		tmpDir: tmpDir,
		// meta
		epoch:   epoch,
		rootCid: rootCid,
		network: network,
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
	go index.fullBufferWriter()
	return index, nil
}

func (a *GsfaWriter) fullBufferWriter() {
	numReadFromChan := uint64(0)
	howManyBuffersToFlushConcurrently := 256
	// NOTE: cap, not len. Previously this was make(..., 256), i.e. a slice of 256
	// EMPTY entries, which made `len(tmpBuf) == 256` true immediately and then
	// never again — so instead of flushing every 256 buffers, tmpBuf grew until a
	// duplicate key happened to arrive.
	tmpBuf := make(linkedlog.KeyToOffsetAndSizeAndBlocktimeSlice, 0, howManyBuffersToFlushConcurrently)

	flush := func() {
		for _, buf := range tmpBuf {
			if len(buf.Values) == 0 {
				continue
			}
			// Write the buffer to the linked log.
			klog.V(5).Infof("Flushing %d transactions for key %s", len(buf.Values), buf.Key)
			if err := a.flushKVs(buf); err != nil {
				klog.Errorf("Error while flushing transactions for key %s: %v", buf.Key, err)
				if a.writerErr == nil {
					a.writerErr = err
				}
			}
		}
		tmpBuf = tmpBuf[:0]
	}

	for {
		// fmt.Println("numReadFromChan", numReadFromChan, "len(a.fullBufferWriterChan)", len(a.fullBufferWriterChan), "a.exiting.Load()", a.exiting.Load())
		if a.exiting.Load() {
			klog.Infof("remaining %d buffers to flush", len(a.fullBufferWriterChan))
		}
		if a.exiting.Load() && len(a.fullBufferWriterChan) == 0 {
			// Flush whatever is still buffered before exiting. Previously this
			// returned without flushing, silently dropping every buffer still in
			// tmpBuf (a timing-dependent set of transactions) from the index.
			flush()
			a.fullBufferWriterDone <- struct{}{}
			return // exit
		}
		select {
		case buffer := <-a.fullBufferWriterChan:
			{
				numReadFromChan++
				has := tmpBuf.Has(buffer.Key)
				if len(tmpBuf) == howManyBuffersToFlushConcurrently || has {
					flush()
				}
				tmpBuf = append(tmpBuf, buffer)
			}
		case <-time.After(1 * time.Second):
			klog.V(5).Infof("Read %d buffers from channel", numReadFromChan)
		}
	}
}

func (a *GsfaWriter) Push(
	offset uint64,
	length uint64,
	slot uint64,
	publicKeys solana.PublicKeySlice,
	// flags:
	hasMeta bool,
	isSuccess bool,
	isVote bool,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Value (not pointer): the accumulator holds these inline, so the slices are
	// pointer-free and the GC never has to scan the (very large) accumulator.
	oas := linkedlog.OffsetAndSizeAndSlot{
		Offset: offset,
		Size:   length,
		Slot:   slot,
	}
	oas.SetHasMeta(hasMeta)
	oas.SetIsSuccess(isSuccess)
	oas.SetIsVote(isVote)

	// NOTE: publicKeys must already be deduplicated by the caller. Deduplication
	// (which sorts internally) is done in the parallel decode workers rather than
	// here, because this method runs on a single consumer goroutine and profiling
	// showed the sort dominating it.
	if slot%500 == 0 && a.accum.Len() > a.periodicFlushThreshold {
		// Periodically drain the less-active keys so the in-memory accumulator
		// stays bounded.
		//
		// This used to allocate ALL keys via Keys(), sort the whole set, then
		// Get() each one back out of the map. Profiling a full-epoch run showed
		// that full sort (~33% of total CPU) and the per-key Get (~20%)
		// dominating this single consumer goroutine — even though only a small
		// unpopular subset is actually flushed while the popular keys are
		// re-sorted every time for nothing.
		//
		// Instead we iterate the map in place with Scan (key+value together, no
		// Keys() allocation, no per-key Get) and collect only the subset to drain.
		// The batches are handed to the single fullBufferWriter goroutine (see
		// below) rather than written here, so that ALL linked-log writes happen on
		// one goroutine in a deterministic order — which is what makes the gsfa
		// index byte-for-byte reproducible across runs. We still sort the drained
		// subset so the enqueue order is deterministic regardless of map iteration
		// order.
		klog.V(4).Infof("Flushing less-active keys from %d accumulated", a.accum.Len())

		a.popRank.purge()

		var toFlush []linkedlog.KeyToOffsetAndSizeAndBlocktime
		a.accum.Scan(func(key solana.PublicKey, values []linkedlog.OffsetAndSizeAndSlot) bool {
			// The objective is to have as big of a batch for each key as possible (max is 1000).
			// So we optimize for delaying the flush for the most popular keys (popular=has been flushed a lot of times).
			// And we flush the less popular keys, periodically if they haven't seen much activity.

			// if this key has less than 100 values and is not in the top list of keys by flush count, then
			// it's very likely that this key isn't going to get a lot of values soon
			if len(values) > 0 && len(values) < 100 && !a.popRank.has(key) {
				toFlush = append(toFlush, linkedlog.KeyToOffsetAndSizeAndBlocktime{
					Key:    key,
					Values: values,
				})
			}
			return true
		})

		// Enqueue in pubkey order so the write order is deterministic.
		sort.Slice(toFlush, func(i, j int) bool {
			return bytes.Compare(toFlush[i].Key[:], toFlush[j].Key[:]) < 0
		})

		for i := range toFlush {
			// Hand off to the single writer goroutine, then remove from the
			// accumulator. After the delete only the enqueued batch references the
			// values slice, so the writer can safely own it.
			a.fullBufferWriterChan <- toFlush[i]
			a.accum.Delete(toFlush[i].Key)
		}
	}
	for _, publicKey := range publicKeys {
		current, ok := a.accum.Get(publicKey)
		if !ok {
			current = make([]linkedlog.OffsetAndSizeAndSlot, 0, itemsPerBatch)
			current = append(current, oas)
			a.accum.Set(publicKey, current)
		} else {
			current = append(current, oas)
			if len(current) >= itemsPerBatch {
				a.popRank.Incr(publicKey, 1)
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
	// Enqueue the remaining accumulated batches to the single writer goroutine,
	// then signal it to drain and exit. All linked-log writes therefore happen on
	// that one goroutine, in a deterministic order.
	a.flushAccum(a.accum)
	a.exiting.Store(true)
	klog.Info("Closing linked log...")
	<-a.fullBufferWriterDone
	klog.Info("Closing full buffer writer...")
	a.cancel()
	// The writer is the sole linked-log writer; surface any error it hit.
	if a.writerErr != nil {
		return fmt.Errorf("error while writing linked log: %w", a.writerErr)
	}
	{
		{
			keys := solana.PublicKeySlice(a.offsets.Keys())
			keys.Sort()
			keys = keys.Dedupe()
			{
				offsetsWriter, err := indexes.NewWriter_PubkeyToOffsetAndSize(
					a.epoch,
					a.rootCid,
					a.network,
					a.tmpDir,
					uint(len(keys)),
				)
				if err != nil {
					return fmt.Errorf("error while opening pubkey-to-offset-and-size writer: %w", err)
				}
				a.offsetsWriter = offsetsWriter
			}
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
		a.ll.Close(),
		a.man.Close(),
	)
}

// flushAccum enqueues all remaining accumulated batches to the single writer
// goroutine, in pubkey order (deterministic). It does not write to the linked
// log directly; the writer does, so ordering stays deterministic and a.offsets
// has a single writer.
func (a *GsfaWriter) flushAccum(m *hashmap.Map[solana.PublicKey, []linkedlog.OffsetAndSizeAndSlot]) {
	keys := solana.PublicKeySlice(m.Keys())
	keys.Sort()
	for ii := range keys {
		key := keys[ii]
		vals, _ := m.Get(key)
		a.fullBufferWriterChan <- linkedlog.KeyToOffsetAndSizeAndBlocktime{
			Key:    key,
			Values: vals,
		}
		m.Delete(key)
	}
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

func debugln(args ...interface{}) {
	klog.V(6).Infoln(args...)
}
