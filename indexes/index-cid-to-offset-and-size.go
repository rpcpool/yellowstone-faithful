package indexes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
)

type CidToOffsetAndSize_Writer struct {
	sealed    bool
	tmpDir    string
	finalPath string
	meta      *Metadata
	index     *compactindexsized.Builder
}

const (
	// 6 bytes for offset (uint48, max 281.5 TB (terabytes)),
	// 3 bytes for size (uint24, max 16.7 MB (megabytes), which is plenty considering the max object size is ~1 MB)
	IndexValueSize_CidToOffsetAndSize = 6 + 3
)

func formatFilename_CidToOffsetAndSize(epoch uint64, rootCid cid.Cid, network Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"cid-to-offset-and-size.index",
	)
}

var Kind_CidToOffsetAndSize = []byte("cid-to-offset-and-size")

func NewWriter_CidToOffsetAndSize(
	epoch uint64,
	rootCid cid.Cid,
	network Network,
	tmpDir string, // Where to put the temporary index files; WILL BE DELETED.
	numItems uint64,
) (*CidToOffsetAndSize_Writer, error) {
	if !IsValidNetwork(network) {
		return nil, ErrInvalidNetwork
	}
	if rootCid == cid.Undef {
		return nil, ErrInvalidRootCid
	}
	index, err := compactindexsized.NewBuilderSized(
		tmpDir,
		uint(numItems),
		IndexValueSize_CidToOffsetAndSize,
	)
	if err != nil {
		return nil, err
	}
	meta := &Metadata{
		Epoch:     epoch,
		RootCid:   rootCid,
		Network:   network,
		IndexKind: Kind_CidToOffsetAndSize,
	}
	if err := setDefaultMetadata(index, meta); err != nil {
		return nil, err
	}
	return &CidToOffsetAndSize_Writer{
		tmpDir: tmpDir,
		meta:   meta,
		index:  index,
	}, nil
}

func (w *CidToOffsetAndSize_Writer) Put(cid_ cid.Cid, offset uint64, size uint64) error {
	if cid_ == cid.Undef {
		return fmt.Errorf("cid is undefined")
	}
	if offset > maxUint48 {
		return fmt.Errorf("offset is too large; max is %d, but got %d", maxUint48, offset)
	}
	if size > maxUint24 {
		return fmt.Errorf("size is too large; max is %d, but got %d", maxUint24, size)
	}
	key := cid_.Bytes()
	value := append(uint48tob(offset), uint24tob(uint32(size))...)
	return w.index.Insert(key, value)
}

func (w *CidToOffsetAndSize_Writer) Seal(ctx context.Context, dstDir string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}

	filepath := filepath.Join(dstDir, formatFilename_CidToOffsetAndSize(w.meta.Epoch, w.meta.RootCid, w.meta.Network))
	w.finalPath = filepath

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := w.index.Seal(ctx, file); err != nil {
		return fmt.Errorf("failed to seal index: %w", err)
	}
	w.sealed = true

	return nil
}

func (w *CidToOffsetAndSize_Writer) Close() error {
	if !w.sealed {
		return fmt.Errorf("attempted to close a cid-to-offset-and-size index that was not sealed")
	}
	return w.index.Close()
}

// GetFilepath returns the path to the sealed index file.
func (w *CidToOffsetAndSize_Writer) GetFilepath() string {
	return w.finalPath
}

func NewOffsetAndSize(offset uint64, size uint64) *OffsetAndSize {
	return &OffsetAndSize{
		Offset: offset,
		Size:   size,
	}
}

// IsValid returns true if the offset and size are valid.
func (oas *OffsetAndSize) IsValid() bool {
	return oas.Offset <= maxUint48 && oas.Size <= maxUint24
}

type OffsetAndSize struct {
	Offset uint64 // uint48, 6 bytes, max 281.5 TB (terabytes)
	Size   uint64 // uint24, 3 bytes, max 16.7 MB (megabytes)
}

// Bytes returns the offset and size as a byte slice.
func (oas *OffsetAndSize) Bytes() []byte {
	return append(uint48tob(oas.Offset), uint24tob(uint32(oas.Size))...)
}

// FromBytes parses the offset and size from a byte slice.
func (oas *OffsetAndSize) FromBytes(buf []byte) error {
	if len(buf) != IndexValueSize_CidToOffsetAndSize {
		return errors.New("invalid byte slice length")
	}
	_ = buf[IndexValueSize_CidToOffsetAndSize-1] // bounds check hint to compiler
	oas.Offset = btoUint48(buf[:6])
	oas.Size = uint64(btoUint24(buf[6:]))
	return nil
}

type CidToOffsetAndSize_Reader struct {
	file  io.Closer
	meta  *Metadata
	index *compactindexsized.DB
}

func Open_CidToOffsetAndSize(file string) (*CidToOffsetAndSize_Reader, error) {
	reader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	return OpenWithReader_CidToOffsetAndSize(reader)
}

func OpenWithReader_CidToOffsetAndSize(reader ReaderAtCloser) (*CidToOffsetAndSize_Reader, error) {
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
	if meta.RootCid == cid.Undef {
		return nil, fmt.Errorf("root cid is undefined")
	}
	if err := meta.AssertIndexKind(Kind_CidToOffsetAndSize); err != nil {
		return nil, err
	}
	return &CidToOffsetAndSize_Reader{
		file:  reader,
		meta:  meta,
		index: index,
	}, nil
}

func (r *CidToOffsetAndSize_Reader) Get(cid_ cid.Cid) (*OffsetAndSize, error) {
	if cid_ == cid.Undef {
		return nil, fmt.Errorf("cid is undefined")
	}
	key := cid_.Bytes()
	value, err := r.index.Lookup(key)
	if err != nil {
		return nil, err
	}
	oas := &OffsetAndSize{}
	if err := oas.FromBytes(value); err != nil {
		return nil, err
	}
	return oas, nil
}

func (r *CidToOffsetAndSize_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *CidToOffsetAndSize_Reader) Meta() *Metadata {
	return r.meta
}

func (r *CidToOffsetAndSize_Reader) Prefetch(b bool) {
	r.index.Prefetch(b)
}
