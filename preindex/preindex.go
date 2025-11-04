// Package preindex provides a system to build a pre-index for large datasets
// where keys may be repeated. It is designed to find the *last* value (slot)
// associated with each key from a stream of (key, slot) pairs.
//
//  1. Build(): Partitions all (key, offset, index) records into temporary
//     shard files. It then sorts each shard file in-memory (by key, then index)
//     and streams the sorted data to write a final, reduced, and key-sorted
//     shard file.
//  2. IsLast(): Performs an on-disk binary search within the correct
//     shard file to find the last-seen offset for a key.
package preindex

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/cespare/xxhash/v2"
)

type Key [64]byte

// Value defines the value. Since this is intended for Solana slots, a uint32 is sufficient.
type Value uint32

// preIndexBase holds the common state for the writer and reader.
type preIndexBase struct {
	dir       string
	numShards int
}

// newPreIndexBase creates a new base instance.
func newPreIndexBase(dir string, numShards int) (*preIndexBase, error) {
	if numShards <= 0 {
		return nil, fmt.Errorf("numShards must be > 0")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create pre-index directory: %w", err)
	}

	return &preIndexBase{
		dir:       dir,
		numShards: numShards,
	}, nil
}

// getShardID calculates the shard ID for a given key.
func (b *preIndexBase) getShardID(key Key) int {
	h := xxhash.New()
	_, _ = h.Write(key[:])
	hashVal := h.Sum64()
	return int(hashVal % uint64(b.numShards))
}

// shardFileName returns the path for a temporary shard file.
func (b *preIndexBase) shardFileName(shardID int, temp bool) string {
	ext := ".dat"
	if temp {
		ext = ".tmp"
	}
	return filepath.Join(b.dir, fmt.Sprintf("shard-%04d%s", shardID, ext))
}

const (
	// defaultMaxBufferCap sets the per-shard RAM buffer to 64MB
	defaultMaxBufferCap = 64 * 1024 * 1024
	// SCALABILITY (RAM): Set a global buffer limit (e.g., 512MB)
	defaultMaxTotalBufferSize = 512 * 1024 * 1024
	// size of a tempRecord on disk (Key + Offset + Index)
	tempRecordSize = 64 + 4 + 8 // 80 bytes
	// size of a finalRecord on disk (Key + Offset)
	finalRecordSize = 64 + 4 // 68 bytes
	// size of the read buffer for slab reads
	readBufferSize = (1 * 1024 * 1024 / tempRecordSize) * tempRecordSize // 1MB aligned to 80 bytes
	// fallback capacity for 32-bit overflow
	fallbackPreallocCap = 1 << 20 // ~1M records
)

// ErrWriterClosed is returned by Push when the writer has been closed.
var ErrWriterClosed = errors.New("preindex: writer closed")

// WriterOption defines a functional option for configuring the PreIndexWriter.
type WriterOption func(*PreIndexWriter)

// WithShardBufferCap sets the per-shard RAM buffer capacity.
func WithShardBufferCap(bytes int) WriterOption {
	return func(w *PreIndexWriter) {
		if bytes > 0 {
			w.maxBufferCap = bytes
		}
	}
}

// WithTotalBufferCap sets the global RAM buffer capacity.
func WithTotalBufferCap(bytes int64) WriterOption {
	return func(w *PreIndexWriter) {
		if bytes > 0 {
			w.maxTotalBufferSize = bytes
		}
	}
}

// WithBuildWorkers sets the number of parallel workers for the Build() phase.
// NOTE: This is currently capped to 1 internally to prevent OOM
// until external merging is implemented.
func WithBuildWorkers(n int) WriterOption {
	return func(w *PreIndexWriter) {
		if n > 0 {
			w.numBuildWorkers = n
		}
	}
}

type shardBuf struct {
	mu   sync.Mutex
	wr   *bufio.Writer // Buffered writer for this shard
	file *os.File      // File handle for this shard
}

// PreIndexWriter handles building the pre-index files.
type PreIndexWriter struct {
	*preIndexBase
	shardBuffers       map[int]*shardBuf
	maxBufferCap       int
	bufferLock         sync.Mutex // Protects shardBuffers map *structure*
	index              atomic.Uint64
	maxTotalBufferSize int64
	totalBufferSize    atomic.Int64
	numBuildWorkers    int
}

// NewPreIndexWriter creates a new PreIndexWriter instance.
// 'dir' is a directory where temporary shard files will be built.
// 'numShards' controls the parallelism and memory usage during the build.
func NewPreIndexWriter(dir string, numShards int, opts ...WriterOption) (*PreIndexWriter, error) {
	base, err := newPreIndexBase(dir, numShards)
	if err != nil {
		return nil, err
	}

	w := &PreIndexWriter{
		preIndexBase:       base,
		shardBuffers:       make(map[int]*shardBuf, numShards),
		maxBufferCap:       defaultMaxBufferCap,
		maxTotalBufferSize: defaultMaxTotalBufferSize,
		numBuildWorkers:    1, // Default to 1
	}

	// Apply options
	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Push adds a single (key, offset) pair to the pre-index builder.
func (w *PreIndexWriter) Push(key Key, offset Value) error {
	w.bufferLock.Lock() // --- Lock global map
	if w.shardBuffers == nil {
		w.bufferLock.Unlock()
		return ErrWriterClosed
	}

	shardID := w.getShardID(key)

	sb, ok := w.shardBuffers[shardID]
	if !ok {
		tmpFile := w.shardFileName(shardID, true)
		// Open in append mode
		f, err := os.OpenFile(tmpFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open shard file %s for append: %w", tmpFile, err)
		}
		sb = &shardBuf{
			mu:   sync.Mutex{},
			wr:   bufio.NewWriterSize(f, w.maxBufferCap),
			file: f, // File handle for this shard
		}
		w.shardBuffers[shardID] = sb
	}
	w.bufferLock.Unlock() // --- Unlock global map

	idx := w.index.Add(1) - 1

	var slab [tempRecordSize]byte
	copy(slab[0:64], key[:])
	binary.LittleEndian.PutUint32(slab[64:68], uint32(offset))
	binary.LittleEndian.PutUint64(slab[68:76], idx)

	sb.mu.Lock()
	if _, err := sb.wr.Write(slab[:]); err != nil {
		// This should be rare (e.g., OOM in bytes.Buffer)
		sb.mu.Unlock()
		return fmt.Errorf("failed to write to in-memory buffer for shard %d: %w", shardID, err)
	}
	w.totalBufferSize.Add(tempRecordSize)
	sb.mu.Unlock()

	return nil
}

// tempRecord is the on-disk structure in the .tmp files
type tempRecord struct {
	Key    Key
	Offset Value
	Index  uint64
}

// Build finalizes the pre-index. It closes the temporary shard files,
// processes each one to find the last-seen offset for each key,
// writes the final .dat files, and cleans up.
// No more Push() calls are allowed after Build() is called.
func (w *PreIndexWriter) Build() error {
	w.bufferLock.Lock()
	if w.shardBuffers == nil {
		w.bufferLock.Unlock()
		return ErrWriterClosed
	}

	buffersToFlush := w.shardBuffers
	w.shardBuffers = nil // Mark as closed
	w.bufferLock.Unlock()

	for shardID, sb := range buffersToFlush {
		// Lock shard to exclude concurrent Push writers
		sb.mu.Lock()
		if sb.wr == nil {
			// This shard was never written to, skip it
			sb.mu.Unlock()
			continue
		}
		if err := sb.wr.Flush(); err != nil {
			sb.mu.Unlock()
			return fmt.Errorf("failed to flush buffer for shard %d: %w", shardID, err)
		}
		// Sync the file to ensure all data is written
		if err := sb.file.Sync(); err != nil {
			sb.mu.Unlock()
			if !isBestEffortError(err) {
				return fmt.Errorf("failed to sync file for shard %d: %w", shardID, err)
			}
		}
		if err := sb.file.Close(); err != nil {
			sb.mu.Unlock()
			return fmt.Errorf("failed to close file for shard %d: %w", shardID, err)
		}
		// Now we can safely remove the shard buffer
		delete(buffersToFlush, shardID)

		sb.mu.Unlock()
	}
	buffersToFlush = nil // Allow GC

	// --- Phase 2: Sort-Reduce Shards (In Parallel) ---
	// Use configured build workers, but still cap at 1 for now
	// to prevent OOM.
	numWorkers := w.numBuildWorkers
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if numWorkers > 1 {
		numWorkers = 1
	}
	if numWorkers > w.numShards {
		numWorkers = w.numShards
	}

	shardJobs := make(chan int, w.numShards)
	errCh := make(chan error, w.numShards)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for shardID := range shardJobs {
				tmpFile := w.shardFileName(shardID, true)
				datFile := w.shardFileName(shardID, false)

				if err := w.processShard(tmpFile, datFile); err != nil {
					errCh <- fmt.Errorf("failed to process shard %d: %w", shardID, err)
					return
				}

				// Clean up the temporary file
				if err := os.Remove(tmpFile); err != nil && !os.IsNotExist(err) {
					errCh <- fmt.Errorf("failed to remove temp shard %s: %w", tmpFile, err)
					return
				}
				errCh <- nil // Success for this shard
			}
		}()
	}

	// Feed jobs
	for i := 0; i < w.numShards; i++ {
		shardJobs <- i
	}
	close(shardJobs)

	// Collect results
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err // Return first error
		}
	}

	return nil
}

// isBestEffortError checks if an error from Sync is a known, non-fatal
// error on some platforms (e.g., EINVAL on ZFS, ENOTSUP).
func isBestEffortError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common cross-platform syscall errors
	// These are often wrapped, so we check errors.Is
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) {
		return true
	}
	return false
}

// processShard sorts a temporary shard file and reduces it to its final
// key-sorted (key, lastOffset) format.
func (w *PreIndexWriter) processShard(tmpFile, datFile string) error {
	// Open the temporary shard file for reading
	f, err := os.Open(tmpFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No data for this shard, skip
		}
		return fmt.Errorf("failed to open temp shard %s: %w", tmpFile, err)
	}
	defer f.Close()

	// Check file size for corruption
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat temp shard %s: %w", tmpFile, err)
	}
	if fi.Size()%tempRecordSize != 0 {
		return fmt.Errorf("temp shard file %s is corrupt: size %d is not a multiple of %d",
			tmpFile, fi.Size(), tempRecordSize)
	}
	if fi.Size() == 0 {
		return nil // Empty file, nothing to do
	}

	numRecs := fi.Size() / tempRecordSize
	// Guard against overflow on 32-bit.
	capRecs := int(numRecs)
	if int64(capRecs) != numRecs {
		capRecs = fallbackPreallocCap // PREALLOC FIX: Use fallback cap
	}
	records := make([]tempRecord, 0, capRecs) // Pre-allocate capacity
	slabBuf := make([]byte, readBufferSize)   // 1MB slab

	readOffset := int64(0)
	for readOffset < fi.Size() {
		toRead := int64(len(slabBuf))
		if remain := fi.Size() - readOffset; remain < toRead {
			toRead = remain
		}

		// Read a full slab
		n, err := f.ReadAt(slabBuf[:toRead], readOffset)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read tmp slab at %d: %w", readOffset, err)
		}
		if int64(n) != toRead {
			return fmt.Errorf("short read at offset %d: read %d, expected %d", readOffset, n, toRead)
		}
		if n == 0 && err == io.EOF {
			break // Should be covered by readOffset < fi.Size()
		}

		// Process the slab (we know it's full records due to file size check)
		for i := 0; i+tempRecordSize <= n; i += tempRecordSize {
			var rec tempRecord
			base := slabBuf[i : i+tempRecordSize]
			copy(rec.Key[:], base[0:64])
			rec.Offset = Value(binary.LittleEndian.Uint32(base[64:68]))
			rec.Index = binary.LittleEndian.Uint64(base[68:76])
			records = append(records, rec)
		}
		readOffset += int64(n)
	}

	if int64(len(records)) != numRecs {
		return fmt.Errorf("record count mismatch: expected %d, read %d (file size %d)", numRecs, len(records), fi.Size())
	}

	if len(records) == 0 {
		return nil // No records, nothing to do
	}

	// --- In-Memory Sort ---
	// CRITICAL LIMITATION: This is the RAM-bound step. For 1B records and
	// 128 shards, this is ~8M records, or ~640MB+ per shard.
	// A bad hash skew or low shard count will exhaust RAM.
	sort.Slice(records, func(i, j int) bool {
		cmp := bytes.Compare(records[i].Key[:], records[j].Key[:])
		if cmp == 0 {
			// If keys are equal, sort by *insertion index*
			return records[i].Index < records[j].Index
		}
		return cmp < 0
	})

	// --- Stream-Reduce to Final File ---
	// Write to a .tmp file first for atomic rename
	datFileTmp := datFile + ".tmp"
	df, err := os.Create(datFileTmp)
	if err != nil {
		return fmt.Errorf("failed to create final shard file %s: %w", datFileTmp, err)
	}
	dw := bufio.NewWriter(df)

	// I/O PERF: Use a reusable slab for writing
	var writeSlab [finalRecordSize]byte

	// Stream through sorted records, writing only the *last* one for each key
	for i := 0; i < len(records); {
		// This is the last record for this key because of the sort order
		lastRecForKey := records[i]

		// Skip all other records for this same key
		j := i + 1
		for j < len(records) && bytes.Equal(records[i].Key[:], records[j].Key[:]) {
			lastRecForKey = records[j] // Update to the one with the highest index
			j++
		}
		i = j // Move cursor to the next new key

		copy(writeSlab[0:64], lastRecForKey.Key[:])
		binary.LittleEndian.PutUint32(writeSlab[64:68], uint32(lastRecForKey.Offset))
		if _, err := dw.Write(writeSlab[:]); err != nil {
			df.Close()
			return fmt.Errorf("failed to write final record: %w", err)
		}
	}

	if err := dw.Flush(); err != nil {
		df.Close()
		return fmt.Errorf("failed to flush final shard %s: %w", datFileTmp, err)
	}
	if err := df.Sync(); err != nil {
		df.Close()
		return fmt.Errorf("failed to sync final shard %s: %w", datFileTmp, err)
	}
	if err := df.Close(); err != nil {
		return fmt.Errorf("failed to close final shard %s: %w", datFileTmp, err)
	}

	// Atomic rename for crash consistency
	if err := os.Rename(datFileTmp, datFile); err != nil {
		return fmt.Errorf("failed to rename final shard: %w", err)
	}

	// DURABILITY: Sync the directory to make the rename durable
	dirF, err := os.Open(w.dir)
	if err != nil {
		return fmt.Errorf("failed to open directory for sync: %w", err)
	}
	// PORTABILITY FIX: Make dir sync best-effort
	if err := dirF.Sync(); err != nil && !isBestEffortError(err) {
		dirF.Close()
		return fmt.Errorf("failed to sync directory: %w", err)
	}
	if err := dirF.Close(); err != nil {
		return fmt.Errorf("failed to close directory after sync: %w", err)
	}
	slog.Info("Processed shard", slog.String("tempFile", tmpFile), slog.String("finalFile", datFile))

	return nil
}

// ReaderOption defines a functional option for configuring the PreIndexReader.
type ReaderOption func(*PreIndexReader)

// PreIndexReader handles loading and reading the pre-index.
// This implementation loads all shard files into RAM.
type PreIndexReader struct {
	*preIndexBase
	shardSlabs map[int][]byte
	loadOnce   sync.Once
	loadErr    error
}

// NewPreIndexReader creates a new PreIndexReader instance.
func NewPreIndexReader(dir string, numShards int, opts ...ReaderOption) (*PreIndexReader, error) {
	base, err := newPreIndexBase(dir, numShards)
	if err != nil {
		return nil, err
	}

	r := &PreIndexReader{
		preIndexBase: base,
		shardSlabs:   make(map[int][]byte, numShards),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// Close releases all cached file handles.
func (r *PreIndexReader) Close() error {
	for shardID, slab := range r.shardSlabs {
		clear(slab) // Help GC
		delete(r.shardSlabs, shardID)
	}
	r.shardSlabs = nil
	return nil
}

// Load reads the final, reduced shard files from disk into memory
// for fast lookups in the second pass.
func (r *PreIndexReader) Load() error {
	r.loadOnce.Do(func() {
		// We read all shard files into memory.
		errs := make(chan error, r.numShards)
		var wg sync.WaitGroup
		wg.Add(r.numShards)

		// Use a mutex to protect the shardSlabs map
		var mu sync.Mutex

		for i := 0; i < r.numShards; i++ {
			go func(shardID int) {
				defer wg.Done()
				shardFile := r.shardFileName(shardID, false)
				f, err := os.Open(shardFile)
				if err != nil {
					if os.IsNotExist(err) {
						// Store nil slice for non-existent shards
						mu.Lock()
						r.shardSlabs[shardID] = nil
						mu.Unlock()
						return // Not an error
					}
					errs <- fmt.Errorf("failed to open shard %s: %w", shardFile, err)
					return
				}
				defer f.Close()

				data, err := io.ReadAll(f)
				if err != nil {
					errs <- fmt.Errorf("failed to read shard %s: %w", shardFile, err)
					return
				}

				if len(data)%finalRecordSize != 0 {
					errs <- fmt.Errorf("shard file %s is corrupt: size %d not multiple of %d",
						shardFile, len(data), finalRecordSize)
					return
				}

				mu.Lock()
				r.shardSlabs[shardID] = data
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				r.loadErr = err // Store first error
				return
			}
		}
	})

	return r.loadErr
}

// IsLast checks if the given (key, offset) pair is the last one seen
// for that key during the build phase. This performs an in-memory
// binary search and *requires* Load() to have been called.
func (r *PreIndexReader) IsLast(key Key, offset Value) (bool, error) {
	shardID := r.getShardID(key)

	slab, ok := r.shardSlabs[shardID]
	if !ok {
		// This implies Load() was not called or the index is corrupt
		return false, fmt.Errorf("shard %d not loaded; call Load() first", shardID)
	}

	// Perform in-memory binary search
	foundOffset, ok, err := r.binarySearchSlab(slab, key)
	if err != nil {
		return false, fmt.Errorf("error searching shard %d: %w", shardID, err)
	}

	if !ok {
		// Key not found in binary search
		return false, nil
	}

	return foundOffset == offset, nil
}

func (r *PreIndexReader) IsLastMustFind(key Key, offset Value) (bool, error) {
	shardID := r.getShardID(key)

	slab, ok := r.shardSlabs[shardID]
	if !ok {
		// This implies Load() was not called or the index is corrupt
		return false, fmt.Errorf("shard %d not loaded; call Load() first", shardID)
	}

	// Perform in-memory binary search
	foundOffset, ok, err := r.binarySearchSlab(slab, key)
	if err != nil {
		return false, fmt.Errorf("error searching shard %d: %w", shardID, err)
	}

	if !ok {
		// Key not found in binary search
		return false, fmt.Errorf("key not found in shard %d", shardID)
	}

	return foundOffset == offset, nil
}

// binarySearchSlab performs a binary search on the key-sorted slab.
func (r *PreIndexReader) binarySearchSlab(slab []byte, targetKey Key) (Value, bool, error) {
	fileSize := int64(len(slab))

	// CORRECTNESS FIX: Treat empty file as "not found"
	if fileSize == 0 {
		return 0, false, nil
	}

	if fileSize%finalRecordSize != 0 {
		// Slab is corrupt (should have been caught in Load)
		return 0, false, fmt.Errorf("shard slab is corrupt: size %d not multiple of %d",
			fileSize, finalRecordSize)
	}

	numRecords := fileSize / finalRecordSize
	low := int64(0)
	high := numRecords - 1

	for low <= high {
		mid := low + (high-low)/2
		offset := mid * finalRecordSize

		// SEARCH PERF: Read from in-memory slab
		recordSlab := slab[offset : offset+finalRecordSize]

		// SEARCH PERF: Manually decode from slab
		cmp := bytes.Compare(recordSlab[0:64], targetKey[:])
		if cmp == 0 {
			// Found it. Decode offset.
			foundOffset := Value(binary.LittleEndian.Uint32(recordSlab[64:68]))
			return foundOffset, true, nil
		} else if cmp < 0 {
			// Read key is less than target key, search in upper half
			low = mid + 1
		} else {
			// Read key is greater than target key, search in lower half
			high = mid - 1
		}
	}

	// Not found
	return 0, false, nil
}
