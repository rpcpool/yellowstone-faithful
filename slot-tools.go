package main

// CalcEpochForSlot returns the epoch for the given slot.
func CalcEpochForSlot(slot uint64) uint64 {
	return slot / EpochLen
}

const EpochLen = 432000

// CalcEpochLimits returns the start and stop slots for the given epoch (inclusive).
func CalcEpochLimits(epoch uint64) (uint64, uint64) {
	epochStart := epoch * EpochLen
	epochStop := epochStart + EpochLen - 1
	return epochStart, epochStop
}

// Uint64RangesHavePartialOverlapIncludingEdges returns true if the two ranges have any overlap.
func Uint64RangesHavePartialOverlapIncludingEdges(r1 [2]uint64, r2 [2]uint64) bool {
	if r1[0] < r2[0] {
		return r1[1] >= r2[0]
	} else {
		return r2[1] >= r1[0]
	}
}
