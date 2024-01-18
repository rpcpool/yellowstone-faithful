package indexes

import "encoding/binary"

const (
	maxUint24 = 1<<24 - 1
	maxUint40 = 1<<40 - 1
	maxUint48 = 1<<48 - 1
	maxUint64 = 1<<64 - 1
)

// uint24tob converts a uint32 to a 3-byte slice; panics if v > maxUint24.
func uint24tob(v uint32) []byte {
	if v > maxUint24 {
		panic("uint24tob: value out of range")
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf[:3]
}

// btoUint24 converts a 3-byte slice to a uint32.
func btoUint24(buf []byte) uint32 {
	_ = buf[2] // bounds check hint to compiler
	return binary.LittleEndian.Uint32(cloneAndPad(buf, 1))
}

// uint40tob converts a uint64 to a 5-byte slice; panics if v > maxUint40.
func uint40tob(v uint64) []byte {
	if v > maxUint40 {
		panic("uint40tob: value out of range")
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf[:5]
}

// btoUint40 converts a 5-byte slice to a uint64.
func btoUint40(buf []byte) uint64 {
	_ = buf[4] // bounds check hint to compiler
	return binary.LittleEndian.Uint64(cloneAndPad(buf, 3))
}

// uint48tob converts a uint64 to a 6-byte slice; panics if v > maxUint48.
func uint48tob(v uint64) []byte {
	if v > maxUint48 {
		panic("uint48tob: value out of range")
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf[:6]
}

// btoUint48 converts a 6-byte slice to a uint64.
func btoUint48(buf []byte) uint64 {
	_ = buf[5] // bounds check hint to compiler
	return binary.LittleEndian.Uint64(cloneAndPad(buf, 2))
}

// uint64tob converts a uint64 to an 8-byte little-endian slice.
func uint64tob(v uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return buf
}

// btoUint64 converts an 8-byte little-endian slice to a uint64.
func btoUint64(buf []byte) uint64 {
	_ = buf[7] // bounds check hint to compiler
	return binary.LittleEndian.Uint64(buf)
}

// cloneAndPad clones a byte slice and pads it with zeros.
func cloneAndPad(buf []byte, pad int) []byte {
	out := make([]byte, len(buf)+pad)
	copy(out, buf)
	return out
}
