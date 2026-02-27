// bucketteer_test.go
package bucketteer

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/require"
)

func TestEytzingerLayout(t *testing.T) {
	// 1-based Eytzinger for [1, 2, 3, 4, 5, 6, 7] should be [4, 2, 6, 1, 3, 5, 7]
	in := []int{1, 2, 3, 4, 5, 6, 7}
	expected := []int{4, 2, 6, 1, 3, 5, 7}
	sortWithCompare(in, func(i, j int) int { return in[i] - in[j] })
	require.Equal(t, expected, in)
}

func TestCleanSet(t *testing.T) {
	in := []uint64{5, 1, 3, 3, 5, 2}
	out := getCleanSet(in)
	require.Equal(t, []uint64{1, 2, 3, 5}, out)
}

func TestBoundaryBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boundary.buck")
	wr, err := NewWriter(path)
	require.NoError(t, err)

	// Force signatures into first and last buckets
	sigMin := [64]byte{0x00, 0x00}
	sigMax := [64]byte{0xFF, 0xFF}
	wr.Put(sigMin)
	wr.Put(sigMax)

	_, err = wr.SealAndClose(indexmeta.Meta{})
	require.NoError(t, err)

	r, err := Open(path)
	require.NoError(t, err)
	defer r.Close()

	ok, err := r.Has(sigMin)
	require.NoError(t, err)
	require.True(t, ok, "min bucket failed")

	ok, err = r.Has(sigMax)
	require.NoError(t, err)
	require.True(t, ok, "max bucket failed")
}

func TestComprehensiveRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roundtrip.buck")
	wr, err := NewWriter(path)
	require.NoError(t, err)

	count := 1000
	sigs := make([][64]byte, count)
	for i := 0; i < count; i++ {
		_, _ = rand.Read(sigs[i][:])
		wr.Put(sigs[i])
	}

	_, err = wr.SealAndClose(indexmeta.Meta{})
	require.NoError(t, err)

	// Test both MMAP and standard OS reader
	readers := []string{"os", "mmap"}
	for _, mode := range readers {
		t.Run(mode, func(t *testing.T) {
			var r *Reader
			var err error
			if mode == "os" {
				r, err = Open(path)
			} else {
				r, err = OpenMMAP(path)
			}
			require.NoError(t, err)
			defer r.Close()

			for _, sig := range sigs {
				ok, err := r.Has(sig)
				require.NoError(t, err)
				require.True(t, ok)
			}

			// Random negative check
			absent := [64]byte{0xDE, 0xAD, 0xBE, 0xEF}
			ok, err := r.Has(absent)
			require.NoError(t, err)
			require.False(t, ok)
		})
	}
}

func TestCorruptFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.buck")

	setupFile := func(modifier func([]byte)) {
		wr, _ := NewWriter(path)
		_, _ = wr.SealAndClose(indexmeta.Meta{})
		data, _ := os.ReadFile(path)
		modifier(data)
		_ = os.WriteFile(path, data, 0o644)
	}

	t.Run("InvalidMagic", func(t *testing.T) {
		setupFile(func(b []byte) {
			copy(b[4:12], []byte("NOT_BUCK"))
		})
		_, err := Open(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid magic")
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		setupFile(func(b []byte) {
			binary.LittleEndian.PutUint64(b[12:20], 999)
		})
		_, err := Open(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "version mismatch")
	})
}

func TestConcurrentReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.buck")
	wr, _ := NewWriter(path)
	sig := [64]byte{0x1}
	wr.Put(sig)
	_, _ = wr.SealAndClose(indexmeta.Meta{})

	r, err := Open(path)
	require.NoError(t, err)
	defer r.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ok, err := r.Has(sig)
				if err != nil || !ok {
					fmt.Printf("Error: ok=%v, err=%v\n", ok, err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.buck")
	wr, _ := NewWriter(path)
	_, err := wr.SealAndClose(indexmeta.Meta{})
	require.NoError(t, err)

	r, err := Open(path)
	require.NoError(t, err)
	defer r.Close()

	ok, err := r.Has([64]byte{1})
	require.NoError(t, err)
	require.False(t, ok)
}
