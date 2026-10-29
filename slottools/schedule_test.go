package slottools

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// useEpochSchedule sets the process-wide schedule for the duration of the test.
func useEpochSchedule(t *testing.T, s EpochSchedule) {
	t.Helper()
	prev := CurrentEpochSchedule()
	SetEpochSchedule(s)
	t.Cleanup(func() { SetEpochSchedule(prev) })
}

func TestDefaultEpochScheduleIsMainnet(t *testing.T) {
	require.Equal(t, MainnetEpochSchedule, CurrentEpochSchedule())
}

func TestMainnetEpochScheduleEquivalence(t *testing.T) {
	for _, s := range []EpochSchedule{MainnetEpochSchedule, DevnetEpochSchedule} {
		slots := []uint64{0, 1, 431999, 432000, 432001, 863999, 864000}
		rng := rand.New(rand.NewSource(1))
		for range 10_000 {
			slots = append(slots, rng.Uint64()%(1<<40))
		}
		for _, slot := range slots {
			require.Equal(t, slot/432000, s.EpochForSlot(slot), "slot %d", slot)
		}
		for epoch := uint64(0); epoch <= 2000; epoch++ {
			start, end := s.EpochLimits(epoch)
			require.Equal(t, epoch*432000, start, "epoch %d", epoch)
			require.Equal(t, epoch*432000+431999, end, "epoch %d", epoch)
			require.Equal(t, uint64(432000), s.SlotsInEpoch(epoch))
		}
	}
}

func TestTestnetEpochSchedule(t *testing.T) {
	s := TestnetEpochSchedule

	require.Equal(t, uint64(444620256), s.FirstSlotInEpoch(1042))
	require.Equal(t, uint64(1042), s.EpochForSlot(444620256))
	require.Equal(t, uint64(1041), s.EpochForSlot(444620255))
	require.Equal(t, uint64(1042), s.EpochForSlot(444625260))

	require.Equal(t, uint64(524256), s.FirstSlotInEpoch(14))
	require.Equal(t, uint64(13), s.EpochForSlot(524255))
	require.Equal(t, uint64(14), s.EpochForSlot(524256))

	require.Equal(t, uint64(262112), s.FirstSlotInEpoch(13))
	require.Equal(t, uint64(262144), s.SlotsInEpoch(13))
	require.Equal(t, uint64(432000), s.SlotsInEpoch(14))

	require.Equal(t, uint64(0), s.EpochForSlot(0))
	require.Equal(t, uint64(0), s.EpochForSlot(31))
	require.Equal(t, uint64(1), s.EpochForSlot(32))
	{
		start, end := s.EpochLimits(0)
		require.Equal(t, uint64(0), start)
		require.Equal(t, uint64(31), end)
	}
	{
		start, end := s.EpochLimits(1)
		require.Equal(t, uint64(32), start)
		require.Equal(t, uint64(95), end)
	}
}

func TestTestnetEpochScheduleRoundTrip(t *testing.T) {
	s := TestnetEpochSchedule
	for epoch := uint64(0); epoch <= 1100; epoch++ {
		first := s.FirstSlotInEpoch(epoch)
		require.Equal(t, epoch, s.EpochForSlot(first), "epoch %d", epoch)
		_, last := s.EpochLimits(epoch)
		require.Equal(t, epoch, s.EpochForSlot(last), "epoch %d", epoch)
		require.Equal(t, epoch+1, s.EpochForSlot(last+1), "epoch %d", epoch)
		if epoch > 0 {
			require.Equal(t, epoch-1, s.EpochForSlot(first-1), "epoch %d", epoch)
		}
	}
}

func TestPackageFunctionsFollowEpochSchedule(t *testing.T) {
	useEpochSchedule(t, TestnetEpochSchedule)

	require.Equal(t, uint64(1042), CalcEpochForSlot(444620256))
	require.Equal(t, uint64(1042), EpochForSlot(444620256))
	start, end := CalcEpochLimits(1042)
	require.Equal(t, uint64(444620256), start)
	require.Equal(t, uint64(444620256+431999), end)

	require.True(t, ParentIsInPreviousEpoch(444620255, 444620256))
	require.False(t, ParentIsInPreviousEpoch(444620256, 444620257))
	// With the mainnet schedule both slots are in the same epoch.
	require.Equal(t, MainnetEpochSchedule.EpochForSlot(444620255), MainnetEpochSchedule.EpochForSlot(444620256))

	require.Equal(t, []uint64{1041, 1042}, CalcEpochsForSlotRange(444620255, 444620256))

	require.NoError(t, ValidateSlotInEpoch(444620256, 1042))
	require.Error(t, ValidateSlotInEpoch(444620255, 1042))
}

func TestEpochScheduleForNetwork(t *testing.T) {
	for network, want := range map[string]EpochSchedule{
		"mainnet": MainnetEpochSchedule,
		"devnet":  DevnetEpochSchedule,
		"testnet": TestnetEpochSchedule,
	} {
		got, err := EpochScheduleForNetwork(network)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	_, err := EpochScheduleForNetwork("mainnet-beta")
	require.Error(t, err)
}
