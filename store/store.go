package store

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/rpcpool/yellowstone-faithful/store/filecache"
	"github.com/rpcpool/yellowstone-faithful/store/freelist"
	"github.com/rpcpool/yellowstone-faithful/store/index"
	"github.com/rpcpool/yellowstone-faithful/store/primary"
	"github.com/rpcpool/yellowstone-faithful/store/primary/gsfaprimary"
	"github.com/rpcpool/yellowstone-faithful/store/primary/sig2epochprimary"
	"github.com/rpcpool/yellowstone-faithful/store/types"
)

var log = logging.Logger("storethehash")

const (
	// Primary types
	GsfaPrimary       = "gsfaprimary"
	SigToEpochPrimary = "sig2epochprimary"
)

type Store struct {
	index     *index.Index
	fileCache *filecache.FileCache
	freelist  *freelist.FreeList

	stateLk sync.RWMutex
	open    bool
	running bool
	err     error

	rateLk      sync.RWMutex
	flushRate   float64 // rate at which data can be flushed
	burstRate   types.Work
	lastFlush   time.Time
	flushNotice chan struct{}

	closed       chan struct{}
	closing      chan struct{}
	flushNow     chan struct{}
	syncInterval time.Duration
	syncOnFlush  bool
	immutable    bool
}

// SetReturnErrorOnDuplicatePut sets whether to return an error when a duplicate key is
// inserted.
func (s *Store) SetReturnErrorOnDuplicatePut(yes bool) {
	s.immutable = yes
}

// OpenStore opens the index and returns a Store with the specified primary type.
//
// Calling Store.Close closes the primary and freelist.
func OpenStore(ctx context.Context, primaryType string, dataPath, indexPath string, options ...Option) (*Store, error) {
	c := config{
		fileCacheSize:   defaultFileCacheSize,
		indexSizeBits:   defaultIndexSizeBits,
		indexFileSize:   defaultIndexFileSize,
		primaryFileSize: defaultPrimaryFileSize,
		syncInterval:    defaultSyncInterval,
		burstRate:       defaultBurstRate,
		gcInterval:      defaultGCInterval,
		gcTimeLimit:     defaultGCTimeLimit,
	}
	c.apply(options)

	freeList, err := freelist.Open(indexPath + ".free")
	if err != nil {
		return nil, err
	}

	fileCache := filecache.New(c.fileCacheSize)

	var primary primary.PrimaryStorage
	switch primaryType {
	case GsfaPrimary:
		primary, err = gsfaprimary.Open(dataPath, freeList, fileCache, c.primaryFileSize)
	case SigToEpochPrimary:
		primary, err = sig2epochprimary.Open(dataPath, freeList, fileCache, c.primaryFileSize)
	default:
		err = fmt.Errorf("unsupported primary type: %s", primaryType)
	}
	if err != nil {
		freeList.Close()
		return nil, err
	}

	idx, err := index.Open(ctx, indexPath, primary, c.indexSizeBits, c.indexFileSize, c.gcInterval, c.gcTimeLimit, fileCache)
	var bitSizeError types.ErrIndexWrongBitSize
	if errors.As(err, &bitSizeError) {
		err = translateIndex(ctx, indexPath, primary, c.indexSizeBits, c.indexFileSize)
		if err != nil {
			err = fmt.Errorf("error translating index to %d bit prefix size: %w", c.indexSizeBits, err)
		} else {
			idx, err = index.Open(ctx, indexPath, primary, c.indexSizeBits, c.indexFileSize, c.gcInterval, c.gcTimeLimit, fileCache)
		}
	}
	if err != nil {
		primary.Close()
		freeList.Close()
		return nil, err
	}

	store := &Store{
		lastFlush:    time.Now(),
		index:        idx,
		fileCache:    fileCache,
		freelist:     freeList,
		open:         true,
		running:      false,
		syncInterval: c.syncInterval,
		burstRate:    c.burstRate,
		closed:       make(chan struct{}),
		closing:      make(chan struct{}),
		flushNow:     make(chan struct{}, 1),
		syncOnFlush:  c.syncOnFlush,
	}
	return store, nil
}

func translateIndex(ctx context.Context, indexPath string, primary primary.PrimaryStorage, indexSizeBits uint8, indexFileSize uint32) error {
	const progressLogInterval = 5 * time.Second

	log.Infof("Translating index to %d bit prefix", indexSizeBits)

	oldFileCache := filecache.New(64)
	log.Info("Reading old index")
	oldIndex, err := index.Open(ctx, indexPath, primary, 0, indexFileSize, 0, 0, oldFileCache)
	if err != nil {
		return fmt.Errorf("cannot open old index: %w", err)
	}
	defer oldIndex.Close()

	indexDir := filepath.Dir(indexPath)
	indexTmp, err := os.MkdirTemp(indexDir, "new_index")
	if err != nil {
		return err
	}
	defer os.RemoveAll(indexTmp)

	newIndexPath := filepath.Join(indexTmp, filepath.Base(indexPath))
	newFileCache := filecache.New(64)
	newIndex, err := index.Open(ctx, newIndexPath, primary, indexSizeBits, indexFileSize, 0, 0, newFileCache)
	if err != nil {
		return fmt.Errorf("cannot open new index: %w", err)
	}
	defer newIndex.Close()

	iter := oldIndex.NewIterator()
	ticker := time.NewTicker(progressLogInterval)
	defer ticker.Stop()
	var count int
	for {
		select {
		case <-ticker.C:
			log.Infof("Translating index records %.1f%% done", iter.Progress())
		default:
		}

		rec, done, err := iter.Next()
		if err != nil {
			return fmt.Errorf("cannot get next index record: %w", err)
		}
		if done {
			break
		}

		indexKey, err := primary.GetIndexKey(rec.Block)
		if err != nil {
			return fmt.Errorf("cannot get old index key: %w", err)
		}

		if err = newIndex.Put(indexKey, rec.Block); err != nil {
			return fmt.Errorf("cannot put new index record: %w", err)
		}

		count++
	}
	ticker.Stop()

	log.Infof("Translated %d index records", count)

	log.Info("Replacing old index files with new")
	if err = newIndex.Close(); err != nil {
		return fmt.Errorf("error closing new index: %w", err)
	}
	if err = oldIndex.Close(); err != nil {
		return fmt.Errorf("error closing old index: %w", err)
	}

	// Create a temp directory for the old index files and move them there.
	oldTmp, err := os.MkdirTemp(indexDir, "old_index")
	if err != nil {
		return err
	}
	if err = index.MoveFiles(indexPath, oldTmp); err != nil {
		return fmt.Errorf("cannot move old index files: %w", err)
	}

	// Move the new index file from the temp directory to the index directory.
	if err = index.MoveFiles(newIndexPath, indexDir); err != nil {
		return fmt.Errorf("cannot move new index files: %w", err)
	}

	// Remove the old index files.
	if err = os.RemoveAll(oldTmp); err != nil {
		return fmt.Errorf("cannot remove old index files: %w", err)
	}

	log.Infof("Finished translating index to %d bit prefix", indexSizeBits)
	return nil
}

func (s *Store) Start() {
	s.stateLk.Lock()
	running := s.running
	s.running = true
	s.stateLk.Unlock()
	if !running {
		go s.run()
	}
}

func (s *Store) Index() *index.Index {
	return s.index
}

func (s *Store) Primary() primary.PrimaryStorage {
	return s.index.Primary
}

func (s *Store) run() {
	defer close(s.closed)
	d := time.NewTicker(s.syncInterval)

	for {
		select {
		case <-s.flushNow:
			if err := s.Flush(); err != nil {
				s.setErr(err)
			}
		case <-s.closing:
			d.Stop()
			select {
			case <-d.C:
			default:
			}
			return
		case <-d.C:
			select {
			case s.flushNow <- struct{}{}:
			default:
				// Already signaled by write, do not need another flush.
			}
		}
	}
}

// Close stops store goroutines and calls Close on the index, primary, and
// freelist. This flushes any outstanding work and buffered data to their
// files.
func (s *Store) Close() error {
	s.stateLk.Lock()
	if !s.open {
		s.stateLk.Unlock()
		return nil
	}
	s.open = false

	running := s.running
	s.running = false
	s.stateLk.Unlock()

	if running {
		close(s.closing)
		<-s.closed
	}

	cerr := s.Err()

	err := s.index.Close()
	if err != nil {
		cerr = err
	}
	if err = s.index.Primary.Close(); err != nil {
		cerr = err
	}
	s.fileCache.Clear()
	if err = s.freelist.Close(); err != nil {
		cerr = err
	}

	return cerr
}

func (s *Store) Get(key []byte) ([]byte, bool, error) {
	err := s.Err()
	if err != nil {
		return nil, false, err
	}

	indexKey, err := s.index.Primary.IndexKey(key)
	if err != nil {
		return nil, false, err
	}
	fileOffset, found, err := s.index.Get(indexKey)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	primaryKey, value, err := s.getPrimaryKeyData(fileOffset, indexKey)
	if err != nil {
		return nil, false, err
	}
	if primaryKey == nil {
		return nil, false, nil
	}

	return value, true, nil
}

func (s *Store) Err() error {
	s.stateLk.RLock()
	defer s.stateLk.RUnlock()
	return s.err
}

func (s *Store) setErr(err error) {
	s.stateLk.Lock()
	s.err = err
	s.stateLk.Unlock()
}

type ErrDuplicate struct {
	Key         []byte
	StoredKey   []byte
	Value       []byte
	StoredValue []byte
}

func (e *ErrDuplicate) Error() string {
	return fmt.Sprintf("duplicate key: %x", e.Key)
}

// Is returns true if the error is an ErrDuplicate.
func (e *ErrDuplicate) Is(err error) bool {
	_, ok := err.(*ErrDuplicate)
	return ok
}

func (s *Store) Put(key []byte, newValue []byte) error {
	err := s.Err()
	if err != nil {
		return err
	}

	// Get the key in primary storage
	indexKey, err := s.index.Primary.IndexKey(key)
	if err != nil {
		return err
	}
	// TODO: (immaterial) What if we just used the `prevOffset` (which is `types.Block`) and just added the extra field `LastOffset` to it?
	// See if the key already exists and get offset
	prevOffset, found, err := s.index.Get(indexKey)
	if err != nil {
		return err
	}
	// If found, get the key and value stored in primary to see if it is the
	// same (index only stores prefixes).
	var storedKey []byte
	var storedVal []byte
	var cmpKey bool
	if found {
		storedKey, storedVal, err = s.getPrimaryKeyData(prevOffset, indexKey)
		if err != nil {
			return err
		}
		// We need to compare to the resulting indexKey to the storedKey. Two
		// keys may point to same IndexKey (i.e. two CIDS same multihash), and
		// they need to be treated as the same key.
		if storedKey != nil {
			// if we're not accepting updates, this is the point we bail --
			// the identical key is in primary storage, we don't do update operations

			if s.immutable {
				return &ErrDuplicate{
					Key:         key,
					StoredKey:   storedKey,
					Value:       newValue,
					StoredValue: storedVal,
				}
			}
			cmpKey = true
		}
		// TODO: the key-value that we got here might be from the cache of primary storage,
		// and this means that it could be outdated if another direct write happened to primary storage.
		if bytes.Equal(newValue, storedVal) {
			// Trying to put the same value in an existing key, so ok to
			// directly return.
			return nil
		}
		if storedKey != nil && bytes.Equal(indexKey, storedKey) {
			// overwrite in primary storage:
			err = s.index.Primary.Overwrite(prevOffset, key, newValue)
			if err != nil {
				return err
			}
			// TODO: remove?
			s.flushTick()

			return nil
		}
	}

	// We are ready now to start putting/updating the value in the key.
	// Put value in primary storage first. In primary storage we put
	// the key, not the indexKey. The storage knows how to manage the key
	// under the hood while the index is primary storage-agnostic.
	fileOffset, err := s.index.Primary.Put(key, newValue)
	if err != nil {
		return err
	}

	// If the key being set is not found, or the stored key is not equal
	// (even if same prefix is shared @index), we put the key without updates
	if !cmpKey {
		if err = s.index.Put(indexKey, fileOffset); err != nil {
			return err
		}
	} else {
		// If the key exists and the one stored is the one we are trying
		// to put this is an update.
		// if found && bytes.Compare(key, storedKey) == 0 {
		if err = s.index.Update(indexKey, fileOffset); err != nil {
			return err
		}
		// Add outdated data in primary storage to freelist
		if err = s.freelist.Put(prevOffset); err != nil {
			return err
		}
	}

	s.flushTick()

	return nil
}

func (s *Store) Remove(key []byte) (bool, error) {
	err := s.Err()
	if err != nil {
		return false, err
	}

	// Get the key in primary storage
	indexKey, err := s.index.Primary.IndexKey(key)
	if err != nil {
		return false, err
	}
	// See if the key already exists and get offset
	offset, found, err := s.index.Get(indexKey)
	if err != nil {
		return false, err
	}

	// If not found it means there's nothing to remove.
	// Return false with no error
	if !found {
		return false, nil
	}

	// If found, get the key and value stored in primary to see if it is the
	// same (index only stores prefixes).
	storedKey, _, err := s.getPrimaryKeyData(offset, indexKey)
	if err != nil {
		return false, err
	}
	if storedKey == nil {
		// The indexKey does not exist and there is nothing to remove.
		return false, nil
	}

	removed, err := s.index.Remove(storedKey)
	if err != nil {
		return false, err
	}
	if removed {
		// Mark slot in freelist
		err = s.freelist.Put(offset)
		if err != nil {
			return false, err
		}
	}

	s.flushTick()
	return removed, nil
}

func (s *Store) SetFileCacheSize(size int) {
	s.fileCache.SetCacheSize(size)
}

func (s *Store) getPrimaryKeyData(blk types.Block, indexKey []byte) ([]byte, []byte, error) {
	// Get the key and value stored in primary to see if it is the same (index
	// only stores prefixes).
	storedKey, storedValue, err := s.index.Primary.Get(blk)
	if err != nil {
		// Log the error reading the primary, since no error is returned if the
		// bad index is successfully deleted.
		log.Errorw("Error reading primary, removing bad index", "err", err)
		// The offset returned from the index is not usable, so delete the
		// index entry regardless of which key in indexes. It is not safe to
		// put this offset onto the free list, since it may be an invalid
		// location in the primary.
		if _, err = s.index.Remove(indexKey); err != nil {
			return nil, nil, fmt.Errorf("error removing unusable index: %w", err)
		}
		return nil, nil, nil
	}

	// Check that the stored key is the correct type.
	storedKey, err = s.index.Primary.IndexKey(storedKey)
	if err != nil {
		// The key read from the primary is bad. This means that the data
		// stored in the primary is bad or the index has an incorrect location.
		// Either way, the index is unusable, so log the error and delete the
		// index. It is not safe to put this offset onto the free list, since
		// it may be an invalid location in the primary.
		log.Errorw("Bad key stored in primary or bad index, removing index", "err", err)
		if _, err = s.index.Remove(indexKey); err != nil {
			return nil, nil, fmt.Errorf("error removing unusable index: %w", err)
		}
		return nil, nil, nil
	}

	// The index stores only prefixes, hence check if the given key fully
	// matches the key that is stored in the primary storage before returning
	// the actual value. If given key and stored key do not match, then some
	// other key that has the same prefix was stored.
	if !bytes.Equal(indexKey, storedKey) {
		return nil, nil, nil
	}

	return storedKey, storedValue, nil
}

func (s *Store) flushTick() {
	s.rateLk.Lock()
	flushRate := s.flushRate
	lastFlush := s.lastFlush
	s.rateLk.Unlock()

	if flushRate == 0 {
		// Do not know the flush rate yet.
		return
	}

	work := s.index.OutstandingWork() + s.index.Primary.OutstandingWork() + s.freelist.OutstandingWork()
	if work <= s.burstRate {
		// Not enough work to be concerned.
		return
	}

	// Calculate inbound rate of data.
	elapsed := time.Since(lastFlush)
	inRate := math.Ceil(float64(work) / elapsed.Seconds())

	// If the rate of incoming work exceeds the rate that work can be flushed
	// at, and there is enough work to be concerned about (work > s.burstRate),
	// then trigger an immediate flush and wait for the flush to complete. It
	// is necessary to wait for the flush, otherwise more work could continue
	// to come in and be stored in memory faster that flushes could handle it,
	// leading to memory exhaustion.
	if inRate > flushRate {
		// Get a channel that broadcasts next flush completion.
		s.rateLk.Lock()
		if s.flushNotice == nil {
			s.flushNotice = make(chan struct{})
		}
		flushNotice := s.flushNotice
		s.rateLk.Unlock()

		// Trigger flush now, non-blocking.
		select {
		case s.flushNow <- struct{}{}:
			log.Debugw("Work ingress rate exceeded flush rate, waiting for flush", "inRate", inRate, "flushRate", s.flushRate, "elapsed", elapsed, "work", work, "burstRate", s.burstRate)
		default:
			// Already signaled, but flush not yet started. No need to wait
			// since the existing unread signal guarantees the a flush.
		}

		// Wait for next flush to complete.
		<-flushNotice
	}
}

func (s *Store) commit() (types.Work, error) {
	primaryWork, err := s.index.Primary.Flush()
	if err != nil {
		return 0, err
	}
	indexWork, err := s.index.Flush()
	if err != nil {
		return 0, err
	}
	flWork, err := s.freelist.Flush()
	if err != nil {
		return 0, err
	}
	if s.syncOnFlush {
		// finalize disk writes
		if err = s.index.Primary.Sync(); err != nil {
			return 0, err
		}
		if err = s.index.Sync(); err != nil {
			return 0, err
		}
		if err = s.freelist.Sync(); err != nil {
			return 0, err
		}
	}
	return primaryWork + indexWork + flWork, nil
}

func (s *Store) outstandingWork() bool {
	return s.index.OutstandingWork()+s.index.Primary.OutstandingWork() > 0
}

// Flush writes outstanding work and buffered data to the primary, index, and
// freelist files. It then syncs these files to permanent storage.
func (s *Store) Flush() error {
	lastFlush := time.Now()

	s.rateLk.Lock()
	s.lastFlush = lastFlush
	s.rateLk.Unlock()

	if !s.outstandingWork() {
		return nil
	}

	work, err := s.commit()
	if err != nil {
		return err
	}

	var rate float64
	if work > types.Work(s.burstRate) {
		now := time.Now()
		elapsed := now.Sub(lastFlush)
		rate = math.Ceil(float64(work) / elapsed.Seconds())
	}

	s.rateLk.Lock()
	if rate != 0 {
		s.flushRate = rate
	}
	if s.flushNotice != nil {
		close(s.flushNotice)
		s.flushNotice = nil
	}
	s.rateLk.Unlock()

	return nil
}

func (s *Store) Has(key []byte) (bool, error) {
	err := s.Err()
	if err != nil {
		return false, err
	}
	indexKey, err := s.index.Primary.IndexKey(key)
	if err != nil {
		return false, err
	}
	blk, found, err := s.index.Get(indexKey)
	if !found || err != nil {
		return false, err
	}

	// The index stores only prefixes, hence check if the given key fully matches the
	// key that is stored in the primary storage before returning the actual value.
	// TODO: avoid second lookup
	primaryIndexKey, err := s.index.Primary.GetIndexKey(blk)
	if err != nil {
		return false, err
	}

	return bytes.Equal(indexKey, primaryIndexKey), nil
}

func (s *Store) GetSize(key []byte) (types.Size, bool, error) {
	indexKey, err := s.index.Primary.IndexKey(key)
	if err != nil {
		return 0, false, err
	}
	blk, found, err := s.index.Get(indexKey)
	if err != nil {
		return 0, false, err
	}
	if !found {
		return 0, false, nil
	}

	// The index stores only prefixes, hence check if the given key fully matches the
	// key that is stored in the primary storage before returning the actual value.
	// TODO: avoid second lookup
	primaryIndexKey, err := s.index.Primary.GetIndexKey(blk)
	if err != nil {
		return 0, false, err
	}

	if !bytes.Equal(indexKey, primaryIndexKey) {
		return 0, false, nil
	}
	return blk.Size - types.Size(len(key)), true, nil
}

// IndexStorageSize returns the storage used by the index files.
func (s *Store) IndexStorageSize() (int64, error) {
	return s.index.StorageSize()
}

// PrimaryStorageSize returns the storage used by the primary storage files.
func (s *Store) PrimaryStorageSize() (int64, error) {
	return s.index.Primary.StorageSize()
}

// FreelistStorageSize returns the storage used by the freelist files.
func (s *Store) FreelistStorageSize() (int64, error) {
	return s.freelist.StorageSize()
}

// StorageSize returns the storage used by the index, primary, and freelist files.
func (s *Store) StorageSize() (int64, error) {
	isize, err := s.index.StorageSize()
	if err != nil {
		return 0, err
	}
	psize, err := s.index.Primary.StorageSize()
	if err != nil {
		return 0, err
	}
	fsize, err := s.freelist.StorageSize()
	if err != nil {
		return 0, err
	}
	return isize + psize + fsize, nil
}
