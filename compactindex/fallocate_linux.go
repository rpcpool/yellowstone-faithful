package compactindex

import (
	"os"
	"syscall"
)

func fallocate(f *os.File, offset int64, size int64) error {
	return syscall.Fallocate(int(f.Fd()), 0, offset, size)
}
