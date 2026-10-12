package main

import (
	"encoding/json"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_newBlockMarkersLine(t *testing.T) {
	t.Run("alpenglow block", func(t *testing.T) {
		markers := ipldbindcode.List__Bytes{{1, 0, 1}, {1, 0, 0, 0xff}}
		markersPtr := &markers
		id := make([]byte, 32)
		id[31] = 1
		idPtr := &id
		block := &ipldbindcode.Block{Slot: 448940256}
		block.Meta.Block_markers = &markersPtr
		block.Meta.Block_id = &idPtr

		raw, err := json.Marshal(newBlockMarkersLine(block))
		require.NoError(t, err)
		assert.JSONEq(t, `{"slot":448940256,"block_id":"11111111111111111111111111111112","markers":["AQAB","AQAA/w=="]}`, string(raw))
	})
	t.Run("block without markers", func(t *testing.T) {
		raw, err := json.Marshal(newBlockMarkersLine(&ipldbindcode.Block{Slot: 7}))
		require.NoError(t, err)
		assert.JSONEq(t, `{"slot":7,"block_id":null,"markers":[]}`, string(raw))
	})
}
