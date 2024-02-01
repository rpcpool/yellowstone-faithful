package indexes

import (
	"fmt"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/deprecated/compactindex"
)

type Deprecated_CidToOffset_Reader struct {
	file  io.Closer
	index *compactindex.DB
}

func Deprecated_Open_CidToOffset(file string) (*Deprecated_CidToOffset_Reader, error) {
	is, err := IsFileOldFormatByPath(file)
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, fmt.Errorf("not old format")
	}
	reader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	return Deprecated_OpenWithReader_CidToOffset(reader)
}

func Deprecated_OpenWithReader_CidToOffset(reader ReaderAtCloser) (*Deprecated_CidToOffset_Reader, error) {
	index, err := compactindex.Open(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	// meta, err := getDefaultMetadata(index)
	// if err != nil {
	// 	return nil, err
	// }
	// if !IsValidNetwork(meta.Network) {
	// 	return nil, fmt.Errorf("invalid network")
	// }
	// if meta.RootCid == cid.Undef {
	// 	return nil, fmt.Errorf("root cid is undefined")
	// }
	// if err := meta.AssertIndexKind(Kind_CidToOffset); err != nil {
	// 	return nil, err
	// }
	return &Deprecated_CidToOffset_Reader{
		file:  reader,
		index: index,
	}, nil
}

// Get returns the offset for the given cid.
func (r *Deprecated_CidToOffset_Reader) Get(cid_ cid.Cid) (uint64, error) {
	if cid_ == cid.Undef {
		return 0, fmt.Errorf("cid is undefined")
	}
	key := cid_.Bytes()
	return r.index.Lookup(key)
}

func (r *Deprecated_CidToOffset_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *Deprecated_CidToOffset_Reader) Meta() *Metadata {
	return nil
}

func (r *Deprecated_CidToOffset_Reader) Prefetch(b bool) {
	r.index.Prefetch(b)
}
