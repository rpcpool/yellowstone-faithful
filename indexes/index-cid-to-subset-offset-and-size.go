package indexes

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
)

type CidToSubsetOffsetAndSize_Writer struct {
	sealed    bool
	tmpDir    string
	finalPath string
	meta      *Metadata
	index     *compactindexsized.Builder
}

const (
	// 3 bytes for subset (uint24, max 16.7 MB (megabytes)),
	// 6 bytes for offset (uint48, max 281.5 TB (terabytes)),
	// 3 bytes for size (uint24, max 16.7 MB (megabytes), which is plenty considering the max object size is ~1 MB)
	IndexValueSize_CidToSubsetOffsetAndSize = 3 + 6 + 3
)

// todo: not sure if this file name is correct
func formatFilename_CidToSubsetOffsetAndSize(epoch uint64, network Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s",
		epoch,
		network,
		"cid-to-subset-offset-and-size.index",
	)
}

var Kind_CidToSubsetOffsetAndSize = []byte("cid-to-subset-offset-and-size")

func NewWriter_CidToSubsetOffsetAndSize(
	epoch uint64,
	network Network,
	tmpDir string,
	numItems uint64,
) (*CidToSubsetOffsetAndSize_Writer, error) {
	if !IsValidNetwork(network) {
		return nil, ErrInvalidNetwork
	}
	index, err := compactindexsized.NewBuilderSized(
		tmpDir,
		uint(numItems),
		IndexValueSize_CidToSubsetOffsetAndSize,
	)
	if err != nil {
		return nil, err
	}
	meta := &Metadata{
		Epoch:     epoch,
		Network:   network,
		IndexKind: Kind_CidToSubsetOffsetAndSize,
	}
	if err := setDefaultMetadata(index, meta); err != nil {
		return nil, err
	}
	return &CidToSubsetOffsetAndSize_Writer{
		tmpDir: tmpDir,
		meta:   meta,
		index:  index,
	}, nil
}

func (w *CidToSubsetOffsetAndSize_Writer) Put(cid_ cid.Cid, subset, offset, size uint64) error {
	if cid_ == cid.Undef {
		return fmt.Errorf("cid is undefined")
	}
	if subset > MaxUint24 {
		return fmt.Errorf("subset number is too large; max is %d, but got %d", MaxUint24, subset)
	}
	if offset > MaxUint48 {
		return fmt.Errorf("offset is too large; max is %d, but got %d", MaxUint48, offset)
	}
	if size > MaxUint24 {
		return fmt.Errorf("size is too large; max is %d, but got %d", MaxUint24, size)
	}

	key := cid_.Bytes()
	value := append(
		Uint24tob(uint32(subset)),
		append(
			Uint48tob(offset),
			Uint24tob(uint32(size))...,
		)...,
	)

	return w.index.Insert(key, value)
}

func (w *CidToSubsetOffsetAndSize_Writer) Seal(ctx context.Context, dstDir string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}

	filepath := filepath.Join(dstDir, formatFilename_CidToSubsetOffsetAndSize(w.meta.Epoch, w.meta.Network))
	w.finalPath = filepath

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	if err := w.index.Seal(ctx, file); err != nil {
		return fmt.Errorf("failed to seal index: %w", err)
	}

	w.sealed = true

	return nil
}

func (w *CidToSubsetOffsetAndSize_Writer) Close() error {
	if !w.sealed {
		return fmt.Errorf("attempted to close a cid-to-subset-offset-and-size index that was not sealed")
	}
	return w.index.Close()
}

func (w *CidToSubsetOffsetAndSize_Writer) GetFilePath() string {
	return w.finalPath
}

type CidToSubsetOffsetAndSize_Reader struct {
	file  io.Closer
	meta  *Metadata
	index *compactindexsized.DB
}

func Open_CidToSubsetOffsetAndSize(file string) (*CidToSubsetOffsetAndSize_Reader, error) {
	reader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}

	return OpenWithReader_CidToSubsetOffsetAndSize(reader)
}

func OpenWithReader_CidToSubsetOffsetAndSize(reader ReaderAtCloser) (*CidToSubsetOffsetAndSize_Reader, error) {
	index, err := compactindexsized.Open(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	meta, err := getDefaultMetadata(index)
	if err != nil {
		return nil, err
	}
	if !IsValidNetwork(meta.Network) {
		return nil, fmt.Errorf("invalid network")
	}
	if err := meta.AssertIndexKind(Kind_CidToSubsetOffsetAndSize); err != nil {
		return nil, err
	}
	return &CidToSubsetOffsetAndSize_Reader{
		file:  reader,
		meta:  meta,
		index: index,
	}, nil
}

func (r *CidToSubsetOffsetAndSize_Reader) Get(cid_ cid.Cid) (*SubsetOffsetAndSize, error) {
	if cid_ == cid.Undef {
		return nil, fmt.Errorf("cid is undefined")
	}
	key := cid_.Bytes()
	value, err := r.index.Lookup(key)
	if err != nil {
		return nil, err
	}
	soas := &SubsetOffsetAndSize{}
	if err := soas.FromBytes(value); err != nil {
		return nil, err
	}
	return soas, nil
}

func (r *CidToSubsetOffsetAndSize_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *CidToSubsetOffsetAndSize_Reader) Meta() *Metadata {
	return r.meta
}

func (r *CidToSubsetOffsetAndSize_Reader) Prefetch(b bool) {
	r.index.Prefetch(b)
}
