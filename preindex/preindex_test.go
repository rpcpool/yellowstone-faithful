package preindex

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// --- Helpers ---

// A simple key generation helper
func key(s string) Key {
	var k Key
	copy(k[:], []byte(s))
	return k
}

// A key generator for benchmarks/race tests
func keyFromInt(i int) Key {
	var k Key
	s := fmt.Sprintf("key-%056d", i) // Pad to 64 bytes
	copy(k[:], []byte(s))
	return k
}

// mustPush fails the test if a Push fails
func mustPush(t testing.TB, w *PreIndexWriter, k Key, o Value) {
	t.Helper()
	if err := w.Push(k, o); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
}

// mustBuild fails the test if a Build fails
func mustBuild(t testing.TB, w *PreIndexWriter) {
	t.Helper()
	if err := w.Build(); err != nil {
		t.Fatalf("Build failed: %v", err)
	}
}

// newTestWriter creates a writer in a new temp directory
func newTestWriter(t testing.TB, numShards int, opts ...WriterOption) (*PreIndexWriter, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := NewPreIndexWriter(dir, numShards, opts...)
	if err != nil {
		t.Fatalf("NewPreIndexWriter failed: %v", err)
	}
	return w, dir
}

// newTestReader creates a reader from a given directory
func newTestReader(t testing.TB, dir string, numShards int, opts ...ReaderOption) *PreIndexReader {
	t.Helper()
	r, err := NewPreIndexReader(dir, numShards, opts...)
	if err != nil {
		t.Fatalf("NewPreIndexReader failed: %v", err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Logf("Warning: reader.Close() failed: %v", err)
		}
	})
	return r
}

// checkIsLast is a test assertion for IsLast
func checkIsLast(t testing.TB, r *PreIndexReader, k Key, o Value, expected bool) {
	t.Helper()
	isLast, err := r.IsLast(k, o)
	if err != nil {
		t.Fatalf("IsLast(%x, %d) failed: %v", k[:8], o, err)
	}
	if isLast != expected {
		t.Errorf("IsLast(%x, %d): got %v, want %v", k[:8], o, isLast, expected)
	}
}

// readAllDatFile reads all records from a final .dat file for verification
func readAllDatFile(t *testing.T, path string) []tempRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("Failed to open dat file %s: %v", path, err)
	}
	defer f.Close()

	var records []tempRecord
	var slab [finalRecordSize]byte
	for {
		_, err := f.Read(slab[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read dat file: %v", err)
		}
		var r tempRecord
		copy(r.Key[:], slab[0:64])
		r.Offset = Value(binary.LittleEndian.Uint32(slab[64:68]))
		records = append(records, r)
	}
	return records
}

// writeTmpFile creates a .tmp file with the given records
func writeTmpFile(t *testing.T, path string, records []tempRecord) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var slab [tempRecordSize]byte
	for _, rec := range records {
		copy(slab[0:64], rec.Key[:])
		binary.LittleEndian.PutUint32(slab[64:68], uint32(rec.Offset))
		binary.LittleEndian.PutUint64(slab[68:76], rec.Index)
		if _, err := f.Write(slab[:]); err != nil {
			t.Fatalf("Failed to write tmp file: %v", err)
		}
	}
}

// --- Tests ---

func Test_Writer_Basic(t *testing.T) {
	w, dir := newTestWriter(t, 2)

	// k1 -> shard 0
	// k2 -> shard 1
	// k3 -> shard 0
	// (Based on my hash seed, adjust if this fails)
	k1 := key("key-a") // shard 0
	k2 := key("key-b") // shard 1
	k3 := key("key-c") // shard 0

	// Push records
	mustPush(t, w, k1, 100) // k1, offset 100
	mustPush(t, w, k2, 200) // k2, offset 200
	mustPush(t, w, k1, 101) // k1, offset 101 (last)
	mustPush(t, w, k3, 300) // k3, offset 300
	mustPush(t, w, k2, 201) // k2, offset 201 (last)

	// Finalize
	mustBuild(t, w)

	// Create reader and check
	r := newTestReader(t, dir, 2)
	if err := r.Load(); err != nil { // IN-MEMORY FIX: Load data
		t.Fatalf("r.Load() failed: %v", err)
	}

	// k1 checks
	checkIsLast(t, r, k1, 100, false)
	checkIsLast(t, r, k1, 101, true)

	// k2 checks
	checkIsLast(t, r, k2, 200, false)
	checkIsLast(t, r, k2, 201, true)

	// k3 check
	checkIsLast(t, r, k3, 300, true)

	// Not found check
	checkIsLast(t, r, key("not-found"), 999, false)
}

func Test_Writer_Lifecycle(t *testing.T) {
	w, _ := newTestWriter(t, 1)

	// Test Push after Build
	mustPush(t, w, key("k1"), 1)
	mustBuild(t, w)

	if err := w.Push(key("k2"), 2); !errors.Is(err, ErrWriterClosed) {
		t.Errorf("Push after Build: got %v, want %v", err, ErrWriterClosed)
	}

	// Test Build after Build
	if err := w.Build(); !errors.Is(err, ErrWriterClosed) {
		t.Errorf("Build after Build: got %v, want %v", err, ErrWriterClosed)
	}

	// Test empty build
	w2, dir2 := newTestWriter(t, 1)
	mustBuild(t, w2)
	r2 := newTestReader(t, dir2, 1)
	if err := r2.Load(); err != nil { // IN-MEMORY FIX: Load data
		t.Fatalf("r2.Load() failed: %v", err)
	}
	checkIsLast(t, r2, key("k-any"), 123, false)
}

func Test_Writer_ShardBufferFlush(t *testing.T) {
	// 1 shard, buffer cap of 1 byte. Every push must flush.
	w, dir := newTestWriter(t, 1, WithShardBufferCap(1))

	// Push 10 records. This must trigger 10 flushes.
	for i := 0; i < 10; i++ {
		mustPush(t, w, keyFromInt(i), Value(i))
	}
	mustPush(t, w, keyFromInt(0), 100) // Last offset for key 0

	mustBuild(t, w)

	// Check .tmp file was created and written to
	tmpFile := w.shardFileName(0, true)
	_, err := os.Stat(tmpFile)
	if !os.IsNotExist(err) {
		t.Errorf("Expected .tmp file to be gone, but it exists (or other error: %v)", err)
	}

	// Check final data
	r := newTestReader(t, dir, 1)
	if err := r.Load(); err != nil { // IN-MEMORY FIX: Load data
		t.Fatalf("r.Load() failed: %v", err)
	}
	checkIsLast(t, r, keyFromInt(0), 0, false)
	checkIsLast(t, r, keyFromInt(0), 100, true)
	checkIsLast(t, r, keyFromInt(5), 5, true)
}

func Test_Writer_GlobalBufferFlush(t *testing.T) {
	// 10 shards, global cap of 1 byte. Every push must flush.
	w, dir := newTestWriter(t, 10, WithTotalBufferCap(1))

	// Push 100 records. This must trigger flushes.
	for i := 0; i < 100; i++ {
		mustPush(t, w, keyFromInt(i), Value(i))
	}
	mustPush(t, w, keyFromInt(42), 100) // Last offset for key 42

	mustBuild(t, w)

	// Check final data
	r := newTestReader(t, dir, 10)
	if err := r.Load(); err != nil { // IN-MEMORY FIX: Load data
		t.Fatalf("r.Load() failed: %v", err)
	}
	checkIsLast(t, r, keyFromInt(42), 42, false)
	checkIsLast(t, r, keyFromInt(42), 100, true)
	checkIsLast(t, r, keyFromInt(10), 10, true)
}

func Test_processShard_ReduceAndSort(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "test.tmp")
	datFile := filepath.Join(dir, "test.dat")

	// k1, k2, k1 (out of order index), k3, k2 (last)
	records := []tempRecord{
		{key("k1"), 100, 0},
		{key("k2"), 200, 1},
		{key("k1"), 102, 3}, // Higher index, should be "last"
		{key("k3"), 300, 4},
		{key("k1"), 101, 2}, // Lower index
		{key("k2"), 201, 5}, // Higher index, "last"
	}
	writeTmpFile(t, tmpFile, records)

	// Create a dummy base to call processShard
	base, _ := newPreIndexBase(dir, 1)
	w := &PreIndexWriter{preIndexBase: base}

	if err := w.processShard(tmpFile, datFile); err != nil {
		t.Fatalf("processShard failed: %v", err)
	}

	// Check .dat file contents
	finalRecs := readAllDatFile(t, datFile)
	expectedRecs := []tempRecord{
		{key("k1"), 102, 0}, // Sorted by key
		{key("k2"), 201, 0}, // Reduced to last offset
		{key("k3"), 300, 0},
	}

	if !reflect.DeepEqual(finalRecs, expectedRecs) {
		t.Errorf("processShard output mismatch:\ngot:  %v\nwant: %v", finalRecs, expectedRecs)
	}
}

func Test_processShard_Corrupt(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "test.tmp")
	datFile := filepath.Join(dir, "test.dat")

	// Write 81 bytes (corrupt)
	f, _ := os.Create(tmpFile)
	f.Write(make([]byte, 81))
	f.Close()

	base, _ := newPreIndexBase(dir, 1)
	w := &PreIndexWriter{preIndexBase: base}

	if err := w.processShard(tmpFile, datFile); err == nil {
		t.Fatal("processShard succeeded on corrupt file, expected error")
	}
}

// IN-MEMORY FIX: Renamed and simplified test for in-memory search
func Test_Reader_binarySearchSlab(t *testing.T) {
	// Create a sorted slab
	var slab []byte
	var slabBuf [finalRecordSize]byte
	recs := []tempRecord{
		{key("k-mid-1"), 10, 0},
		{key("k-mid-2"), 20, 0},
		{key("k-mid-3"), 30, 0},
	}
	for _, rec := range recs {
		copy(slabBuf[0:64], rec.Key[:])
		binary.LittleEndian.PutUint32(slabBuf[64:68], uint32(rec.Offset))
		slab = append(slab, slabBuf[:]...)
	}

	// Create empty and corrupt slabs
	var slabEmpty []byte
	slabCorrupt := make([]byte, 73) // 73 bytes, not multiple of 72

	r := newTestReader(t, t.TempDir(), 1) // Dummy reader to call method

	tests := []struct {
		name       string
		slab       []byte
		key        Key
		wantOffset Value
		wantOk     bool
		wantErr    bool
	}{
		{"Found", slab, key("k-mid-2"), 20, true, false},
		{"Not Found (Before)", slab, key("k-aaa"), 0, false, false},
		{"Not Found (After)", slab, key("k-zzz"), 0, false, false},
		{"Not Found (Mid)", slab, key("k-mid-2a"), 0, false, false},
		{"Empty File", slabEmpty, key("k-mid-2"), 0, false, false},
		{"Corrupt File", slabCorrupt, key("k-mid-2"), 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			off, ok, err := r.binarySearchSlab(tt.slab, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("binarySearchSlab() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantOk {
				t.Errorf("binarySearchSlab() ok = %v, wantOk %v", ok, tt.wantOk)
			}
			if off != tt.wantOffset {
				t.Errorf("binarySearchSlab() offset = %v, wantOffset %v", off, tt.wantOffset)
			}
		})
	}
}

// Test_Reader_Race hammers the reader with concurrent IsLast calls
// across a small, aggressively-evicted cache.
// Run with `go test -race ./...` to verify.
func Test_Reader_Race(t *testing.T) {
	t.Parallel()

	numKeys := 1000
	numShards := 10
	w, dir := newTestWriter(t, numShards)

	// Build a file
	offsets := make(map[int]Value)
	for i := 0; i < numKeys; i++ {
		off := Value(rand.Uint64())
		mustPush(t, w, keyFromInt(i), off)
		offsets[i] = off // Store last offset
	}
	mustBuild(t, w)

	// Create reader with tiny, aggressive cache
	r := newTestReader(t, dir, numShards) // IN-MEMORY FIX: Removed WithReaderCacheCap
	if err := r.Load(); err != nil {      // IN-MEMORY FIX: Load data
		t.Fatalf("r.Load() failed: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	numGoroutines := 50
	numReadsPerGo := 200

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goID int) { // Pass goroutine ID
			defer wg.Done()
			for j := 0; j < numReadsPerGo; j++ {
				keyIdx := rand.Intn(numKeys)
				key := keyFromInt(keyIdx)
				expectedOffset := offsets[keyIdx]

				// This call hammers getShardFile, pin/unpin, and binarySearch
				isLast, err := r.IsLast(key, expectedOffset)
				if err != nil {
					// We must not have errors (like EBADF)
					t.Errorf("Goroutine %d: IsLast failed: %v", goID, err)
					return // RACE FIX: Stop goroutine on error
				}
				if !isLast {
					t.Errorf("Goroutine %d: IsLast check failed for key %d", goID, keyIdx)
					return // RACE FIX: Stop goroutine on error
				}
			}
		}(i) // Pass goroutine ID
	}

	wg.Wait()
}

func Test_randomKeys(t *testing.T) {
	// Generate 1M random keys random reader
	numKeys := 100
	keys := make(map[Key]struct{})
	for len(keys) < numKeys {
		var k Key
		if _, err := rand.Read(k[:]); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}
		keys[k] = struct{}{}
	}

	// write to index
	w, dir := newTestWriter(t, 128)
	for k := range keys {
		mustPush(t, w, k, Value(0))
	}
	withMultipleRecords := make(map[Key][]Value)
	// for a subset, push a second record to test "last"
	i := 0
	for k := range keys {
		if i%10 == 0 { // 10% of keys get a second record
			o := Value(rand.Uint64())
			mustPush(t, w, k, o)
			withMultipleRecords[k] = append(withMultipleRecords[k], o)
		}
		i++
	}
	// for a subset, push a third record to test "last"
	i = 0
	for k := range keys {
		if i%50 == 0 { // 2% of keys get a third record
			o := Value(rand.Uint64())
			mustPush(t, w, k, o)
			withMultipleRecords[k] = append(withMultipleRecords[k], o)
		}
		i++
	}
	mustBuild(t, w)

	// read back and verify
	r := newTestReader(t, dir, 128)
	if err := r.Load(); err != nil { // IN-MEMORY FIX: Load data
		t.Fatalf("r.Load() failed: %v", err)
	}
	for k := range keys {
		// We don't care about offsets here, just that IsLast works without error
		last, err := r.IsLast(k, 0)
		if err != nil {
			t.Fatalf("IsLast failed for key %x: %v", k[:8], err)
		}
		_, hasMultiple := withMultipleRecords[k]
		if hasMultiple {
			if last {
				// cannot be last if multiple records
				t.Errorf("IsLast for key %x with multiple records: got true, want false", k[:8])
			} else {
				// now check the last offset
				lastOffsets := withMultipleRecords[k]
				lastOffset := lastOffsets[len(lastOffsets)-1]
				isLast, err := r.IsLast(k, lastOffset)
				if err != nil {
					t.Fatalf("IsLast failed for key %x last offset %d: %v", k[:8], lastOffset, err)
				}
				if !isLast {
					t.Errorf("IsLast for key %x last offset %d: got false, want true", k[:8], lastOffset)
				}
			}
		} else {
			if !last {
				// must be last if single record
				t.Errorf("IsLast for key %x with single record: got false, want true", k[:8])
			}
		}
	}
}

// --- Benchmarks ---

// buildTestIndex is a helper for benchmarks to create a pre-built index
func buildTestIndex(b *testing.B, numKeys int, numShards int) (string, map[int]Value) {
	b.Helper()
	// Stop timer during setup
	b.StopTimer()

	w, dir := newTestWriter(b, numShards)

	offsets := make(map[int]Value)
	for i := 0; i < numKeys; i++ {
		// Push 2 records per key to test "last" logic
		mustPush(b, w, keyFromInt(i), Value(i*2))
		lastOff := Value(i*2 + 1)
		mustPush(b, w, keyFromInt(i), lastOff)
		offsets[i] = lastOff // Store last offset
	}
	mustBuild(b, w)

	// Restart timer after setup
	b.StartTimer()
	return dir, offsets
}

func Benchmark_Writer_Push(b *testing.B) {
	// 100 shards, default buffer sizes
	w, _ := newTestWriter(b, 100)
	defer w.Build()

	// Pre-generate keys to avoid that in the loop
	keys := make([]Key, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = keyFromInt(i)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Use pre-generated key
		if err := w.Push(keys[i], Value(i)); err != nil {
			b.Fatal(err)
		}
	}
}

// IN-MEMORY FIX: Renamed benchmark
func Benchmark_Reader_IsLast(b *testing.B) {
	numKeys := 100_000
	numShards := 128
	dir, offsets := buildTestIndex(b, numKeys, numShards)

	// Create reader and load data
	r := newTestReader(b, dir, numShards)
	if err := r.Load(); err != nil {
		b.Fatalf("r.Load() failed: %v", err)
	}
	defer r.Close()

	// IN-MEMORY FIX: Removed pre-warming loop

	// All lookups should be hot
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		keyIdx := i % numKeys
		key := keyFromInt(keyIdx)
		expectedOffset := offsets[keyIdx]

		isLast, err := r.IsLast(key, expectedOffset)
		if err != nil {
			b.Fatal(err)
		}
		if !isLast {
			b.Fatalf("Cache-hit check failed for key %d", keyIdx)
		}
	}
}
