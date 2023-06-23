package filecache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	var (
		evictedCount int
		evictedName  string
		evictedRefs  int
	)
	onEvicted := func(file *os.File, refs int) {
		t.Logf("Removed %q from cache", filepath.Base(file.Name()))
		evictedCount++
		evictedName = file.Name()
		evictedRefs = refs
	}

	fc := NewOpenFile(2, os.O_CREATE|os.O_RDWR, 0644)
	fc.SetOnEvicted(onEvicted)

	tmp := t.TempDir()
	fooName := filepath.Join(tmp, "foo")
	barName := filepath.Join(tmp, "bar")
	bazName := filepath.Join(tmp, "baz")

	_, err := fc.Open(fooName)
	require.NoError(t, err)

	barFile, err := fc.Open(barName)
	require.NoError(t, err)

	fooFile, err := fc.Open(fooName)
	require.NoError(t, err)

	require.Zero(t, evictedCount)

	bazFile, err := fc.Open(bazName)
	require.NoError(t, err)

	require.Equal(t, 1, evictedCount)
	require.Equal(t, barName, evictedName)
	require.Equal(t, 1, evictedRefs)

	require.NoError(t, fc.Close(barFile))

	barFile, err = fc.Open(barName)
	require.NoError(t, err)
	require.NoError(t, fc.Close(barFile))

	require.Equal(t, 2, evictedCount)
	require.Equal(t, fooName, evictedName)
	require.Equal(t, 2, evictedRefs)

	require.NoError(t, fc.Close(fooFile))
	require.NoError(t, fc.Close(fooFile))
	err = fc.Close(fooFile)
	require.ErrorContains(t, err, os.ErrClosed.Error())

	fc.Remove(bazName)

	require.Equal(t, 3, evictedCount)
	require.Equal(t, bazName, evictedName)
	require.Equal(t, 1, evictedRefs)

	require.NoError(t, fc.Close(bazFile))

	err = fc.Close(bazFile)
	require.ErrorContains(t, err, os.ErrClosed.Error())

	// barFile closed, but still in cache, with zero references.
	require.Equal(t, 1, fc.Len())
	fc.Remove(barName)

	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.removed))

	// Check that double close returns error
	fooFile, err = fc.Open(fooName)
	require.NoError(t, err)
	require.NoError(t, fc.Close(fooFile))
	err = fc.Close(fooFile)
	require.ErrorContains(t, err, os.ErrClosed.Error())

	fc.Clear()

	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.removed))

	hits, misses, ln, cp := fc.Stats()
	t.Logf("Cache stats: hits=%d misses=%d len=%d cap=%d", hits, misses, ln, cp)
}

func TestMultiFileInstances(t *testing.T) {
	var (
		evictedCount int
		evictedName  string
		evictedRefs  int
	)
	onEvicted := func(file *os.File, refs int) {
		t.Logf("Removed %q from cache", filepath.Base(file.Name()))
		evictedCount++
		evictedName = file.Name()
		evictedRefs = refs
	}
	fc := NewOpenFile(2, os.O_CREATE|os.O_RDWR, 0644)
	fc.SetOnEvicted(onEvicted)

	tmp := t.TempDir()
	fooName := filepath.Join(tmp, "foo")
	barName := filepath.Join(tmp, "bar")
	bazName := filepath.Join(tmp, "baz")

	// Incr reference count to 3.
	fooFile, err := fc.Open(fooName)
	require.NoError(t, err)
	_, err = fc.Open(fooName)
	require.NoError(t, err)
	_, err = fc.Open(fooName)
	require.NoError(t, err)

	barFile, err := fc.Open(barName)
	require.NoError(t, err)
	require.NoError(t, fc.Close(barFile))

	require.Equal(t, 0, len(fc.removed))
	require.Equal(t, 2, fc.Len())

	// Cause foo to be evicted.
	bazFile, err := fc.Open(bazName)
	require.NoError(t, err)
	require.NoError(t, fc.Close(bazFile))
	require.Equal(t, 2, fc.Len())

	// Since foo is still referenced, so should be put into removed.
	require.Equal(t, 1, len(fc.removed))

	require.Equal(t, 1, evictedCount)
	require.Equal(t, fooName, evictedName)
	require.Equal(t, 3, evictedRefs)

	// Open foo again, should be one file with reference of 1.
	fooFileX, err := fc.Open(fooName)
	require.NoError(t, err)
	require.NotEqual(t, fooFile, fooFileX)

	// Check that bar was evicted.
	require.Equal(t, 2, evictedCount)
	require.Equal(t, barName, evictedName)
	require.Equal(t, 0, evictedRefs)

	// Since bar was evicted with no references, it should not be put into
	// removed. Make sure that only fooFile is in removed.
	require.Equal(t, 1, len(fc.removed))
	refs, ok := fc.removed[fooFile]
	require.True(t, ok)
	require.Equal(t, 3, refs)

	// Remove the fooFileX from cache, without closing the file first. Since it
	// still has a non-zero reference count, it is put into removed, along with
	// the other instance of fooFile.
	fc.Remove(fooName)

	// Check that there are two distinct files in removed, with different
	// reference counts.
	require.Equal(t, 2, len(fc.removed))
	refs, ok = fc.removed[fooFile]
	require.True(t, ok)
	require.Equal(t, 3, refs)
	refs, ok = fc.removed[fooFileX]
	require.True(t, ok)
	require.Equal(t, 1, refs)

	// Close fooFileX and check that is no longer in removed.
	require.NoError(t, fc.Close(fooFileX))
	require.Equal(t, 1, len(fc.removed))
	refs, ok = fc.removed[fooFile]
	require.True(t, ok)
	require.Equal(t, 3, refs)

	// Closing fooFileX again should result in error.
	err = fc.Close(fooFileX)
	require.ErrorContains(t, err, os.ErrClosed.Error())

	// Make sure 3 closes are required to remove fooFile.
	require.NoError(t, fc.Close(fooFile))
	require.Equal(t, 1, len(fc.removed))
	require.NoError(t, fc.Close(fooFile))
	require.Equal(t, 1, len(fc.removed))
	require.NoError(t, fc.Close(fooFile))
	require.Equal(t, 0, len(fc.removed))
	err = fc.Close(fooFile)
	require.ErrorContains(t, err, os.ErrClosed.Error())

	// baz should still be in cache.
	require.Equal(t, 1, fc.Len())

	fc.Clear()
	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.removed))

	hits, misses, ln, cp := fc.Stats()
	require.Equal(t, 2, hits)
	require.Equal(t, 4, misses)
	t.Logf("Cache stats: hits=%d misses=%d len=%d cap=%d", hits, misses, ln, cp)
}

func TestZeroSize(t *testing.T) {
	fc := NewOpenFile(0, os.O_CREATE|os.O_RDWR, 0644)

	var evicted bool
	fc.SetOnEvicted(func(file *os.File, refs int) {
		evicted = true
	})
	require.Zero(t, fc.Len())
	require.Zero(t, fc.Cap())

	tmp := t.TempDir()
	fooName := filepath.Join(tmp, "foo")
	barName := filepath.Join(tmp, "bar")

	file1, err := fc.Open(fooName)
	require.NoError(t, err)
	require.Zero(t, fc.Len())
	require.False(t, evicted)

	file2, err := fc.Open(barName)
	require.NoError(t, err)
	require.False(t, evicted)

	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.removed))

	require.NoError(t, fc.Close(file1))
	require.Zero(t, len(fc.removed))

	require.NoError(t, fc.Close(file2))
	require.Zero(t, len(fc.removed))

	require.Zero(t, fc.Len())
}

func TestFuzz(t *testing.T) {
	const (
		capacity    = 3
		concurrency = 1000
		reps        = 100
		delay       = 500 * time.Microsecond
	)

	fc := NewOpenFile(capacity, os.O_CREATE|os.O_RDWR, 0644)

	tmp := t.TempDir()
	fooName := filepath.Join(tmp, "foo")
	barName := filepath.Join(tmp, "bar")
	bazName := filepath.Join(tmp, "baz")
	bifName := filepath.Join(tmp, "bif")
	names := []string{fooName, barName, bazName, bifName}

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(name string) {
			for x := 0; x < reps; x++ {
				f, err := fc.Open(name)
				require.NoError(t, err, "opening file", name)
				time.Sleep(delay)
				err = fc.Close(f)
				require.NoError(t, err, "closing file", name)
			}
			defer wg.Done()
		}(names[i%len(names)])
	}

	wg.Wait()

	for name, elem := range fc.cache {
		ent := elem.Value.(*entry)
		require.Zero(t, ent.refs, "expected zero ref count for cached file", name)
	}

	require.Zero(t, len(fc.removed))
	fc.Clear()
	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.cache))
	require.Zero(t, len(fc.removed))
}

func TestEvict(t *testing.T) {
	// Open a file to get 1st *File, let it be evicted, open that file again to
	// get 2nd *File, then close both 1st and 2nd *File. This tests that the
	// first Close closes the evicted *File, not the one in cache.
	const (
		capacity = 2
	)

	var evictions int
	onEvicted := func(file *os.File, refs int) {
		t.Logf("Removed %q from cache, refs: %d", filepath.Base(file.Name()), refs)
		evictions++
	}
	fc := NewOpenFile(capacity, os.O_CREATE|os.O_RDWR, 0644)
	fc.SetOnEvicted(onEvicted)

	tmp := t.TempDir()
	fooName := filepath.Join(tmp, "foo")
	barName := filepath.Join(tmp, "bar")
	bazName := filepath.Join(tmp, "baz")
	names := []string{fooName, barName, bazName}

	var err error
	files := make([]*os.File, len(names))
	for i, name := range names {
		t.Log("Opening", name)
		files[i], err = fc.Open(name)
		require.NoError(t, err, "opening file", name)
	}

	t.Log("Opening", fooName, "again")
	f2, err := fc.Open(fooName)
	require.NoError(t, err, "opening file", fooName)

	for _, file := range files {
		t.Log("Closing", file.Name())
		err = fc.Close(file)
		require.NoError(t, err, "closing file", file.Name())
	}

	t.Log("Closing other handle for", f2.Name())
	err = fc.Close(f2)
	require.NoError(t, err, "closing file", f2.Name())

	require.Equal(t, len(names)+1-capacity, evictions)
	fc.Clear()
	require.Zero(t, fc.Len())
	require.Zero(t, len(fc.cache))
	require.Zero(t, len(fc.removed))

	// Check that all files are really closed.
	for _, file := range files {
		require.Error(t, file.Close())
	}
	require.Error(t, f2.Close())
}
