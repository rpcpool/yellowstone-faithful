package slottools

import (
	"fmt"
	"math/bits"
	"sync/atomic"
)

// minimumSlotsPerEpoch is the length of the first warmup epoch (Agave's MINIMUM_SLOTS_PER_EPOCH).
const minimumSlotsPerEpoch = 32

// EpochSchedule mirrors Agave's solana-epoch-schedule.
// With warmup, epoch e < FirstNormalEpoch is (32 << e) slots long;
// after that every epoch is SlotsPerEpoch slots long.
type EpochSchedule struct {
	SlotsPerEpoch    uint64
	FirstNormalEpoch uint64
	FirstNormalSlot  uint64
}

var (
	MainnetEpochSchedule = EpochSchedule{SlotsPerEpoch: 432000}
	DevnetEpochSchedule  = EpochSchedule{SlotsPerEpoch: 432000}
	TestnetEpochSchedule = EpochSchedule{SlotsPerEpoch: 432000, FirstNormalEpoch: 14, FirstNormalSlot: 524256}
)

func (s EpochSchedule) String() string {
	return fmt.Sprintf("slotsPerEpoch=%d firstNormalEpoch=%d firstNormalSlot=%d", s.SlotsPerEpoch, s.FirstNormalEpoch, s.FirstNormalSlot)
}

// FirstSlotInEpoch returns the first slot of the given epoch.
func (s EpochSchedule) FirstSlotInEpoch(epoch uint64) uint64 {
	if epoch <= s.FirstNormalEpoch {
		return (1<<epoch - 1) * minimumSlotsPerEpoch
	}
	return (epoch-s.FirstNormalEpoch)*s.SlotsPerEpoch + s.FirstNormalSlot
}

// SlotsInEpoch returns the number of slots in the given epoch.
func (s EpochSchedule) SlotsInEpoch(epoch uint64) uint64 {
	if epoch < s.FirstNormalEpoch {
		return minimumSlotsPerEpoch << epoch
	}
	return s.SlotsPerEpoch
}

// EpochForSlot returns the epoch that contains the given slot.
func (s EpochSchedule) EpochForSlot(slot uint64) uint64 {
	if slot < s.FirstNormalSlot {
		// warmup epoch e spans [(2^e-1)*32, (2^(e+1)-1)*32)
		return uint64(bits.Len64(slot/minimumSlotsPerEpoch+1)) - 1
	}
	return s.FirstNormalEpoch + (slot-s.FirstNormalSlot)/s.SlotsPerEpoch
}

// EpochLimits returns the first and last slot of the epoch (inclusive).
func (s EpochSchedule) EpochLimits(epoch uint64) (uint64, uint64) {
	start := s.FirstSlotInEpoch(epoch)
	return start, start + s.SlotsInEpoch(epoch) - 1
}

var currentEpochSchedule atomic.Pointer[EpochSchedule]

func init() {
	SetEpochSchedule(MainnetEpochSchedule)
}

// SetEpochSchedule sets the process-wide epoch schedule used by the package-level
// functions (CalcEpochForSlot, CalcEpochLimits, ...). The default is mainnet.
// It should only be called at startup, before anything does slot/epoch math.
func SetEpochSchedule(s EpochSchedule) {
	currentEpochSchedule.Store(&s)
}

// CurrentEpochSchedule returns the process-wide epoch schedule.
func CurrentEpochSchedule() EpochSchedule {
	return *currentEpochSchedule.Load()
}

// EpochScheduleForNetwork returns the epoch schedule of the named cluster
// ("mainnet", "devnet" or "testnet").
func EpochScheduleForNetwork(network string) (EpochSchedule, error) {
	switch network {
	case "mainnet":
		return MainnetEpochSchedule, nil
	case "devnet":
		return DevnetEpochSchedule, nil
	case "testnet":
		return TestnetEpochSchedule, nil
	default:
		return EpochSchedule{}, fmt.Errorf("unknown network %q", network)
	}
}
