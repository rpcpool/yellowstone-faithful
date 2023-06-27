package types

// Position indicates a position in a file
type Position uint64

const OffBytesLen = 8

type Block struct {
	Offset Position
	Size   Size
}

type Size uint32

const SizeBytesLen = 4

type Work uint64
