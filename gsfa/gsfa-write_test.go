package gsfa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/require"
)

// TestGsfaWriterNoDataLossAtClose guards against a bug where the background
// fullBufferWriter dropped every batch still buffered in tmpBuf when it exited,
// silently losing a timing-dependent set of transactions from the index.
//
// We push exactly itemsPerBatch entries for several DISTINCT keys. Each key's
// batch is handed to the background writer (distinct keys, fewer than the 256
// flush threshold, no duplicate to force an early flush), so they all sit in
// tmpBuf when Close() sets the exit flag. Every entry must still be readable.
func TestGsfaWriterNoDataLossAtClose(t *testing.T) {
	dir := t.TempDir()
	rootCid := cid.MustParse("bafyreigywdbfkql2vnu3vodbmyhlhkfuegnkv5jorcilumi3gf6yo2jncy")

	w, err := NewGsfaWriter(dir, indexmeta.Meta{}, 0, rootCid, indexes.NetworkMainnet, t.TempDir())
	require.NoError(t, err)

	// Distinct keys; each gets one full itemsPerBatch batch (handed to the
	// background writer) plus a small remainder (flushed from accum at Close).
	const numKeys = 300
	const perKey = itemsPerBatch + 5
	keys := make([]solana.PublicKey, numKeys)
	for i := range keys {
		keys[i][0] = byte(i % 256)
		keys[i][1] = byte(i / 256)
		keys[i][2] = 1 // ensure non-zero
	}

	off := uint64(1)
	for _, key := range keys {
		for j := 0; j < perKey; j++ {
			require.NoError(t, w.Push(off, 1, 1 /*slot*/, solana.PublicKeySlice{key}, true, true, false))
			off++
		}
	}
	require.NoError(t, w.Close())

	r, err := NewGsfaReader(dir)
	require.NoError(t, err)
	defer r.Close()

	for i, key := range keys {
		locs, err := r.Get(context.Background(), key, perKey*2)
		require.NoErrorf(t, err, "key %d", i)
		require.Lenf(t, locs, perKey, "key %d (%s) lost entries", i, key)
	}
}

// TestGsfaWriterDeterministic verifies the gsfa index is byte-for-byte identical
// across re-runs from identical input, so downstream Filecoin piece CIDs are
// stable. It exercises both write paths: the background full-batch writer (hot
// keys exceeding itemsPerBatch) and the periodic accumulator drain (many small
// keys, with a lowered threshold), which previously interleaved
// non-deterministically when writing the linked log.
func TestGsfaWriterDeterministic(t *testing.T) {
	rootCid := cid.MustParse("bafyreigywdbfkql2vnu3vodbmyhlhkfuegnkv5jorcilumi3gf6yo2jncy")

	build := func(dir string) {
		w, err := NewGsfaWriter(dir, indexmeta.Meta{}, 0, rootCid, indexes.NetworkMainnet, t.TempDir())
		require.NoError(t, err)
		w.periodicFlushThreshold = 50 // trigger the periodic drain with few keys

		off := uint64(1)
		push := func(k solana.PublicKey) {
			// slot must be a multiple of 500 to reach the periodic-drain branch.
			require.NoError(t, w.Push(off, 1, 500, solana.PublicKeySlice{k}, true, true, false))
			off++
		}
		// Many single-entry keys drive the periodic drain (accum exceeds the
		// lowered threshold); a few hot keys pushed on every iteration accumulate
		// well past itemsPerBatch, driving the background full-batch writer. Both
		// run concurrently, which is exactly the interleaving that must stay
		// deterministic.
		for i := 0; i < 4000; i++ {
			var k solana.PublicKey
			k[0] = byte(i)
			k[1] = byte(i >> 8)
			k[2] = 1
			push(k)
			for h := 0; h < 3; h++ {
				var hot solana.PublicKey
				hot[0] = 0xff
				hot[1] = byte(h)
				push(hot) // 4000 entries each => 4 full itemsPerBatch batches
			}
		}
		require.NoError(t, w.Close())
	}

	sha := func(path string) string {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])
	}

	files := []string{"linked-log", string(indexes.Kind_PubkeyToOffsetAndSize) + ".index"}
	const runs = 4
	var ref map[string]string
	for r := 0; r < runs; r++ {
		dir := t.TempDir()
		build(dir)
		cur := map[string]string{}
		for _, f := range files {
			cur[f] = sha(filepath.Join(dir, f))
		}
		if ref == nil {
			ref = cur
			continue
		}
		for _, f := range files {
			require.Equalf(t, ref[f], cur[f], "run %d: %s differs (non-deterministic)", r, f)
		}
	}
}
