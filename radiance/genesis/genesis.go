package genesis

import (
	"time"

	"github.com/rpcpool/yellowstone-faithful/radiance/runtime"
)

// Genesis contains the genesis state of a Solana ledger.
type Genesis struct {
	CreationTime  time.Time
	Accounts      []AccountEntry
	Builtins      []BuiltinProgram
	RewardPools   []AccountEntry
	TicksPerSlot  uint64
	PohParams     runtime.PohParams
	Fees          runtime.FeeParams
	Rent          runtime.RentParams
	Inflation     runtime.InflationParams
	EpochSchedule runtime.EpochSchedule
	ClusterID     uint32
}

type AccountEntry struct {
	Pubkey [32]byte
	runtime.Account
}

type BuiltinProgram struct {
	Key    string
	Pubkey [32]byte
}

func (g *Genesis) FillAccounts(state runtime.Accounts) {
	for _, acc := range g.Accounts {
		state.SetAccount(&acc.Pubkey, &acc.Account)
	}
}
