//go:build linux

package compactindexsized

import (
	"fmt"
	"os"
	"syscall"
)

func fallocate(f *os.File, offset int64, size int64) error {
	err := syscall.Fallocate(int(f.Fd()), 0, offset, size)
	if err != nil {
		return fmt.Errorf("failure while linux fallocate: %w", err)
	}
	return nil
}
