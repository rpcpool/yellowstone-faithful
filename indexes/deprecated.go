package indexes

import (
	"fmt"
	"io"
	"os"

	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
)

var oldMagic = [8]byte{'r', 'd', 'c', 'e', 'c', 'i', 'd', 'x'}

func IsOldMagic(magicBytes [8]byte) bool {
	return magicBytes == oldMagic
}

func IsFileOldFormatByPath(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	return IsFileOldFormat(file)
}

func IsFileOldFormat(file io.ReaderAt) (bool, error) {
	var magic [8]byte
	if _, err := file.ReadAt(magic[:], 0); err != nil {
		return false, fmt.Errorf("failed to read magic: %w", err)
	}
	return IsOldMagic(magic), nil
}

func IsFileNewFormat(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	var magic [8]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		return false, fmt.Errorf("failed to read magic: %w", err)
	}
	return magic == compactindexsized.Magic, nil
}
