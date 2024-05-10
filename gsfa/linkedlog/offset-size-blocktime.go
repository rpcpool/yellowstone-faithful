package linkedlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

func NewOffsetAndSizeAndBlocktime(offset uint64, size uint64, blocktime uint64) *OffsetAndSizeAndBlocktime {
	return &OffsetAndSizeAndBlocktime{
		Offset:    offset,
		Size:      size,
		Blocktime: blocktime,
	}
}

type OffsetAndSizeAndBlocktime struct {
	Offset    uint64 // uint48, 6 bytes, max 281.5 TB (terabytes)
	Size      uint64 // uint24, 3 bytes, max 16.7 MB (megabytes)
	Blocktime uint64 // uint40, 5 bytes, max 1099511627775 (seconds since epoch)
}

// Bytes returns the offset and size as a byte slice.
func (oas OffsetAndSizeAndBlocktime) Bytes() []byte {
	buf := make([]byte, 0, binary.MaxVarintLen64*3)
	buf = binary.AppendUvarint(buf, oas.Offset)
	buf = binary.AppendUvarint(buf, oas.Size)
	buf = binary.AppendUvarint(buf, oas.Blocktime)
	buf = slices.Clip(buf)
	return buf
}

// FromBytes parses the offset and size from a byte slice.
func (oas *OffsetAndSizeAndBlocktime) FromBytes(buf []byte) error {
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
	oas.Blocktime, n = binary.Uvarint(buf)
	if n <= 0 {
		return errors.New("failed to parse blocktime")
	}
	return nil
}

func (oas *OffsetAndSizeAndBlocktime) FromReader(r UvarintReader) error {
	var err error
	oas.Offset, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read offset: %w", err)
	}
	oas.Size, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read size: %w", err)
	}
	oas.Blocktime, err = r.ReadUvarint()
	if err != nil {
		return fmt.Errorf("failed to read blocktime: %w", err)
	}
	return nil
}

type UvarintReader interface {
	ReadUvarint() (uint64, error)
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

func OffsetAndSizeAndBlocktimeSliceFromBytes(buf []byte) ([]OffsetAndSizeAndBlocktime, error) {
	r := &uvarintReader{buf: buf}
	oass := make([]OffsetAndSizeAndBlocktime, 0)
	for {
		oas := OffsetAndSizeAndBlocktime{}
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
