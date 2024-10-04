package gsfa

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestPopRank(t *testing.T) {
	// Test the rollingRankOfTopPerformers type:
	{
		// Create a new rollingRankOfTopPerformers:
		r := newRollingRankOfTopPerformers(5)
		if r == nil {
			t.Fatal("expected non-nil rollingRankOfTopPerformers")
		}
		// Test the Incr method:
		{
			key := solana.SysVarRentPubkey
			delta := 1
			value := r.Incr(key, delta)
			require.Equal(t, 1, value)
		}
		// Test the purge method:
		{
			r.purge()
			// the value should still be 1
			value, ok := r.Get(solana.SysVarRentPubkey)
			require.True(t, ok)
			require.Equal(t, 1, value)
		}
		{
			// now add a few more values:
			r.Incr(solana.SysVarClockPubkey, 6)
			r.Incr(solana.SysVarEpochSchedulePubkey, 5)
			r.Incr(solana.SysVarFeesPubkey, 4)
			r.Incr(solana.SysVarInstructionsPubkey, 3)
			r.Incr(solana.SysVarRewardsPubkey, 2)

			// there should be 6 values now
			require.Equal(t, 6, r.set.Len())

			// purge should remove the lowest values
			r.purge()

			// there should be 5 values now (equivalent values are kept)
			require.Equal(t, 5, r.set.Len())

			// the lowest value should be 2
			require.Equal(t, 2, r.minValue)
			require.Equal(t, 6, r.maxValue)
		}
	}
}
