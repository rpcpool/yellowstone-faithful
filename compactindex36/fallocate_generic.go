//go:build !linux

package compactindex

import (
	"os"
)

func fallocate(f *os.File, offset int64, size int64) error {
	const blockSize = 4096
	var zero [blockSize]byte

	for size > 0 {
		step := size
		if step > blockSize {
			step = blockSize
		}

		if _, err := f.Write(zero[:step]); err != nil {
			return err
		}

		offset += step
		size -= step
	}

	return nil
}
