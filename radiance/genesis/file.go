package genesis

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/radiance/archiveutil"
)

// ReadGenesisFromFile is a convenience wrapper for ReadGenesisFromArchive.
func ReadGenesisFromFile(fpath string) (genesis *Genesis, hash *[32]byte, err error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return ReadGenesisFromArchive(f)
}

// ReadGenesisFromArchive reads a `genesis.tar.bz2` file.
func ReadGenesisFromArchive(archive io.Reader) (genesis *Genesis, hash *[32]byte, err error) {
	var files *tar.Reader
	var hdr *tar.Header
	files, err = archiveutil.OpenTar(archive)
	if err != nil {
		return
	}
	hdr, err = files.Next()
	if err != nil {
		return
	}
	if hdr.Name != "genesis.bin" {
		err = fmt.Errorf("first file is not genesis.bin")
		return
	}

	// Read and hash first file
	const maxSize = 10_000_001
	var rd io.Reader
	rd = files
	hasher := sha256.New()
	rd = io.TeeReader(rd, hasher)
	rd = io.LimitReader(rd, maxSize)

	// Decode content
	var genesisBytes []byte
	genesisBytes, err = io.ReadAll(rd)
	if err != nil {
		return
	}
	if len(genesisBytes) >= maxSize {
		err = fmt.Errorf("genesis.bin too large")
		return
	}
	genesis = new(Genesis)
	dec := bin.NewBinDecoder(genesisBytes)
	err = dec.Decode(genesis)
	if err == nil {
		if dec.HasRemaining() {
			err = fmt.Errorf("not all of genesis.bin was read (%d bytes remaining)", dec.Remaining())
		} else {
			hash = new([32]byte)
			hasher.Sum(hash[:0])
		}
	}

	return
}
