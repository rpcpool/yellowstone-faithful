package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_findSlotIndexBinarySearch(t *testing.T) {
	tests := []struct {
		name     string
		slot     uint64
		blocks   []uint64
		expected int
	}{
		{
			name:     "exact match at start",
			slot:     10,
			blocks:   []uint64{10, 20, 30, 40, 50},
			expected: 0,
		},
		{
			name:     "exact match in middle",
			slot:     30,
			blocks:   []uint64{10, 20, 30, 40, 50},
			expected: 2,
		},
		{
			name:     "exact match at end",
			slot:     50,
			blocks:   []uint64{10, 20, 30, 40, 50},
			expected: 4,
		},
		{
			name:     "slot below all entries returns 0",
			slot:     5,
			blocks:   []uint64{10, 20, 30},
			expected: 0,
		},
		{
			name:     "slot above all entries returns len(blocks)",
			slot:     100,
			blocks:   []uint64{10, 20, 30},
			expected: 3,
		},
		{
			name:     "slot between entries returns insertion point",
			slot:     25,
			blocks:   []uint64{10, 20, 30, 40},
			expected: 2,
		},
		{
			name:     "empty slice returns 0",
			slot:     42,
			blocks:   []uint64{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSlotIndexBinarySearch(tt.slot, tt.blocks)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_getBlocks_slicing(t *testing.T) {
	tests := []struct {
		name      string
		blocks    []uint64
		startSlot uint64
		endSlot   uint64
		expected  []uint64
	}{
		{
			name:      "exact range match",
			blocks:    []uint64{100, 200, 300, 400, 500},
			startSlot: 200,
			endSlot:   400,
			expected:  []uint64{200, 300, 400},
		},
		{
			name:      "start below first block",
			blocks:    []uint64{100, 200, 300},
			startSlot: 50,
			endSlot:   200,
			expected:  []uint64{100, 200},
		},
		{
			name:      "end above last block",
			blocks:    []uint64{100, 200, 300},
			startSlot: 200,
			endSlot:   999,
			expected:  []uint64{200, 300},
		},
		{
			name:      "range with no confirmed blocks",
			blocks:    []uint64{100, 200, 300},
			startSlot: 150,
			endSlot:   180,
			expected:  []uint64{},
		},
		{
			name:      "single block in range",
			blocks:    []uint64{100, 200, 300},
			startSlot: 200,
			endSlot:   200,
			expected:  []uint64{200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startIdx := findSlotIndexBinarySearch(tt.startSlot, tt.blocks)
			endIdx := findSlotIndexBinarySearch(tt.endSlot, tt.blocks)
			// Include endSlot if it exists in the list.
			if endIdx < len(tt.blocks) && tt.blocks[endIdx] == tt.endSlot {
				endIdx++
			}
			got := tt.blocks[startIdx:endIdx]
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseGetBlocksWithLimitRequest(t *testing.T) {
	tests := []struct {
		name       string
		params     string
		wantStart  uint64
		wantLimit  uint64
		wantCommit string
		wantErr    bool
	}{
		{
			name:       "valid startSlot and limit",
			params:     `[100, 50]`,
			wantStart:  100,
			wantLimit:  50,
			wantCommit: "",
		},
		{
			name:       "valid with commitment",
			params:     `[200, 10, {"commitment": "finalized"}]`,
			wantStart:  200,
			wantLimit:  10,
			wantCommit: "finalized",
		},
		{
			name:    "missing limit",
			params:  `[100]`,
			wantErr: true,
		},
		{
			name:    "wrong type for startSlot",
			params:  `["abc", 10]`,
			wantErr: true,
		},
		{
			name:    "wrong type for limit",
			params:  `[100, "ten"]`,
			wantErr: true,
		},
		{
			name:    "options not an object",
			params:  `[100, 10, "finalized"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(tt.params)
			got, err := parseGetBlocksWithLimitRequest(&raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, got.StartSlot)
			assert.Equal(t, tt.wantLimit, got.Limit)
			assert.Equal(t, tt.wantCommit, got.Commitment)
		})
	}
}

func Test_GetBlocksWithLimitRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     GetBlocksWithLimitRequest
		wantErr bool
	}{
		{
			name:    "valid limit",
			req:     GetBlocksWithLimitRequest{StartSlot: 0, Limit: 100},
			wantErr: false,
		},
		{
			name:    "limit at max",
			req:     GetBlocksWithLimitRequest{StartSlot: 0, Limit: GetBlocksWithLimitMaxLimit},
			wantErr: false,
		},
		{
			name:    "limit zero",
			req:     GetBlocksWithLimitRequest{StartSlot: 0, Limit: 0},
			wantErr: true,
		},
		{
			name:    "limit exceeds max",
			req:     GetBlocksWithLimitRequest{StartSlot: 0, Limit: GetBlocksWithLimitMaxLimit + 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func newTestMultiEpoch(epochBlocks map[uint64][]uint64) *MultiEpoch {
	epochs := make(map[uint64]*Epoch, len(epochBlocks))
	for epochNum, blocks := range epochBlocks {
		epochs[epochNum] = &Epoch{epoch: epochNum, blocks: blocks}
	}
	return &MultiEpoch{epochs: epochs}
}

func Test_getBlocksInRange(t *testing.T) {
	// Epoch length is 432000 slots. Epoch 0: 0–431999, Epoch 1: 432000–863999, etc.
	const epochLen = uint64(432000)

	tests := []struct {
		name        string
		epochBlocks map[uint64][]uint64 // epoch number -> sorted confirmed block slots
		startSlot   uint64
		endSlot     uint64
		expected    []uint64
		wantErr     bool
	}{
		// --- Same-epoch cases ---
		{
			name:        "same epoch: end slot exists and is inclusive",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300, 400, 500}},
			startSlot:   200,
			endSlot:     400,
			expected:    []uint64{200, 300, 400},
		},
		{
			name:        "same epoch: end slot is not a confirmed block (skipped slot)",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300, 400, 500}},
			startSlot:   200,
			endSlot:     350,
			expected:    []uint64{200, 300},
		},
		{
			name:        "same epoch: end slot is the last block in the epoch",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300}},
			startSlot:   100,
			endSlot:     300,
			expected:    []uint64{100, 200, 300},
		},
		{
			name:        "same epoch: start and end are the same confirmed slot",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300}},
			startSlot:   200,
			endSlot:     200,
			expected:    []uint64{200},
		},
		{
			name:        "same epoch: end slot exceeds all blocks in epoch",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300}},
			startSlot:   200,
			endSlot:     999,
			expected:    []uint64{200, 300},
		},
		{
			name:        "same epoch: no confirmed blocks in range",
			epochBlocks: map[uint64][]uint64{0: {100, 200, 300}},
			startSlot:   150,
			endSlot:     180,
			expected:    []uint64{},
		},
		// --- Multi-epoch cases ---
		{
			name: "two epochs: end slot exists in second epoch and is inclusive",
			epochBlocks: map[uint64][]uint64{
				0: {epochLen - 2, epochLen - 1},
				1: {epochLen, epochLen + 1, epochLen + 2},
			},
			startSlot: epochLen - 2,
			endSlot:   epochLen + 2,
			expected:  []uint64{epochLen - 2, epochLen - 1, epochLen, epochLen + 1, epochLen + 2},
		},
		{
			name: "two epochs: end slot is not a confirmed block in second epoch",
			epochBlocks: map[uint64][]uint64{
				0: {epochLen - 2, epochLen - 1},
				1: {epochLen, epochLen + 2, epochLen + 4},
			},
			startSlot: epochLen - 2,
			endSlot:   epochLen + 3,
			expected:  []uint64{epochLen - 2, epochLen - 1, epochLen, epochLen + 2},
		},
		{
			name: "two epochs: end slot is the last block in the second epoch's list",
			epochBlocks: map[uint64][]uint64{
				0: {epochLen - 1},
				1: {epochLen, epochLen + 1, epochLen + 2},
			},
			startSlot: epochLen - 1,
			endSlot:   epochLen + 2,
			expected:  []uint64{epochLen - 1, epochLen, epochLen + 1, epochLen + 2},
		},
		{
			name: "three epochs: end slot exists in third epoch and is inclusive",
			epochBlocks: map[uint64][]uint64{
				0: {epochLen - 1},
				1: {epochLen, epochLen + 100},
				2: {2 * epochLen, 2*epochLen + 1},
			},
			startSlot: epochLen - 1,
			endSlot:   2*epochLen + 1,
			expected:  []uint64{epochLen - 1, epochLen, epochLen + 100, 2 * epochLen, 2*epochLen + 1},
		},
		{
			name: "three epochs: end slot is not a confirmed block in third epoch",
			epochBlocks: map[uint64][]uint64{
				0: {epochLen - 1},
				1: {epochLen, epochLen + 100},
				2: {2 * epochLen, 2*epochLen + 5},
			},
			startSlot: epochLen - 1,
			endSlot:   2*epochLen + 3,
			expected:  []uint64{epochLen - 1, epochLen, epochLen + 100, 2 * epochLen},
		},
		// --- Error cases ---
		{
			name:        "missing epoch returns error",
			epochBlocks: map[uint64][]uint64{0: {100, 200}},
			startSlot:   100,
			endSlot:     epochLen + 1, // requires epoch 1, which is not loaded
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multi := newTestMultiEpoch(tt.epochBlocks)
			got, rpcErr, err := multi.getBlocksInRange(context.Background(), tt.startSlot, tt.endSlot)
			if tt.wantErr {
				assert.True(t, rpcErr != nil || err != nil, "expected an error but got none")
				return
			}
			require.NoError(t, err)
			assert.Nil(t, rpcErr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseGetBlocksRequest(t *testing.T) {
	tests := []struct {
		name       string
		params     string
		wantStart  uint64
		wantEnd    uint64
		wantCommit string
		wantErr    bool
	}{
		{
			name:      "valid startSlot and endSlot",
			params:    `[100, 200]`,
			wantStart: 100,
			wantEnd:   200,
		},
		{
			name:       "valid with commitment",
			params:     `[100, 200, {"commitment": "finalized"}]`,
			wantStart:  100,
			wantEnd:    200,
			wantCommit: "finalized",
		},
		{
			name:    "missing endSlot",
			params:  `[100]`,
			wantErr: true,
		},
		{
			name:    "wrong type for startSlot",
			params:  `["abc", 200]`,
			wantErr: true,
		},
		{
			name:    "wrong type for endSlot",
			params:  `[100, "end"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(tt.params)
			got, err := parseGetBlocksRequest(&raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, got.StartSlot)
			assert.Equal(t, tt.wantEnd, got.EndSlot)
			assert.Equal(t, tt.wantCommit, got.Commitment)
		})
	}
}
