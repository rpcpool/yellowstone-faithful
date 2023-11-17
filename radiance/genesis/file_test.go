package genesis

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/radiance/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadGenesisFromArchive(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "mainnet", "genesis.tar.bz2"))
	require.NoError(t, err)
	defer f.Close()
	genesis, _, err := ReadGenesisFromArchive(f)
	require.NoError(t, err)

	assert.Equal(t, time.Date(2020, time.March, 16, 14, 29, 0, 0, time.UTC), genesis.CreationTime)
	assert.Equal(t, int64(1584368940), genesis.CreationTime.Unix())
	assert.Equal(t, []BuiltinProgram{
		{
			Key:    "solana_config_program",
			Pubkey: solana.MustPublicKeyFromBase58("Config1111111111111111111111111111111111111"),
		},
		{
			Key:    "solana_stake_program",
			Pubkey: solana.MustPublicKeyFromBase58("Stake11111111111111111111111111111111111111"),
		},
		{
			Key:    "solana_system_program",
			Pubkey: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		},
		{
			Key:    "solana_vote_program",
			Pubkey: solana.MustPublicKeyFromBase58("Vote111111111111111111111111111111111111111"),
		},
	}, genesis.Builtins)
	assert.Equal(t, uint64(0x40), genesis.TicksPerSlot)
	assert.Equal(t, runtime.PohParams{
		TickDuration:     6250000,
		HasHashesPerTick: true,
		HashesPerTick:    12500,
		HasTickCount:     false,
	}, genesis.PohParams)
	assert.Equal(t, runtime.FeeParams{
		TargetLamportsPerSig: 10000,
		TargetSigsPerSlot:    20000,
		MinLamportsPerSig:    5000,
		MaxLamportsPerSig:    100000,
		BurnPercent:          100,
	}, genesis.Fees)
	assert.Equal(t, runtime.RentParams{
		LamportsPerByteYear: 3480,
		ExemptionThreshold:  2,
		BurnPercent:         100,
	}, genesis.Rent)
	assert.Equal(t, runtime.InflationParams{ /* empty */ }, genesis.Inflation)
	assert.Equal(t, runtime.EpochSchedule{
		SlotPerEpoch:             432000,
		LeaderScheduleSlotOffset: 432000,
	}, genesis.EpochSchedule)
	assert.Equal(t, uint32(1), genesis.ClusterID)
}
