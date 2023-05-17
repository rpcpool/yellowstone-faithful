package compactindex

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaxCls64(t *testing.T) {
	cases := [][2]uint64{
		{0x0000_0000_0000_0000, 0x0000_0000_0000_0000},
		{0x0000_0000_0000_0001, 0x0000_0000_0000_0001},
		{0x0000_0000_0000_0003, 0x0000_0000_0000_0002},
		{0x0000_0000_0000_0003, 0x0000_0000_0000_0003},
		{0x0000_0000_0000_0007, 0x0000_0000_0000_0004},
		{0x0000_0000_FFFF_FFFF, 0x0000_0000_F000_000F},
		{0xFFFF_FFFF_FFFF_FFFF, 0xFFFF_FFFF_FFFF_FFFF},
	}
	for _, tc := range cases {
		assert.Equal(t, tc[0], maxCls64(tc[1]))
	}
}

func TestHeader_BucketHash(t *testing.T) {
	const numItems = 500000
	const numBuckets = 1000

	header := Header{
		NumBuckets: numBuckets,
	}

	keys := make([][]byte, numItems)
	hits := make([]int, numBuckets)
	for i := range keys {
		var buf [16]byte
		n, _ := rand.Read(buf[:])
		keys[i] = buf[:n]
	}

	// Bounds check and count hits.
	for _, key := range keys {
		idx := header.BucketHash(key)
		require.True(t, idx < numBuckets)
		hits[idx]++
	}

	// Calculate standard deviation.
	mean := float64(numItems) / float64(numBuckets)
	var cumVariance float64
	for _, bucketHits := range hits {
		delta := float64(bucketHits) - mean
		cumVariance += (delta * delta)
	}
	variance := cumVariance / float64(len(hits))
	stddev := math.Sqrt(variance)
	t.Logf("mean % 12.2f", mean)
	normStddev := stddev / mean
	t.Logf("stddev % 10.2f", stddev)
	t.Logf("1σ / mean % 7.2f%%", 100*normStddev)

	const failNormStddev = 0.08
	if normStddev > failNormStddev {
		t.Logf("FAIL: > %f%%", 100*failNormStddev)
		t.Fail()
	} else {
		t.Logf("   OK: <= %f%%", 100*failNormStddev)
	}

	// Print percentiles.
	sort.Ints(hits)
	t.Logf("min % 10d", hits[0])
	t.Logf("p01 % 10d", hits[int(math.Round(0.01*float64(len(hits))))])
	t.Logf("p05 % 10d", hits[int(math.Round(0.05*float64(len(hits))))])
	t.Logf("p10 % 10d", hits[int(math.Round(0.10*float64(len(hits))))])
	t.Logf("p50 % 10d", hits[int(math.Round(0.50*float64(len(hits))))])
	t.Logf("p90 % 10d", hits[int(math.Round(0.90*float64(len(hits))))])
	t.Logf("p95 % 10d", hits[int(math.Round(0.95*float64(len(hits))))])
	t.Logf("p99 % 10d", hits[int(math.Round(0.99*float64(len(hits))))])
	t.Logf("max % 10d", hits[len(hits)-1])
}
