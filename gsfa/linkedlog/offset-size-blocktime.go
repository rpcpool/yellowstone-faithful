package linkedlog

import (
	"errors"
	"fmt"

	"github.com/rpcpool/yellowstone-faithful/indexes"
)

func NewOffsetAndSizeAndBlocktime(offset uint64, size uint64, blocktime uint64) *OffsetAndSizeAndBlocktime {
	return &OffsetAndSizeAndBlocktime{
		Offset:    offset,
		Size:      size,
		Blocktime: blocktime,
	}
}

// IsValid returns true if the offset and size are valid.
func (oas *OffsetAndSizeAndBlocktime) IsValid() bool {
	return oas.Offset <= indexes.MaxUint48 && oas.Size <= indexes.MaxUint24 && oas.Blocktime <= indexes.MaxUint40
}

type OffsetAndSizeAndBlocktime struct {
	Offset    uint64 // uint48, 6 bytes, max 281.5 TB (terabytes)
	Size      uint64 // uint24, 3 bytes, max 16.7 MB (megabytes)
	Blocktime uint64 // uint40, 5 bytes, max 1099511627775 (seconds since epoch)
}

const OffsetAndSizeAndBlocktimeSize = 6 + 3 + 5

// Bytes returns the offset and size as a byte slice.
func (oas OffsetAndSizeAndBlocktime) Bytes() []byte {
	return append(
		indexes.Uint48tob(oas.Offset),
		append(
			indexes.Uint24tob(uint32(oas.Size)),
			indexes.Uint40tob(uint64(oas.Blocktime))...,
		)...,
	)
}

// FromBytes parses the offset and size from a byte slice.
func (oas *OffsetAndSizeAndBlocktime) FromBytes(buf []byte) error {
	if len(buf) != OffsetAndSizeAndBlocktimeSize {
		return errors.New("invalid byte slice length")
	}
	_ = buf[OffsetAndSizeAndBlocktimeSize-1] // bounds check hint to compiler
	oas.Offset = indexes.BtoUint48(buf[:6])
	oas.Size = uint64(indexes.BtoUint24(buf[6:9]))
	oas.Blocktime = uint64(indexes.BtoUint40(buf[9:14]))
	return nil
}

func OffsetAndSizeAndBlocktimeSliceFromBytes(buf []byte) ([]OffsetAndSizeAndBlocktime, error) {
	if len(buf)%OffsetAndSizeAndBlocktimeSize != 0 {
		return nil, errors.New("invalid byte slice length")
	}
	oass := make([]OffsetAndSizeAndBlocktime, len(buf)/OffsetAndSizeAndBlocktimeSize)
	for i := 0; i < len(oass); i++ {
		if err := oass[i].FromBytes(buf[i*OffsetAndSizeAndBlocktimeSize : (i+1)*OffsetAndSizeAndBlocktimeSize]); err != nil {
			return nil, fmt.Errorf("failed to parse offset and size and blocktime at index %d: %w", i, err)
		}
	}
	return oass, nil
}
