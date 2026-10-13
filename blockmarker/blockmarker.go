// Package blockmarker parses Alpenglow block markers (Agave's
// VersionedBlockMarker, entry/src/block_component.rs), as archived in
// SlotMeta.block_markers.
//
// Wire layout (little-endian): u16 version (1) | u8 variant | u16 len | payload (len bytes).
package blockmarker

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Variant uint8

const (
	VariantFooter             Variant = 0
	VariantHeader             Variant = 1
	VariantUpdateParent       Variant = 2
	VariantGenesisCertificate Variant = 3
)

func (v Variant) String() string {
	switch v {
	case VariantFooter:
		return "BlockFooter"
	case VariantHeader:
		return "BlockHeader"
	case VariantUpdateParent:
		return "UpdateParent"
	case VariantGenesisCertificate:
		return "GenesisCertificate"
	default:
		return fmt.Sprintf("Variant(%d)", uint8(v))
	}
}

const (
	markerVersionV1 = 1
	headerSize      = 2 + 1 + 2 // version + variant + len
	// Inner version byte of the footer, header and update-parent payloads.
	payloadVersionV1 = 1
)

var ErrInvalid = errors.New("invalid block marker")

// Marker is a parsed VersionedBlockMarker. Payload and Raw alias the input.
type Marker struct {
	Variant Variant
	// Payload is the len-prefixed inner value.
	Payload []byte
	// Raw is the whole VersionedBlockMarker, as archived in SlotMeta.block_markers.
	Raw []byte
}

// ParsePrefix parses a marker from the front of b and returns the number of
// bytes it occupies; trailing bytes are left for the caller.
func ParsePrefix(b []byte) (*Marker, int, error) {
	if len(b) < headerSize {
		return nil, 0, fmt.Errorf("%w: short header (%d bytes)", ErrInvalid, len(b))
	}
	if version := binary.LittleEndian.Uint16(b[0:2]); version != markerVersionV1 {
		return nil, 0, fmt.Errorf("%w: unsupported version %d", ErrInvalid, version)
	}
	variant := Variant(b[2])
	if variant > VariantGenesisCertificate {
		return nil, 0, fmt.Errorf("%w: unknown variant %d", ErrInvalid, variant)
	}
	size := headerSize + int(binary.LittleEndian.Uint16(b[3:5]))
	if len(b) < size {
		return nil, 0, fmt.Errorf("%w: %s payload needs %d bytes, have %d",
			ErrInvalid, variant, size-headerSize, len(b)-headerSize)
	}
	return &Marker{
		Variant: variant,
		Payload: b[headerSize:size],
		Raw:     b[:size],
	}, size, nil
}

// Parse parses b as exactly one marker.
func Parse(b []byte) (*Marker, error) {
	m, n, err := ParsePrefix(b)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, fmt.Errorf("%w: %s len says %d payload bytes, have %d",
			ErrInvalid, m.Variant, n-headerSize, len(b)-headerSize)
	}
	return m, nil
}

// FindFooter returns the first footer in markers, or nil if there is none.
func FindFooter(markers [][]byte) (*Marker, error) {
	for i, raw := range markers {
		m, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("marker %d: %w", i, err)
		}
		if m.Variant == VariantFooter {
			return m, nil
		}
	}
	return nil, nil
}

// FooterV1 is an Alpenglow block footer.
type FooterV1 struct {
	BankHash               [32]byte
	BlockProducerTimeNanos uint64
	BlockUserAgent         []byte
	// Certificates holds the undecoded trailing Option<BlockFinalizationCert>,
	// Option<SkipRewardCertificate> and Option<NotarRewardCertificate>.
	Certificates []byte
}

// HeaderV1 is an Alpenglow block header.
type HeaderV1 struct {
	ParentSlot    uint64
	ParentBlockID [32]byte
}

// UpdateParentV1 switches a block to a new parent (fast leader handover).
type UpdateParentV1 struct {
	NewParentSlot    uint64
	NewParentBlockID [32]byte
}

// GenesisCertificate is the certificate for the Alpenglow genesis block.
type GenesisCertificate struct {
	Slot         uint64
	BlockID      [32]byte
	BLSSignature [192]byte
	Bitmap       []byte
}

const maxGenesisBitmapLen = 512

// Footer decodes a BlockFooter marker. Byte slices alias the marker.
func (m *Marker) Footer() (*FooterV1, error) {
	p, err := m.versionedPayload(VariantFooter)
	if err != nil {
		return nil, err
	}
	// bank_hash | u64 time | u8 len + user agent
	if len(p) < 32+8+1 {
		return nil, fmt.Errorf("%w: short footer (%d bytes)", ErrInvalid, len(p))
	}
	var f FooterV1
	copy(f.BankHash[:], p[0:32])
	f.BlockProducerTimeNanos = binary.LittleEndian.Uint64(p[32:40])
	uaEnd := 41 + int(p[40])
	if len(p) < uaEnd {
		return nil, fmt.Errorf("%w: short footer user agent", ErrInvalid)
	}
	f.BlockUserAgent = p[41:uaEnd]
	f.Certificates = p[uaEnd:]
	return &f, nil
}

// Header decodes a BlockHeader marker.
func (m *Marker) Header() (*HeaderV1, error) {
	p, err := m.versionedPayload(VariantHeader)
	if err != nil {
		return nil, err
	}
	slot, id, err := slotAndBlockID(p)
	if err != nil {
		return nil, err
	}
	return &HeaderV1{ParentSlot: slot, ParentBlockID: id}, nil
}

// UpdateParent decodes an UpdateParent marker.
func (m *Marker) UpdateParent() (*UpdateParentV1, error) {
	p, err := m.versionedPayload(VariantUpdateParent)
	if err != nil {
		return nil, err
	}
	slot, id, err := slotAndBlockID(p)
	if err != nil {
		return nil, err
	}
	return &UpdateParentV1{NewParentSlot: slot, NewParentBlockID: id}, nil
}

// GenesisCert decodes a GenesisCertificate marker. Its payload has no inner
// version byte. Bitmap aliases the marker.
func (m *Marker) GenesisCert() (*GenesisCertificate, error) {
	if m.Variant != VariantGenesisCertificate {
		return nil, fmt.Errorf("%w: %s is not a %s", ErrInvalid, m.Variant, VariantGenesisCertificate)
	}
	p := m.Payload
	const fixed = 8 + 32 + 192 + 8
	if len(p) < fixed {
		return nil, fmt.Errorf("%w: short genesis certificate (%d bytes)", ErrInvalid, len(p))
	}
	var g GenesisCertificate
	g.Slot = binary.LittleEndian.Uint64(p[0:8])
	copy(g.BlockID[:], p[8:40])
	copy(g.BLSSignature[:], p[40:232])
	bitmapLen := binary.LittleEndian.Uint64(p[232:fixed])
	if bitmapLen > maxGenesisBitmapLen {
		return nil, fmt.Errorf("%w: genesis certificate bitmap too long (%d bytes)", ErrInvalid, bitmapLen)
	}
	if uint64(len(p)-fixed) != bitmapLen {
		return nil, fmt.Errorf("%w: genesis certificate bitmap is %d bytes, have %d", ErrInvalid, bitmapLen, len(p)-fixed)
	}
	g.Bitmap = p[fixed:]
	return &g, nil
}

// versionedPayload checks the variant and strips the inner version byte.
func (m *Marker) versionedPayload(want Variant) ([]byte, error) {
	if m.Variant != want {
		return nil, fmt.Errorf("%w: %s is not a %s", ErrInvalid, m.Variant, want)
	}
	if len(m.Payload) == 0 {
		return nil, fmt.Errorf("%w: empty %s payload", ErrInvalid, want)
	}
	if v := m.Payload[0]; v != payloadVersionV1 {
		return nil, fmt.Errorf("%w: unsupported %s version %d", ErrInvalid, want, v)
	}
	return m.Payload[1:], nil
}

func slotAndBlockID(p []byte) (uint64, [32]byte, error) {
	var id [32]byte
	if len(p) != 8+32 {
		return 0, id, fmt.Errorf("%w: expected 40-byte slot and block id, have %d bytes", ErrInvalid, len(p))
	}
	copy(id[:], p[8:40])
	return binary.LittleEndian.Uint64(p[0:8]), id, nil
}
