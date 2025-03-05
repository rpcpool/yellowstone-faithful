package linkedlog

import "fmt"

type Bitmap byte

func NewBitmap() Bitmap {
	return Bitmap(0)
}

func NewBitmapFromValues(values ...bool) Bitmap {
	if len(values) > 8 {
		panic(fmt.Errorf("too many values: %d", len(values)))
	}
	var bm Bitmap
	for i, v := range values {
		bm.Set(i, v)
	}
	return bm
}

// Get returns the value of the bit at the given index.
func (b Bitmap) Get(index int) bool {
	if index < 0 || index >= 8 {
		panic(fmt.Errorf("index out of bounds: %d", index))
	}
	return b&(1<<uint(index)) != 0
}

// Set sets the value of the bit at the given index.
func (b *Bitmap) Set(index int, value bool) {
	if index < 0 || index >= 8 {
		panic(fmt.Errorf("index out of bounds: %d", index))
	}
	if value {
		*b |= 1 << uint(index)
	} else {
		*b &= ^(1 << uint(index))
	}
}

// IsEmpty returns true if all the bits are unset.
func (b Bitmap) IsEmpty() bool {
	return b == 0
}
