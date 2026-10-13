package blockmarker

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

// marker builds u16 version | u8 variant | u16 len | payload.
func marker(version uint16, variant uint8, payload []byte) []byte {
	b := binary.LittleEndian.AppendUint16(nil, version)
	b = append(b, variant)
	b = binary.LittleEndian.AppendUint16(b, uint16(len(payload)))
	return append(b, payload...)
}

func cat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func u64(v uint64) []byte {
	return binary.LittleEndian.AppendUint64(nil, v)
}

var (
	bankHash = bytes.Repeat([]byte{0xab}, 32)
	blockID  = bytes.Repeat([]byte{0xcd}, 32)
	blsSig   = bytes.Repeat([]byte{0xef}, 192)
	certs    = []byte{0x01, 0x02, 0x03}
)

func footerPayload(userAgent string, certs []byte) []byte {
	return cat([]byte{1}, bankHash, u64(1_700_000_000_123), []byte{byte(len(userAgent))}, []byte(userAgent), certs)
}

func TestParseFooter(t *testing.T) {
	raw := marker(1, 0, footerPayload("agave/4.3.0", certs))
	m, err := Parse(raw)
	require.NoError(t, err)
	require.Equal(t, VariantFooter, m.Variant)
	require.Equal(t, raw, m.Raw)
	require.Equal(t, raw[5:], m.Payload)

	f, err := m.Footer()
	require.NoError(t, err)
	require.Equal(t, bankHash, f.BankHash[:])
	require.Equal(t, uint64(1_700_000_000_123), f.BlockProducerTimeNanos)
	require.Equal(t, []byte("agave/4.3.0"), f.BlockUserAgent)
	require.Equal(t, certs, f.Certificates)

	_, err = m.Header()
	require.ErrorIs(t, err, ErrInvalid)
}

func TestParseFooterEmptyUserAgentNoCerts(t *testing.T) {
	m, err := Parse(marker(1, 0, footerPayload("", nil)))
	require.NoError(t, err)
	f, err := m.Footer()
	require.NoError(t, err)
	require.Empty(t, f.BlockUserAgent)
	require.Empty(t, f.Certificates)
}

func TestParseHeaderAndUpdateParent(t *testing.T) {
	payload := cat([]byte{1}, u64(12345), blockID)

	m, err := Parse(marker(1, 1, payload))
	require.NoError(t, err)
	require.Equal(t, VariantHeader, m.Variant)
	h, err := m.Header()
	require.NoError(t, err)
	require.Equal(t, uint64(12345), h.ParentSlot)
	require.Equal(t, blockID, h.ParentBlockID[:])

	m, err = Parse(marker(1, 2, payload))
	require.NoError(t, err)
	require.Equal(t, VariantUpdateParent, m.Variant)
	u, err := m.UpdateParent()
	require.NoError(t, err)
	require.Equal(t, uint64(12345), u.NewParentSlot)
	require.Equal(t, blockID, u.NewParentBlockID[:])
}

func TestParseGenesisCert(t *testing.T) {
	bitmap := []byte{0xff, 0x0f}
	m, err := Parse(marker(1, 3, cat(u64(777), blockID, blsSig, u64(uint64(len(bitmap))), bitmap)))
	require.NoError(t, err)
	require.Equal(t, VariantGenesisCertificate, m.Variant)
	g, err := m.GenesisCert()
	require.NoError(t, err)
	require.Equal(t, uint64(777), g.Slot)
	require.Equal(t, blockID, g.BlockID[:])
	require.Equal(t, blsSig, g.BLSSignature[:])
	require.Equal(t, bitmap, g.Bitmap)
}

func TestParseErrors(t *testing.T) {
	for name, raw := range map[string][]byte{
		"empty":           nil,
		"short header":    {0x01, 0x00, 0x00, 0x00},
		"bad version":     marker(2, 0, footerPayload("", nil)),
		"unknown variant": marker(1, 4, nil),
		"len too long":    marker(1, 0, footerPayload("", nil))[:20],
		"len too short":   append(marker(1, 1, cat([]byte{1}, u64(1), blockID)), 0x00),
	} {
		_, err := Parse(raw)
		require.ErrorIs(t, err, ErrInvalid, name)
	}
}

func TestParsePrefix(t *testing.T) {
	first := marker(1, 1, cat([]byte{1}, u64(1), blockID))
	m, n, err := ParsePrefix(cat(first, []byte{0xde, 0xad}))
	require.NoError(t, err)
	require.Equal(t, len(first), n)
	require.Equal(t, first, m.Raw)
}

func TestDecodeErrors(t *testing.T) {
	decode := func(variant uint8, payload []byte) error {
		m, err := Parse(marker(1, variant, payload))
		require.NoError(t, err)
		switch Variant(variant) {
		case VariantFooter:
			_, err = m.Footer()
		case VariantHeader:
			_, err = m.Header()
		case VariantUpdateParent:
			_, err = m.UpdateParent()
		case VariantGenesisCertificate:
			_, err = m.GenesisCert()
		}
		return err
	}
	for name, tc := range map[string]struct {
		variant uint8
		payload []byte
	}{
		"footer empty":               {0, nil},
		"footer bad inner version":   {0, append([]byte{2}, footerPayload("", nil)[1:]...)},
		"footer short":               {0, footerPayload("", nil)[:20]},
		"footer short user agent":    {0, footerPayload("agave", nil)[:44]},
		"header short":               {1, cat([]byte{1}, u64(1))},
		"header trailing":            {1, cat([]byte{1}, u64(1), blockID, []byte{0})},
		"header bad inner version":   {1, cat([]byte{0}, u64(1), blockID)},
		"update parent short":        {2, cat([]byte{1}, u64(1), blockID[:31])},
		"genesis short":              {3, cat(u64(1), blockID)},
		"genesis bitmap too long":    {3, cat(u64(1), blockID, blsSig, u64(513), make([]byte, 513))},
		"genesis bitmap len differs": {3, cat(u64(1), blockID, blsSig, u64(4), []byte{1, 2})},
	} {
		require.ErrorIs(t, decode(tc.variant, tc.payload), ErrInvalid, name)
	}
}

func TestFindFooter(t *testing.T) {
	header := marker(1, 1, cat([]byte{1}, u64(1), blockID))
	footer := marker(1, 0, footerPayload("x", nil))

	m, err := FindFooter([][]byte{header, footer})
	require.NoError(t, err)
	require.Equal(t, footer, m.Raw)

	m, err = FindFooter([][]byte{header})
	require.NoError(t, err)
	require.Nil(t, m)

	_, err = FindFooter([][]byte{{0x01}})
	require.ErrorIs(t, err, ErrInvalid)
}
