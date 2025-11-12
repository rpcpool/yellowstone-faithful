package slottools

import (
	"encoding/binary"
	"fmt"
)

// CalcEpochForSlot returns the epoch for the given slot.
func CalcEpochForSlot(slot uint64) uint64 {
	return CalcEpochForSlotWithEpochLen(slot, DEFAULT_EPOCH_LEN)
}

func CalcEpochForSlotWithEpochLen(slot uint64, epochLen uint64) uint64 {
	return slot / epochLen
}

const DEFAULT_EPOCH_LEN = 432000

// CalcEpochLimits returns the start and stop slots for the given epoch (inclusive).
func CalcEpochLimits(epoch uint64) (uint64, uint64) {
	return CalcEpochLimitsWithEpochLen(epoch, DEFAULT_EPOCH_LEN)
}

func CalcEpochLimitsWithEpochLen(epoch uint64, epochLen uint64) (uint64, uint64) {
	epochStart := epoch * epochLen
	epochStop := epochStart + epochLen - 1
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

// EpochForSlot returns the epoch for the given slot.
func EpochForSlot(slot uint64) uint64 {
	return CalcEpochForSlot(slot)
}

// EpochLimits returns the start and stop slots for the given epoch (inclusive).
func EpochLimits(epoch uint64) (uint64, uint64) {
	return CalcEpochLimits(epoch)
}

func Uint64ToLEBytes(v uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf
}

func Uint64FromLEBytes(buf []byte) uint64 {
	return binary.LittleEndian.Uint64(buf)
}

func ParentIsInPreviousEpoch(parentSlot uint64, childSlot uint64) bool {
	// If the parent slot is less than the start of the current epoch,
	// then it must be in the previous epoch.
	epochStart := CalcEpochForSlot(childSlot) * DEFAULT_EPOCH_LEN
	return parentSlot < epochStart
}

func CalcEpochsForSlotRange(startSlot, endSlot uint64) []uint64 {
	epochStart := CalcEpochForSlot(startSlot)
	epochEnd := CalcEpochForSlot(endSlot)
	return calcRangeInclusive(epochStart, epochEnd)
}

func calcRangeInclusive(start, end uint64) []uint64 {
	if start == end {
		return []uint64{start} // if start and end are the same, return a slice with that single value
	}
	if start > end {
		end, start = start, end // ensure start is less than or equal to end
	}
	rangeSlice := make([]uint64, end-start+1)
	for i := range rangeSlice {
		rangeSlice[i] = start + uint64(i)
	}
	return rangeSlice
}

// ValidateSlotInEpoch checks if a slot belongs to the expected epoch.
func ValidateSlotInEpoch(slot, epoch, epochLen uint64) error {
	if slotEpoch := CalcEpochForSlotWithEpochLen(slot, epochLen); epoch != slotEpoch {
		return fmt.Errorf("provided slot %v belongs to epoch %d (expected epoch %d)", slot, slotEpoch, epoch)
	}
	return nil
}
