package rangecache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type RangeCache struct {
	mu sync.RWMutex
	// the size of the file.
	size int64
	name string

	occupiedSpace uint64
	remoteFetcher func(p []byte, off int64) (n int, err error)

	cache map[Range]RangeCacheEntry
}

type RangeCacheEntry struct {
	Value    []byte
	LastRead time.Time
}

type Range [2]int64 // [start, end)

// contains returns true if the given range is contained in this range.
func (r Range) contains(r2 Range) bool {
	return r[0] <= r2[0] && r[1] >= r2[1]
}

func (r Range) isContainedIn(r2 Range) bool {
	return r2.contains(r)
}

func (r Range) isValidFor(size int64) bool {
	return r[0] >= 0 && r[1] <= size && r[0] <= r[1]
}

// NewRangeCache creates a new RangeCache.
func NewRangeCache(
	size int64,
	name string,
	fetcher func(p []byte, off int64) (n int, err error),
) *RangeCache {
	if fetcher == nil {
		panic("fetcher must not be nil")
	}
	return &RangeCache{
		size:          size,
		name:          name,
		cache:         make(map[Range]RangeCacheEntry),
		remoteFetcher: fetcher,
	}
}

func (rc *RangeCache) Size() int64 {
	return rc.size
}

func (rc *RangeCache) OccupiedSpace() uint64 {
	return rc.occupiedSpace
}

func (rc *RangeCache) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = nil
	return nil
}

// StartCacheGC starts a goroutine that will delete old cache entries.
func (rc *RangeCache) StartCacheGC(ctx context.Context, maxAge time.Duration) {
	go func() {
		t := time.NewTicker(maxAge)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				rc.DeleteOldEntries(ctx, maxAge)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (rc *RangeCache) DeleteOldEntries(ctx context.Context, maxAge time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for r, e := range rc.cache {
		if ctx.Err() != nil {
			return
		}
		if time.Since(e.LastRead) > maxAge {
			delete(rc.cache, r)
			rc.occupiedSpace -= uint64(len(e.Value))
		}
	}
}

// SetRange sets the range [start, end) to the given value.
func (rc *RangeCache) SetRange(ctx context.Context, start, ln int64, value []byte) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.setRange(ctx, start, ln, value)
}

func (rc *RangeCache) setRange(ctx context.Context, start, ln int64, value []byte) error {
	end := start + ln
	if start < 0 || end > rc.size || start > end {
		return fmt.Errorf("invalid range: [%d, %d)", start, end)
	}
	if len(value) != int(end-start) {
		return fmt.Errorf("invalid value length: %d", len(value))
	}
	{
		for r, rv := range rc.cache {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// check if one of the ranges in the cache contains the requested range.
			if r.contains(Range{start, end}) {
				klog.V(5).Infof("there's already a cache entry for this or a superset of this range: %v", r)
				return nil
			}
			// check if the requested range contains one of the ranges in the cache.
			if (Range{start, end}).contains(r) {
				klog.V(5).Infof("deleting a subset of this range: %v", r)
				delete(rc.cache, r)
				rc.occupiedSpace -= uint64(len(rv.Value))
			}
		}
	}
	rc.cache[Range{start, end}] = RangeCacheEntry{
		Value:    value,
		LastRead: time.Now(),
	}
	rc.occupiedSpace += uint64(len(value))
	return nil
}

// GetRange gets the range [start, end) from the given reader.
func (rc *RangeCache) GetRange(ctx context.Context, start, ln int64) ([]byte, error) {
	end := start + ln
	got, err := rc.getRange(ctx, start, end, func() ([]byte, error) {
		v := make([]byte, end-start)
		klog.V(5).Infof(
			orange("[cache-MISS] reading from source %s: start=%d end=%d len=%d\n"),
			rc.name,
			start,
			end,
			end-start,
		)
		_, err := rc.remoteFetcher(v, start)
		if err == nil {
			cloned := clone(v)
			rc.setRange(ctx, start, ln, cloned)
		}
		return v, err
	})
	if err != nil {
		return nil, err
	}
	if len(got) != int(end-start) {
		return nil, fmt.Errorf("invalid length: %d", len(got))
	}
	return got, nil
}

func orange(s string) string {
	return "\033[38;5;208m" + s + "\033[0m"
}

func lime(s string) string {
	return "\033[38;5;118m" + s + "\033[0m"
}

func (rc *RangeCache) getRange(ctx context.Context, start, end int64, miss func() ([]byte, error)) ([]byte, error) {
	if start < 0 || end > rc.size || start > end {
		return nil, fmt.Errorf("invalid range: [%d, %d)", start, end)
	}
	if end-start > rc.size {
		return nil, fmt.Errorf("range too large: %d", end-start)
	}
	v, ok, err := rc.getRangeFromCache(ctx, start, end)
	if err != nil {
		return nil, err
	}
	if ok {
		return v, nil
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return miss()
}

// getRangeFromCache gets the range [start, end) from the cache.
// It will look for an exact match first, then for a superset of the requested range.
func (rc *RangeCache) getRangeFromCache(ctx context.Context, start, end int64) ([]byte, bool, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if len(rc.cache) == 0 {
		return nil, false, nil
	}
	if v, ok := rc.cache[Range{start, end}]; ok {
		klog.V(5).Infof(
			lime("[exact-cache-HIT] for %s: start=%d end=%d len=%d\n"),
			rc.name,
			start,
			end,
			end-start,
		)
		return clone(v.Value), true, nil
	}
	{
		// check if we have a cache entry that is a superset of the requested range.
		for r := range rc.cache {
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}
			if r.contains(Range{start, end}) {
				klog.V(5).Infof(
					lime("[cache-HIT] range superset in %s: start=%d end=%d len=%d\n"),
					rc.name,
					start,
					end,
					end-start,
				)
				return clone(rc.cache[r].Value[start-r[0] : end-r[0]]), true, nil
			}
		}
	}
	return nil, false, nil
}

func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// ReaderAtSeeker is the interface that groups the basic ReadAt and Seek methods.
type ReaderAtSeeker interface {
	ReadAt(p []byte, off int64) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}
