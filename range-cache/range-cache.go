package rangecache

import (
	"container/list"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// Range defines a half-open interval [start, end).
type Range [2]int64

// contains returns true if the given range r2 is entirely contained within r.
func (r Range) contains(r2 Range) bool {
	return r[0] <= r2[0] && r[1] >= r2[1]
}

// isContainedIn returns true if this range is entirely contained within r2.
func (r Range) isContainedIn(r2 Range) bool {
	return r2.contains(r)
}

// isValidFor checks if the range is valid given a total size.
func (r Range) isValidFor(size int64) bool {
	return r[0] >= 0 && r[1] <= size && r[0] <= r[1]
}

// intersects returns true if the two ranges overlap at all.
func (r Range) intersects(r2 Range) bool {
	return r[0] < r2[1] && r[1] > r2[0]
}

// isAdjacent returns true if the two ranges are immediately next to each other.
func (r Range) isAdjacent(r2 Range) bool {
	return r[1] == r2[0] || r2[1] == r[0]
}

// union returns the smallest range encompassing both r and r2 if they are
// overlapping or adjacent. Returns (Range{}, false) if no union is possible.
func (r Range) union(r2 Range) (Range, bool) {
	if r.intersects(r2) || r.isAdjacent(r2) {
		start := min(r[0], r2[0])
		end := max(r[1], r2[1])
		return Range{start, end}, true
	}
	return Range{}, false
}

// RangeCacheEntry stores the cached value and its last access time.
type RangeCacheEntry struct {
	Value    []byte
	LastRead time.Time
}

// RangeCache manages cached byte ranges with an LRU eviction policy.
type RangeCache struct {
	mu sync.RWMutex
	// The total size of the file/data source.
	size int64
	name string

	// maxMemorySize is the maximum allowed memory usage for the cache.
	maxMemorySize int64
	// occupiedSpace is the current memory usage of the cache.
	occupiedSpace int64

	remoteFetcher func(p []byte, off int64) (n int, err error)

	// cache stores the actual range data.
	cache map[Range]RangeCacheEntry
	// lruList maintains the order of cache entries for LRU eviction.
	// Elements are Range keys. Front is MRU, back is LRU.
	lruList *list.List
	// lruMap provides quick access to list elements for moving to front.
	lruMap map[Range]*list.Element

	// Used for double-check locking in GetRange to coordinate fetches for the same range.
	// Stores map[Range]*sync.Cond. Each Cond uses rc.mu as its Locker.
	fetching sync.Map
}

// NewRangeCache creates a new RangeCache with a specified maximum memory size.
// maxMemorySize must be non-negative.
// fetcher must not be nil.
func NewRangeCache(
	size int64,
	name string,
	fetcher func(p []byte, off int64) (n int, err error),
	maxMemorySize int64,
) *RangeCache {
	if fetcher == nil {
		panic("fetcher must not be nil")
	}
	if maxMemorySize < 0 {
		panic("maxMemorySize must be non-negative")
	}
	return &RangeCache{
		size:          size,
		name:          name,
		maxMemorySize: maxMemorySize,
		cache:         make(map[Range]RangeCacheEntry),
		lruList:       list.New(),
		lruMap:        make(map[Range]*list.Element),
		remoteFetcher: fetcher,
	}
}

// Size returns the total size of the data source.
func (rc *RangeCache) Size() int64 {
	return rc.size
}

// OccupiedSpace returns the current memory occupied by the cache.
func (rc *RangeCache) OccupiedSpace() int64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.occupiedSpace
}

// Close clears the cache and releases resources.
func (rc *RangeCache) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = nil
	rc.lruList = nil
	rc.lruMap = nil
	rc.occupiedSpace = 0
	return nil
}

// StartCacheGC starts a goroutine that will delete old cache entries.
// This method provides time-based eviction, complementing the size-based LRU.
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

// DeleteOldEntries deletes cache entries older than maxAge.
// It acquires a write lock to safely modify the cache and LRU structures.
func (rc *RangeCache) DeleteOldEntries(ctx context.Context, maxAge time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	var rangesToDelete []Range
	for r, e := range rc.cache {
		if ctx.Err() != nil {
			return // Context cancelled, stop GC
		}
		if time.Since(e.LastRead) > maxAge {
			rangesToDelete = append(rangesToDelete, r)
		}
	}

	for _, r := range rangesToDelete {
		if entry, ok := rc.cache[r]; ok {
			delete(rc.cache, r)
			rc.occupiedSpace -= int64(len(entry.Value))
			if elem, ok := rc.lruMap[r]; ok {
				rc.lruList.Remove(elem)
				delete(rc.lruMap, r)
			}
			klog.V(5).Infof("GC evicted old entry for %s: %v, occupied space: %d", rc.name, r, rc.occupiedSpace)
		}
	}
}

// addEntry adds a new range entry to the cache and updates LRU.
// It assumes rc.mu is locked.
// Returns an error if the value length exceeds maxMemorySize (for a single entry).
func (rc *RangeCache) addEntry(r Range, value []byte) error {
	if len(value) == 0 { // Don't cache empty ranges
		return nil
	}
	if int64(len(value)) > rc.maxMemorySize && rc.maxMemorySize > 0 {
		return fmt.Errorf("value length %d exceeds max memory size %d for a single entry", len(value), rc.maxMemorySize)
	}

	entry := RangeCacheEntry{
		Value:    value,
		LastRead: time.Now(),
	}
	rc.cache[r] = entry
	rc.occupiedSpace += int64(len(value))

	// Add to LRU list (new entry is MRU)
	elem := rc.lruList.PushFront(r)
	rc.lruMap[r] = elem
	return nil
}

// updateLRU moves an existing entry to the front of the LRU list (MRU) and updates its LastRead time.
// It assumes rc.mu is locked.
func (rc *RangeCache) updateLRU(r Range) {
	if elem, ok := rc.lruMap[r]; ok {
		rc.lruList.MoveToFront(elem)
		// Update LastRead timestamp
		entry := rc.cache[r]
		entry.LastRead = time.Now()
		rc.cache[r] = entry
	}
}

// removeLRU removes an entry from the LRU list and map.
// It assumes rc.mu is locked.
func (rc *RangeCache) removeLRU(r Range) {
	if elem, ok := rc.lruMap[r]; ok {
		rc.lruList.Remove(elem)
		delete(rc.lruMap, r)
	}
}

// evictLRU removes least recently used entries until maxMemorySize is met.
// It assumes rc.mu is locked.
func (rc *RangeCache) evictLRU() {
	for rc.occupiedSpace > rc.maxMemorySize && rc.lruList.Len() > 0 {
		// Get the LRU element (back of the list)
		elem := rc.lruList.Back()
		r := elem.Value.(Range)

		if entry, ok := rc.cache[r]; ok {
			delete(rc.cache, r)
			rc.occupiedSpace -= int64(len(entry.Value))
			rc.lruList.Remove(elem)
			delete(rc.lruMap, r)
			klog.V(5).Infof("Evicted LRU entry for %s: %v, occupied space: %d", rc.name, r, rc.occupiedSpace)
		} else {
			// This indicates an inconsistency between lruMap/lruList and cache map.
			// Remove the inconsistent element from LRU structures.
			klog.Errorf("LRU list contains key %v not found in cache map for %s. Removing from LRU.", r, rc.name)
			rc.lruList.Remove(elem)
			delete(rc.lruMap, r)
		}
	}
}

// SetRange sets the range [start, end) to the given value.
// It handles overlaps, merges, and triggers LRU eviction.
// This method is concurrency-safe.
func (rc *RangeCache) SetRange(ctx context.Context, start, ln int64, value []byte) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.setRange(ctx, start, ln, value)
}

// setRange is the internal method for setting a range, assuming rc.mu is locked.
// It handles complex overlap logic and consolidation to minimize fragmentation.
func (rc *RangeCache) setRange(ctx context.Context, start, ln int64, value []byte) error {
	end := start + ln
	newRange := Range{start, end}

	if !newRange.isValidFor(rc.size) {
		return fmt.Errorf("invalid range: [%d, %d) for size %d", start, end, rc.size)
	}
	if len(value) != int(end-start) {
		return fmt.Errorf("invalid value length: %d, expected %d", len(value), end-start)
	}

	// Use a temporary map to store all bytes that will be in the cache after this operation.
	// This allows easy merging and overwriting, prioritizing the new `value`.
	consolidatedBytes := make(map[int64]byte)

	// Add the new data first, as it takes precedence over existing cached data in overlaps.
	for i := int64(0); i < ln; i++ {
		consolidatedBytes[start+i] = value[i]
	}

	// Collect all existing ranges that intersect or are adjacent to the newRange.
	// These will be removed and their data potentially merged into `consolidatedBytes`.
	var rangesToRemove []Range
	for r, entry := range rc.cache {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if r.intersects(newRange) || r.isAdjacent(newRange) {
			rangesToRemove = append(rangesToRemove, r)
			// Add existing data to consolidatedBytes, but only for parts not overwritten by newRange.
			for i := r[0]; i < r[1]; i++ {
				if _, exists := consolidatedBytes[i]; !exists { // Only add if not already covered by newRange
					// Ensure index is within bounds of entry.Value
					if i-r[0] >= 0 && i-r[0] < int64(len(entry.Value)) {
						consolidatedBytes[i] = entry.Value[i-r[0]]
					} else {
						klog.Errorf("Internal error: Index out of bounds for existing range %v at offset %d. Data might be corrupted.", r, i)
					}
				}
			}
		}
	}

	// Remove old intersecting/adjacent ranges from cache and LRU structures.
	for _, r := range rangesToRemove {
		if entry, ok := rc.cache[r]; ok {
			delete(rc.cache, r)
			rc.occupiedSpace -= int64(len(entry.Value))
			rc.removeLRU(r) // Remove from LRU list and map
		}
	}

	// Reconstruct consolidated ranges from `consolidatedBytes`.
	// This process creates new, non-overlapping, and potentially larger ranges,
	// minimizing fragmentation.
	if len(consolidatedBytes) == 0 {
		return nil // No data to add (e.g., new range was empty or only covered already empty parts)
	}

	// Extract and sort all unique offsets from the consolidated data.
	var offsets []int64
	for off := range consolidatedBytes {
		offsets = append(offsets, off)
	}
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})

	// Iterate through sorted offsets to form contiguous segments.
	currentSegmentStart := offsets[0]
	currentSegmentEnd := offsets[0] + 1
	currentSegmentValue := []byte{consolidatedBytes[offsets[0]]}

	for i := 1; i < len(offsets); i++ {
		offset := offsets[i]
		if offset == currentSegmentEnd { // Contiguous
			currentSegmentEnd++
			currentSegmentValue = append(currentSegmentValue, consolidatedBytes[offset])
		} else { // Gap found or non-contiguous segment, add current segment and start a new one.
			addErr := rc.addEntry(Range{currentSegmentStart, currentSegmentEnd}, currentSegmentValue)
			if addErr != nil {
				return addErr // Propagate error if a consolidated segment is too large
			}
			currentSegmentStart = offset
			currentSegmentEnd = offset + 1
			currentSegmentValue = []byte{consolidatedBytes[offset]}
		}
	}
	// Add the very last segment after the loop finishes.
	addErr := rc.addEntry(Range{currentSegmentStart, currentSegmentEnd}, currentSegmentValue)
	if addErr != nil {
		return addErr
	}

	// Perform LRU eviction if needed after adding all new/merged entries.
	rc.evictLRU()

	return nil
}

// GetRange gets the range [start, end) from the cache or fetches it remotely.
// It uses a double-check locking pattern to ensure efficient and safe concurrent access.
func (rc *RangeCache) GetRange(ctx context.Context, start, ln int64) ([]byte, error) {
	end := start + ln
	requestedRange := Range{start, end}

	if !requestedRange.isValidFor(rc.size) {
		return nil, fmt.Errorf("invalid range: [%d, %d) for size %d", start, end, rc.size)
	}
	if end-start > rc.size {
		return nil, fmt.Errorf("range too large: %d", end-start)
	}

	// First check (read lock): Attempt to get the range from cache.
	hitRange, val, ok, err := rc.getRangeFromCache(ctx, start, end)
	if err != nil {
		return nil, err
	}
	if ok {
		// Cache hit, acquire write lock to update LRU and LastRead.
		rc.mu.Lock()
		defer rc.mu.Unlock()
		rc.updateLRU(hitRange) // Update LRU for the actual cached range that provided the data.
		return val, nil
	}

	// Cache miss, acquire write lock to perform fetch and update cache.
	rc.mu.Lock()
	defer rc.mu.Unlock() // Ensure mutex is always unlocked on function exit.

	// Second check (after acquiring write lock): Re-check cache.
	// Another goroutine might have filled the cache while we were waiting for the lock.
	hitRange, val, ok, err = rc.getRangeFromCacheInternal(ctx, start, end) // Internal version that doesn't acquire RLock.
	if err != nil {
		return nil, err
	}
	if ok {
		rc.updateLRU(hitRange) // Update LRU for the actual cached range.
		return val, nil
	}

	// Still a miss, proceed with remote fetch.
	// Use sync.Cond to coordinate fetches for the same range.
	// Store the condition variable in the fetching map, associated with the requested range.
	condInterface, loaded := rc.fetching.LoadOrStore(requestedRange, sync.NewCond(&rc.mu))
	c := condInterface.(*sync.Cond)

	if loaded { // Another goroutine is already fetching this range, wait for it.
		// Wait will atomically unlock rc.mu and suspend, then re-lock on wake.
		c.Wait()
		// After waiting, re-check cache as the data might have been fetched.
		hitRange, val, ok, err = rc.getRangeFromCacheInternal(ctx, start, end)
		if err != nil {
			return nil, err
		}
		if ok {
			rc.updateLRU(hitRange) // Update LRU for the actual cached range.
			return val, nil
		}
		// If we waited and still missed, it means the previous fetch failed or didn't cover this range.
		// We will now become the designated fetcher.
	}

	// If we are here, rc.mu is locked, and we are the designated fetcher for requestedRange.
	klog.V(5).Infof(
		orange("[cache-MISS] reading from source %s: start=%d end=%d len=%d\n"),
		rc.name,
		start,
		end,
		end-start,
	)

	// Unlock rc.mu during the potentially long remote fetch operation.
	rc.mu.Unlock()
	v := make([]byte, end-start)
	n, fetchErr := rc.remoteFetcher(v, start)
	rc.mu.Lock() // Re-acquire lock after fetch.

	// Clean up the condition variable and broadcast to any waiting goroutines.
	rc.fetching.Delete(requestedRange)
	c.Broadcast()

	if fetchErr != nil {
		return nil, fetchErr
	}
	if int64(n) != end-start {
		return nil, fmt.Errorf("remote fetcher returned %d bytes, expected %d", n, end-start)
	}

	// Set the fetched range in cache (this will handle overlaps and LRU).
	cloned := clone(v)
	setErr := rc.setRange(ctx, start, ln, cloned) // This call expects rc.mu to be locked.

	if setErr != nil {
		return nil, setErr
	}

	return cloned, nil
}

// orange returns a string formatted with an orange ANSI color code if klog V(5) is enabled.
func orange(s string) string {
	if klog.V(5).Enabled() {
		return "\033[38;5;208m" + s + "\033[0m"
	}
	return s
}

// lime returns a string formatted with a lime green ANSI color code if klog V(5) is enabled.
func lime(s string) string {
	if klog.V(5).Enabled() {
		return "\033[38;5;118m" + s + "\033[0m"
	}
	return s
}

// getRangeFromCache gets the range [start, end) from the cache.
// It will look for an exact match first, then for a superset of the requested range.
// It acquires an RLock and does NOT modify LastRead or LRU order.
func (rc *RangeCache) getRangeFromCache(ctx context.Context, start, end int64) (Range, []byte, bool, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.getRangeFromCacheInternal(ctx, start, end)
}

// getRangeFromCacheInternal is like getRangeFromCache but assumes rc.mu is already locked (read or write).
// It does NOT acquire any locks and does NOT modify LastRead or LRU order.
func (rc *RangeCache) getRangeFromCacheInternal(ctx context.Context, start, end int64) (Range, []byte, bool, error) {
	if len(rc.cache) == 0 {
		return Range{}, nil, false, nil
	}

	requestedRange := Range{start, end}

	// Check for exact match first
	if v, ok := rc.cache[requestedRange]; ok {
		klog.V(5).Infof(
			lime("[exact-cache-HIT] (internal) for %s: start=%d end=%d len=%d\n"),
			rc.name,
			start,
			end,
			end-start,
		)
		return requestedRange, clone(v.Value), true, nil // Return the exact range key
	}

	// Check if we have a cache entry that is a superset of the requested range.
	for r, entry := range rc.cache {
		if ctx.Err() != nil {
			return Range{}, nil, false, ctx.Err()
		}
		if r.contains(requestedRange) {
			klog.V(5).Infof(
				lime("[cache-HIT] (internal) range superset in %s: start=%d end=%d len=%d\n"),
				rc.name,
				start,
				end,
				end-start,
			)
			// Calculate the sub-slice from the superset value
			offsetInSuperset := requestedRange[0] - r[0]
			subSliceEnd := offsetInSuperset + (requestedRange[1] - requestedRange[0])
			if subSliceEnd > int64(len(entry.Value)) {
				klog.Errorf("Internal error: Sub-slice end %d out of bounds for superset value length %d. Data might be corrupted.", subSliceEnd, len(entry.Value))
				return Range{}, nil, false, fmt.Errorf("internal error: superset value too short")
			}
			return r, clone(entry.Value[offsetInSuperset:subSliceEnd]), true, nil // Return the superset range key
		}
	}
	return Range{}, nil, false, nil
}

// clone creates a deep copy of a byte slice.
func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// min helper function
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// max helper function
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
