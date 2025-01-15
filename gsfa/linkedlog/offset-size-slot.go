package linkedlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

func NewOffsetAndSizeAndSlot(offset uint64, size uint64, slot uint64) *OffsetAndSizeAndSlot {
	return &OffsetAndSizeAndSlot{
		Offset: offset,
		Size:   size,
		Slot:   slot,
	}
}

type OffsetAndSizeAndSlot struct {
	Offset uint64 // encoded as uvarint
	Size   uint64 // encoded as uvarint
	Slot   uint64 // encoded as uvarint
	Flags  Bitmap // encoded as byte
}

// HasMeta returns true if the transaction had metadata.
func (oas *OffsetAndSizeAndSlot) HasMeta() bool {
	return oas.Flags.Get(0)
}

// SetHasMeta sets the hasMeta flag.
func (oas *OffsetAndSizeAndSlot) SetHasMeta(hasMeta bool) {
	oas.Flags.Set(0, hasMeta)
}

// IsSuccess returns the isSuccess flag.
func (oas *OffsetAndSizeAndSlot) IsSuccess() bool {
	return oas.Flags.Get(1)
}

// SetIsSuccess sets the isSuccess flag.
func (oas *OffsetAndSizeAndSlot) SetIsSuccess(isSuccess bool) {
	oas.Flags.Set(1, isSuccess)
}

// IsVote returns the isVote flag.
func (oas *OffsetAndSizeAndSlot) IsVote() bool {
	return oas.Flags.Get(2)
}

// SetIsVote sets the isVote flag.
func (oas *OffsetAndSizeAndSlot) SetIsVote(isVote bool) {
	oas.Flags.Set(2, isVote)
}

// Bytes returns the offset and size as a byte slice.
func (oas OffsetAndSizeAndSlot) Bytes() []byte {
	buf := make([]byte, 0, binary.MaxVarintLen64*3)
	buf = binary.AppendUvarint(buf, oas.Offset)
	buf = binary.AppendUvarint(buf, oas.Size)
	buf = binary.AppendUvarint(buf, oas.Slot)
	buf = append(buf, byte(oas.Flags))
	buf = slices.Clip(buf)
	return buf
}

// FromBytes parses the offset and size from a byte slice.
func (oas *OffsetAndSizeAndSlot) FromBytes(buf []byte) error {
	if len(buf) > binary.MaxVarintLen64*3 {
		return errors.New("invalid byte slice length")
	}
	var n int
	oas.Offset, n = binary.Uvarint(buf)
	if n <= 0 {
		return errors.New("failed to parse offset")
	}
	buf = buf[n:]
	oas.Size, n = binary.Uvarint(buf)
	if n <= 0 {
		return errors.New("failed to parse size")
	}
	buf = buf[n:]
	oas.Slot, n = binary.Uvarint(buf)
	if n <= 0 {
		return errors.New("failed to parse slot")
	}
	buf = buf[n:]
	if len(buf) == 0 {
		return errors.New("missing flags")
	}
	oas.Flags = Bitmap(buf[0])
	return nil
}

func (oas *OffsetAndSizeAndSlot) FromReader(r UvarintReader) error {
	var err error
	oas.Offset, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read offset: %w", err)
	}
	oas.Size, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read size: %w", err)
	}
	oas.Slot, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read slot: %w", err)
	}
	b, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read flags: %w", err)
	}
	oas.Flags = Bitmap(b)
	return nil
}

type UvarintReader interface {
	ReadUvarint() (uint64, error)
	ReadByte() (byte, error)
}

type uvarintReader struct {
	pos int
	buf []byte
}

func (r *uvarintReader) ReadUvarint() (uint64, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	v, n := binary.Uvarint(r.buf[r.pos:])
	if n <= 0 {
		return 0, errors.New("failed to parse uvarint")
	}
	r.pos += n
	return v, nil
}

func (r *uvarintReader) ReadByte() (byte, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

func OffsetAndSizeAndSlotSliceFromBytes(buf []byte) ([]OffsetAndSizeAndSlot, error) {
	r := &uvarintReader{buf: buf}
	oass := make([]OffsetAndSizeAndSlot, 0)
	for {
		oas := OffsetAndSizeAndSlot{}
		err := oas.FromReader(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse offset and size: %w", err)
		}
		oass = append(oass, oas)
	}
	return oass, nil
}
