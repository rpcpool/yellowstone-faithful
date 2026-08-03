package gsfa

import (
	"slices"
	"sort"

	"github.com/gagliardetto/solana-go"
	"github.com/tidwall/hashmap"
)

type rollingRankOfTopPerformers struct {
	rankListSize int
	maxValue     int
	minValue     int
	set          hashmap.Map[solana.PublicKey, int]
}

func newRollingRankOfTopPerformers(rankListSize int) *rollingRankOfTopPerformers {
	return &rollingRankOfTopPerformers{
		rankListSize: rankListSize,
	}
}

func (r *rollingRankOfTopPerformers) Incr(key solana.PublicKey, delta int) int {
	value, ok := r.set.Get(key)
	if !ok {
		value = 0
	}
	value = value + delta
	r.set.Set(key, value)
	if value > r.maxValue {
		r.maxValue = value
	}
	if value < r.minValue {
		r.minValue = value
	}
	return value
}

func (r *rollingRankOfTopPerformers) Get(key solana.PublicKey) (int, bool) {
	value, ok := r.set.Get(key)
	return value, ok
}

// purge will remove all keys by the lowest values until the rankListSize is reached.
// keys with equivalent values are kept.
func (r *rollingRankOfTopPerformers) purge() {
	values := r.set.Values()
	sort.Ints(values)
	values = slices.Compact(values)
	if len(values) <= r.rankListSize {
		return
	}

	// Keep only keys whose value is within the top rankListSize distinct values;
	// everything below the threshold is removed. This is a single O(N) pass over
	// the keys (plus the O(N log N) sort above) rather than the previous
	// O(distinctValues * N) nested scan, which grew super-linearly over an epoch
	// as hot accounts accumulated large, widely-distributed counts.
	threshold := values[len(values)-r.rankListSize]
	for _, key := range r.set.Keys() {
		if v, _ := r.set.Get(key); v < threshold {
			r.set.Delete(key)
		}
	}

	// update the min and max values
	r.minValue = threshold
	r.maxValue = values[len(values)-1]
}

func (r *rollingRankOfTopPerformers) has(key solana.PublicKey) bool {
	_, ok := r.set.Get(key)
	return ok
}
