package bucketteer

import (
	"sort"

	"github.com/cespare/xxhash/v2"
)

var _Magic = [8]byte{'b', 'u', 'c', 'k', 'e', 't', 't', 'e'}

func Magic() [8]byte {
	return _Magic
}

const Version = uint64(2)

func sortWithCompare[T any](a []T, compare func(i, j int) int) {
	sort.Slice(a, func(i, j int) bool {
		return compare(i, j) < 0
	})
	sorted := make([]T, len(a))
	eytzinger(a, sorted, 0, 1)
	copy(a, sorted)
}

func eytzinger[T any](in, out []T, i, k int) int {
	if k <= len(in) {
		i = eytzinger(in, out, i, 2*k)
		out[k-1] = in[i]
		i++
		i = eytzinger(in, out, i, 2*k+1)
	}
	return i
}

func Hash(sig [64]byte) uint64 {
	return xxhash.Sum64(sig[:])
}
