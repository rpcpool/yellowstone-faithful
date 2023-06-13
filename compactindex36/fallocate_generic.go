//go:build !linux

package compactindex36

import (
	"os"
)

func fallocate(f *os.File, offset int64, size int64) error {
	return fake_fallocate(f, offset, size)
}
