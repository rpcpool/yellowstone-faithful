package iplddecoders

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/stretchr/testify/require"
)

// blockWithMeta re-encodes block_raw0 with the given raw SlotMeta tuple.
func blockWithMeta(t *testing.T, meta []any) []byte {
	t.Helper()
	var arr []any
	require.NoError(t, cbor.Unmarshal(block_raw0, &arr))
	require.Len(t, arr, 6)
	arr[4] = meta
	out, err := cbor.Marshal(arr)
	require.NoError(t, err)
	return out
}

func decodeBoth(t *testing.T, raw []byte) (*ipldbindcode.Block, *ipldbindcode.Block) {
	t.Helper()
	classic, err := _DecodeBlockClassic(raw)
	require.NoError(t, err)
	fast, err := _DecodeBlockFast(raw)
	require.NoError(t, err)
	require.True(t, classic.Meta.Equivalent(fast.Meta), "classic and fast SlotMeta differ")
	return classic, fast
}

func TestSlotMetaBlockMarkers(t *testing.T) {
	// u16 version | u8 variant | u16 len | payload
	header := []byte{0x01, 0x00, 0x01, 0x02, 0x00, 0x01, 0xee}
	footer := []byte{0x01, 0x00, 0x00, 0x03, 0x00, 0xaa, 0xbb, 0xcc}
	markers := [][]byte{header, footer}
	blockID := bytes.Repeat([]byte{0x42}, 32)

	metaLen := func(t *testing.T, raw []byte) int {
		var arr []any
		require.NoError(t, cbor.Unmarshal(raw, &arr))
		return len(arr[4].([]any))
	}
	// checkRoundTrips re-encodes via the fast encoder and bindnode and checks
	// both reproduce the input bytes.
	checkRoundTrips := func(t *testing.T, raw []byte, classic, fast *ipldbindcode.Block) {
		t.Helper()
		encoded, err := fast.MarshalCBOR()
		require.NoError(t, err)
		reClassic, reFast := decodeBoth(t, encoded)
		require.True(t, fast.Meta.Equivalent(reClassic.Meta))
		require.True(t, fast.Meta.Equivalent(reFast.Meta))
		require.Equal(t, metaLen(t, raw), metaLen(t, encoded))
		encoded, err = ipld.Marshal(dagcbor.Encode, classic, ipldbindcode.Prototypes.Block.Type())
		require.NoError(t, err)
		require.Equal(t, raw, encoded)
	}

	t.Run("2-element", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123)}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.Equal(t, 8, b.Meta.Parent_slot)
			require.Equal(t, 123, b.Meta.Blocktime)
			require.False(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("3-element", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7)})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			bh, ok := b.GetBlockHeight()
			require.True(t, ok)
			require.Equal(t, uint64(7), bh)
			require.False(t, b.Meta.HasBlockFooter())
			_, ok = b.GetBlockMarkers()
			require.False(t, ok)
		}
		checkRoundTrips(t, raw, classic, fast)
	})
	t.Run("3-element/null-height", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), nil}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.False(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("4-element/markers", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), markers})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			got, ok := b.GetBlockMarkers()
			require.True(t, ok)
			require.Equal(t, markers, got)
			gotFooter, ok := b.GetBlockFooter()
			require.True(t, ok)
			require.Equal(t, footer, gotFooter)
			require.True(t, b.Meta.HasBlockFooter())
			_, ok = b.GetBlockID()
			require.False(t, ok)
		}
		checkRoundTrips(t, raw, classic, fast)
	})
	t.Run("4-element/null-height", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), nil, markers})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.False(t, b.Meta.HasBlockHeight())
			got, ok := b.GetBlockFooter()
			require.True(t, ok)
			require.Equal(t, footer, got)
		}
		checkRoundTrips(t, raw, classic, fast)
	})
	t.Run("4-element/null-markers", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), nil}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.True(t, b.Meta.HasBlockHeight())
			_, ok := b.GetBlockMarkers()
			require.False(t, ok)
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("4-element/empty-markers", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), [][]byte{}}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			got, ok := b.GetBlockMarkers()
			require.True(t, ok)
			require.Empty(t, got)
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("4-element/no-footer-marker", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), [][]byte{header}}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			_, ok := b.GetBlockFooter()
			require.False(t, ok)
		}
	})
	t.Run("5-element/markers-and-id", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), markers, blockID})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			got, ok := b.GetBlockMarkers()
			require.True(t, ok)
			require.Equal(t, markers, got)
			id, ok := b.GetBlockID()
			require.True(t, ok)
			require.Equal(t, blockID, id)
		}
		checkRoundTrips(t, raw, classic, fast)
	})
	t.Run("5-element/all-null", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), nil, nil, nil}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.False(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
			_, ok := b.GetBlockMarkers()
			require.False(t, ok)
			_, ok = b.GetBlockID()
			require.False(t, ok)
		}
	})
	t.Run("5-element/id-only", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), nil, nil, blockID})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			_, ok := b.GetBlockMarkers()
			require.False(t, ok)
			id, ok := b.GetBlockID()
			require.True(t, ok)
			require.Equal(t, blockID, id)
		}
		checkRoundTrips(t, raw, classic, fast)
	})
	t.Run("fast-decoder/rejects-bad-block-id", func(t *testing.T) {
		_, err := _DecodeBlockFast(blockWithMeta(t, []any{uint64(8), uint64(123), nil, markers, []byte{1, 2, 3}}))
		require.ErrorContains(t, err, "block_id")
	})
	t.Run("fast-decoder/rejects-non-bytes-marker", func(t *testing.T) {
		_, err := _DecodeBlockFast(blockWithMeta(t, []any{uint64(8), uint64(123), nil, []any{uint64(1)}}))
		require.ErrorContains(t, err, "block_markers[0]")
	})
	t.Run("fast-decoder/rejects-bytes-as-markers", func(t *testing.T) {
		_, err := _DecodeBlockFast(blockWithMeta(t, []any{uint64(8), uint64(123), nil, footer}))
		require.ErrorContains(t, err, "block_markers")
	})
	t.Run("fast-encoder/shortest-tuple", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			meta []any
			want int
		}{
			{"pre-alpenglow", []any{uint64(8), uint64(123), uint64(7)}, 3},
			{"markers", []any{uint64(8), uint64(123), uint64(7), markers}, 4},
			{"markers-and-id", []any{uint64(8), uint64(123), uint64(7), markers, blockID}, 5},
		} {
			raw := blockWithMeta(t, tc.meta)
			_, fast := decodeBoth(t, raw)
			encoded, err := fast.MarshalCBOR()
			require.NoError(t, err, tc.name)
			require.Equal(t, tc.want, metaLen(t, encoded), tc.name)
		}
		// pre-Alpenglow blocks must encode byte-identically to before.
		_, fast := decodeBoth(t, block_raw0)
		encoded, err := fast.MarshalCBOR()
		require.NoError(t, err)
		require.Equal(t, block_raw0, encoded)
		require.Equal(t, 3, metaLen(t, encoded))
	})
	t.Run("reset-clears-markers", func(t *testing.T) {
		_, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), markers, blockID}))
		fast.Reset()
		_, ok := fast.GetBlockMarkers()
		require.False(t, ok)
		_, ok = fast.GetBlockID()
		require.False(t, ok)
		require.False(t, fast.Meta.HasBlockFooter())
		// the marker bytes must not have been clobbered by Reset.
		require.Equal(t, []byte{0x01, 0x00, 0x00, 0x03, 0x00, 0xaa, 0xbb, 0xcc}, footer)
	})
	t.Run("equivalent", func(t *testing.T) {
		_, a := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), markers, blockID}))
		_, b := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), markers}))
		_, c := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), [][]byte{header}, blockID}))
		require.False(t, a.Meta.Equivalent(b.Meta))
		require.False(t, a.Meta.Equivalent(c.Meta))
	})
	t.Run("json", func(t *testing.T) {
		_, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), [][]byte{footer}, blockID}))
		got, err := json.Marshal(fast.Meta)
		require.NoError(t, err)
		require.JSONEq(t, `{"parent_slot":8,"blocktime":123,"block_height":7,"block_markers":["AQAAAwCqu8w="],"block_id":"QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI="}`, string(got))
	})
	t.Run("qp-build", func(t *testing.T) {
		// This is how the CAR builder (radiance) constructs Block nodes.
		classic, _ := decodeBoth(t, block_raw0)
		build := func(heightFn qp.Assemble, markersFn, idFn qp.Assemble) []byte {
			node, err := qp.BuildMap(ipldbindcode.Prototypes.Block, -1, func(ma datamodel.MapAssembler) {
				qp.MapEntry(ma, "kind", qp.Int(2))
				qp.MapEntry(ma, "slot", qp.Int(int64(classic.Slot)))
				qp.MapEntry(ma, "shredding", qp.List(-1, func(la datamodel.ListAssembler) {}))
				qp.MapEntry(ma, "entries", qp.List(-1, func(la datamodel.ListAssembler) {}))
				qp.MapEntry(ma, "meta", qp.Map(-1, func(ma datamodel.MapAssembler) {
					qp.MapEntry(ma, "parent_slot", qp.Int(8))
					qp.MapEntry(ma, "blocktime", qp.Int(123))
					qp.MapEntry(ma, "block_height", heightFn)
					if markersFn != nil {
						qp.MapEntry(ma, "block_markers", markersFn)
					}
					if idFn != nil {
						qp.MapEntry(ma, "block_id", idFn)
					}
				}))
				qp.MapEntry(ma, "rewards", qp.Link(classic.Rewards.(cidlink.Link)))
			})
			require.NoError(t, err)
			var buf bytes.Buffer
			require.NoError(t, dagcbor.Encode(node.(interface{ Representation() datamodel.Node }).Representation(), &buf))
			return buf.Bytes()
		}
		markersList := qp.List(-1, func(la datamodel.ListAssembler) {
			for _, m := range markers {
				qp.ListEntry(la, qp.Bytes(m))
			}
		})
		{
			raw := build(qp.Int(7), markersList, qp.Bytes(blockID))
			require.Equal(t, 5, metaLen(t, raw))
			classic, fast := decodeBoth(t, raw)
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				got, ok := b.GetBlockMarkers()
				require.True(t, ok)
				require.Equal(t, markers, got)
				id, ok := b.GetBlockID()
				require.True(t, ok)
				require.Equal(t, blockID, id)
				gotFooter, ok := b.GetBlockFooter()
				require.True(t, ok)
				require.Equal(t, footer, gotFooter)
			}
			encoded, err := fast.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, raw, encoded)
		}
		{
			raw := build(qp.Null(), markersList, nil)
			require.Equal(t, 4, metaLen(t, raw))
			classic, fast := decodeBoth(t, raw)
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				require.False(t, b.Meta.HasBlockHeight())
				require.True(t, b.Meta.HasBlockFooter())
				_, ok := b.GetBlockID()
				require.False(t, ok)
			}
			encoded, err := fast.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, raw, encoded)
		}
		{
			raw := build(qp.Int(7), nil, nil)
			require.Equal(t, 3, metaLen(t, raw))
			classic, fast := decodeBoth(t, raw)
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				_, ok := b.GetBlockMarkers()
				require.False(t, ok)
				require.False(t, b.Meta.HasBlockFooter())
			}
		}
		{
			classic, fast := decodeBoth(t, build(qp.Int(7), qp.Null(), qp.Null()))
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				_, ok := b.GetBlockMarkers()
				require.False(t, ok)
				_, ok = b.GetBlockID()
				require.False(t, ok)
			}
		}
	})
}
