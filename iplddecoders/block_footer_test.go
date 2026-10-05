package iplddecoders

import (
	"bytes"
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

func TestSlotMetaBlockFooter(t *testing.T) {
	footer := []byte{0x01, 0x00, 0x00, 0x03, 0x00, 0xaa, 0xbb, 0xcc}

	t.Run("2-element", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123)}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.Equal(t, 8, b.Meta.Parent_slot)
			require.Equal(t, 123, b.Meta.Blocktime)
			require.False(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
			_, ok := b.GetBlockFooter()
			require.False(t, ok)
		}
	})
	t.Run("3-element", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7)}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			bh, ok := b.GetBlockHeight()
			require.True(t, ok)
			require.Equal(t, uint64(7), bh)
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("3-element/null-height", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), nil}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.False(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("4-element/bytes", func(t *testing.T) {
		raw := blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), footer})
		classic, fast := decodeBoth(t, raw)
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			bh, ok := b.GetBlockHeight()
			require.True(t, ok)
			require.Equal(t, uint64(7), bh)
			got, ok := b.GetBlockFooter()
			require.True(t, ok)
			require.Equal(t, footer, got)
		}
		// round-trip through the fast encoder.
		encoded, err := fast.MarshalCBOR()
		require.NoError(t, err)
		reClassic, reFast := decodeBoth(t, encoded)
		require.True(t, fast.Meta.Equivalent(reClassic.Meta))
		require.True(t, fast.Meta.Equivalent(reFast.Meta))
		// round-trip through bindnode.
		encoded, err = ipld.Marshal(dagcbor.Encode, classic, ipldbindcode.Prototypes.Block.Type())
		require.NoError(t, err)
		require.Equal(t, raw, encoded)
	})
	t.Run("4-element/null-height", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), nil, footer}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.False(t, b.Meta.HasBlockHeight())
			got, ok := b.GetBlockFooter()
			require.True(t, ok)
			require.Equal(t, footer, got)
		}
		encoded, err := fast.MarshalCBOR()
		require.NoError(t, err)
		reClassic, _ := decodeBoth(t, encoded)
		require.True(t, fast.Meta.Equivalent(reClassic.Meta))
	})
	t.Run("4-element/null-footer", func(t *testing.T) {
		classic, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), nil}))
		for _, b := range []*ipldbindcode.Block{classic, fast} {
			require.True(t, b.Meta.HasBlockHeight())
			require.False(t, b.Meta.HasBlockFooter())
		}
	})
	t.Run("fast-encoder/no-footer-keeps-3-elements", func(t *testing.T) {
		_, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7)}))
		encoded, err := fast.MarshalCBOR()
		require.NoError(t, err)
		var arr []any
		require.NoError(t, cbor.Unmarshal(encoded, &arr))
		require.Len(t, arr[4], 3)
	})
	t.Run("reset-clears-footer", func(t *testing.T) {
		_, fast := decodeBoth(t, blockWithMeta(t, []any{uint64(8), uint64(123), uint64(7), footer}))
		fast.Reset()
		require.False(t, fast.Meta.HasBlockFooter())
		// the footer bytes must not have been clobbered by Reset.
		require.Equal(t, []byte{0x01, 0x00, 0x00, 0x03, 0x00, 0xaa, 0xbb, 0xcc}, footer)
	})
	t.Run("qp-build", func(t *testing.T) {
		// This is how the CAR builder (radiance) constructs Block nodes.
		classic, _ := decodeBoth(t, block_raw0)
		build := func(footerFn qp.Assemble) []byte {
			node, err := qp.BuildMap(ipldbindcode.Prototypes.Block, -1, func(ma datamodel.MapAssembler) {
				qp.MapEntry(ma, "kind", qp.Int(2))
				qp.MapEntry(ma, "slot", qp.Int(int64(classic.Slot)))
				qp.MapEntry(ma, "shredding", qp.List(-1, func(la datamodel.ListAssembler) {}))
				qp.MapEntry(ma, "entries", qp.List(-1, func(la datamodel.ListAssembler) {}))
				qp.MapEntry(ma, "meta", qp.Map(-1, func(ma datamodel.MapAssembler) {
					qp.MapEntry(ma, "parent_slot", qp.Int(8))
					qp.MapEntry(ma, "blocktime", qp.Int(123))
					qp.MapEntry(ma, "block_height", qp.Null())
					qp.MapEntry(ma, "block_footer", footerFn)
				}))
				qp.MapEntry(ma, "rewards", qp.Link(classic.Rewards.(cidlink.Link)))
			})
			require.NoError(t, err)
			var buf bytes.Buffer
			require.NoError(t, dagcbor.Encode(node.(interface{ Representation() datamodel.Node }).Representation(), &buf))
			return buf.Bytes()
		}
		{
			classic, fast := decodeBoth(t, build(qp.Bytes(footer)))
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				require.False(t, b.Meta.HasBlockHeight())
				got, ok := b.GetBlockFooter()
				require.True(t, ok)
				require.Equal(t, footer, got)
			}
		}
		{
			classic, fast := decodeBoth(t, build(qp.Null()))
			for _, b := range []*ipldbindcode.Block{classic, fast} {
				require.False(t, b.Meta.HasBlockFooter())
			}
		}
	})
}
