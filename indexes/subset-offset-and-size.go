package indexes

import "errors"

type SubsetOffsetAndSize struct {
	Subset uint64 // uint24, 3 bytes, max 16.7 MB (megabytes)
	Offset uint64 // uint48, 6 bytes, max 281.5 TB (terabytes)
	Size   uint64 // uint24, 3 bytes, max 16.7 MB (megabytes)
}

// FromBytes parses the offset and size from a byte slice.
func (soas *SubsetOffsetAndSize) FromBytes(buf []byte) error {
	if len(buf) != IndexValueSize_CidToSubsetOffsetAndSize {
		return errors.New("invalid byte slice length")
	}
	_ = buf[IndexValueSize_CidToSubsetOffsetAndSize-1] // bounds check hint to compiler
	soas.Subset = uint64(BtoUint24(buf[:3]))
	soas.Offset = BtoUint48(buf[3:9])
	soas.Size = uint64(BtoUint24(buf[9:]))
	return nil
}
