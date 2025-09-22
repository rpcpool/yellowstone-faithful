package slottools

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalcEpochForSlot(t *testing.T) {
	require.Equal(t, uint64(0), CalcEpochForSlot(0))
	require.Equal(t, uint64(0), CalcEpochForSlot(1))
	require.Equal(t, uint64(0), CalcEpochForSlot(431999))
	require.Equal(t, uint64(1), CalcEpochForSlot(432000))
	require.Equal(t, uint64(1), CalcEpochForSlot(863999))
	require.Equal(t, uint64(2), CalcEpochForSlot(864000))
	require.Equal(t, uint64(477), CalcEpochForSlot(206459118))
}

func TestCalcEpochLimits(t *testing.T) {
	{
		epochStart, epochStop := CalcEpochLimits(0)
		require.Equal(t, uint64(0), epochStart)
		require.Equal(t, uint64(431_999), epochStop)
	}
	{
		epochStart, epochStop := CalcEpochLimits(1)
		require.Equal(t, uint64(432_000), epochStart)
		require.Equal(t, uint64(863_999), epochStop)
	}
	{
		epochStart, epochStop := CalcEpochLimits(333)
		require.Equal(t, uint64(143_856_000), epochStart)
		require.Equal(t, uint64(144_287_999), epochStop)
	}
	{
		epochStart, epochStop := CalcEpochLimits(447)
		require.Equal(t, uint64(193_104_000), epochStart)
		require.Equal(t, uint64(193_535_999), epochStop)
	}
}

func TestUint64RangesHavePartialOverlapIncludingEdges(t *testing.T) {
	{
		r1 := [2]uint64{0, 10}
		r2 := [2]uint64{5, 15}
		require.True(t, Uint64RangesHavePartialOverlapIncludingEdges(r1, r2))
	}
	{
		r1 := [2]uint64{0, 10}
		r2 := [2]uint64{10, 15}
		require.True(t, Uint64RangesHavePartialOverlapIncludingEdges(r1, r2))
	}
	{
		r1 := [2]uint64{0, 10}
		r2 := [2]uint64{11, 15}
		require.False(t, Uint64RangesHavePartialOverlapIncludingEdges(r1, r2))
	}
	{
		r1 := [2]uint64{0, 10}
		r2 := [2]uint64{0, 10}
		require.True(t, Uint64RangesHavePartialOverlapIncludingEdges(r1, r2))
	}
	{
		r1 := [2]uint64{10, 20}
		r2 := [2]uint64{0, 10}
		require.True(t, Uint64RangesHavePartialOverlapIncludingEdges(r1, r2))
	}
}

func TestParentIsInPreviousEpoch(t *testing.T) {
	require.False(t, ParentIsInPreviousEpoch(320975998, 320975999))
	require.True(t, ParentIsInPreviousEpoch(320543999, 320544000))
	require.True(t, ParentIsInPreviousEpoch(431998, 432000))
	require.False(t, ParentIsInPreviousEpoch(0, 1))
}

func TestRanges(t *testing.T) {
	got := calcRangeInclusive(0, 5)
	if !reflect.DeepEqual(got, []uint64{0, 1, 2, 3, 4, 5}) {
		panic(fmt.Sprintf("calcRangeInclusive(0, 5) = %v, want [0 1 2 3 4 5]", got))
	}
	got = calcRangeInclusive(5, 0)
	if !reflect.DeepEqual(got, []uint64{0, 1, 2, 3, 4, 5}) {
		panic(fmt.Sprintf("calcRangeInclusive(5, 0) = %v, want [0 1 2 3 4 5]", got))
	}
	got = calcRangeInclusive(3, 3)
	if !reflect.DeepEqual(got, []uint64{3}) {
		panic(fmt.Sprintf("calcRangeInclusive(3, 3) = %v, want [3]", got))
	}
	got = calcRangeInclusive(0, 0)
	if !reflect.DeepEqual(got, []uint64{0}) {
		panic(fmt.Sprintf("calcRangeInclusive(0, 0) = %v, want [0]", got))
	}
	got = calcRangeInclusive(10, 15)
	if !reflect.DeepEqual(got, []uint64{10, 11, 12, 13, 14, 15}) {
		panic(fmt.Sprintf("calcRangeInclusive(10, 15) = %v, want [10 11 12 13 14 15]", got))
	}
	got = calcRangeInclusive(15, 10)
	if !reflect.DeepEqual(got, []uint64{10, 11, 12, 13, 14, 15}) {
		panic(fmt.Sprintf("calcRangeInclusive(15, 10) = %v, want [10 11 12 13 14 15]", got))
	}
}

func TestEpochRange(t *testing.T) {
	{
		gotEpochs := CalcEpochsForSlotRange(0, 431999)
		wantEpochs := []uint64{0}
		if !reflect.DeepEqual(gotEpochs, wantEpochs) {
			t.Errorf("CalcEpochsForSlotRange(0, 431999) = %v, want %v", gotEpochs, wantEpochs)
		}
	}
	{
		gotEpochs := CalcEpochsForSlotRange(432000, 863999)
		wantEpochs := []uint64{1}
		if !reflect.DeepEqual(gotEpochs, wantEpochs) {
			t.Errorf("CalcEpochsForSlotRange(432000, 863999) = %v, want %v", gotEpochs, wantEpochs)
		}
	}
	{
		gotEpochs := CalcEpochsForSlotRange(343169187, 345302213)
		wantEpochs := []uint64{
			794, 795, 796, 797, 798, 799,
		}
		require.Equal(t, wantEpochs, gotEpochs, "CalcEpochsForSlotRange(343169187, 345302213) should return the expected epochs")
	}
}
