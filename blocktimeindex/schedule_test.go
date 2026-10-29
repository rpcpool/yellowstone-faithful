package blocktimeindex

import (
	"errors"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/slottools"
	"github.com/stretchr/testify/require"
)

func useEpochSchedule(t *testing.T, s slottools.EpochSchedule) {
	t.Helper()
	prev := slottools.CurrentEpochSchedule()
	slottools.SetEpochSchedule(s)
	t.Cleanup(func() { slottools.SetEpochSchedule(prev) })
}

func TestIndexByteSizeForEpochMainnet(t *testing.T) {
	require.Equal(t, DefaultIndexByteSize, IndexByteSizeForEpoch(0))
	require.Equal(t, DefaultIndexByteSize, IndexByteSizeForEpoch(800))
}

func TestWriterTestnet(t *testing.T) {
	useEpochSchedule(t, slottools.TestnetEpochSchedule)

	t.Run("normal epoch", func(t *testing.T) {
		const first = uint64(444_620_256)
		const last = first + 431_999
		w := NewForEpoch(1042)
		require.Equal(t, first, w.start)
		require.Equal(t, last, w.end)
		require.Equal(t, uint64(1042), w.epoch)
		require.Equal(t, uint64(432_000), w.capacity)

		require.NoError(t, w.Set(first, 1))
		require.NoError(t, w.Set(last, 2))
		for _, slot := range []uint64{first - 1, last + 1, 1042 * 432_000} {
			err := w.Set(slot, 3)
			require.True(t, errors.Is(err, &ErrSlotOutOfRange{}), "slot %d: %v", slot, err)
		}

		buf, err := w.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, IndexByteSizeForEpoch(1042), len(buf))

		got, err := FromBytes(buf)
		require.NoError(t, err)
		require.Equal(t, uint64(1042), got.Epoch())
		v, err := got.Get(first)
		require.NoError(t, err)
		require.Equal(t, int64(1), v)
		v, err = got.Get(last)
		require.NoError(t, err)
		require.Equal(t, int64(2), v)
	})

	t.Run("warmup epoch", func(t *testing.T) {
		w := NewForEpoch(3)
		require.Equal(t, uint64(224), w.start)
		require.Equal(t, uint64(479), w.end)
		require.Equal(t, uint64(256), w.capacity)
		require.Len(t, w.values, 256)
		require.NoError(t, w.Set(479, 7))

		buf, err := w.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, IndexByteSizeForEpoch(3), len(buf))
		require.Less(t, len(buf), DefaultIndexByteSize)

		got, err := FromBytes(buf)
		require.NoError(t, err)
		v, err := got.Get(479)
		require.NoError(t, err)
		require.Equal(t, int64(7), v)
	})
}
