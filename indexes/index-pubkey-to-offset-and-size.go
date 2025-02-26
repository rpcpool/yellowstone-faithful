package indexes

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
)

type PubkeyToOffsetAndSize_Writer struct {
	sealed    bool
	tmpDir    string
	finalPath string
	meta      *Metadata
	index     *compactindexsized.Builder
}

const (
	// 6 bytes for offset (uint48, max 281.5 TB (terabytes)),
	// 3 bytes for size (uint24, max 16.7 MB (megabytes), which is plenty considering the max object size is ~1 MB)
	IndexValueSize_PubkeyToOffsetAndSize = 6 + 3
)

func FormatFilename_PubkeyToOffsetAndSize(epoch uint64, rootCid cid.Cid, network Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"pubkey-to-offset-and-size.index",
	)
}

var Kind_PubkeyToOffsetAndSize = []byte("pubkey-to-offset-and-size")

func NewWriter_PubkeyToOffsetAndSize(
	epoch uint64,
	rootCid cid.Cid,
	network Network,
	tmpDir string, // Where to put the temporary index files; WILL BE DELETED.
) (*PubkeyToOffsetAndSize_Writer, error) {
	if !IsValidNetwork(network) {
		return nil, ErrInvalidNetwork
	}
	if rootCid == cid.Undef {
		return nil, ErrInvalidRootCid
	}
	index, err := compactindexsized.NewBuilderSized(
		tmpDir,
		uint(1000000), // TODO: can this be not precise?
		IndexValueSize_PubkeyToOffsetAndSize,
	)
	if err != nil {
		return nil, err
	}
	meta := &Metadata{
		Epoch:     epoch,
		RootCid:   rootCid,
		Network:   network,
		IndexKind: Kind_PubkeyToOffsetAndSize,
	}
	if err := setDefaultMetadata(index, meta); err != nil {
		return nil, err
	}
	return &PubkeyToOffsetAndSize_Writer{
		tmpDir: tmpDir,
		meta:   meta,
		index:  index,
	}, nil
}

func (w *PubkeyToOffsetAndSize_Writer) Put(pk solana.PublicKey, offset uint64, size uint64) error {
	if offset > MaxUint48 {
		return fmt.Errorf("offset is too large; max is %d, but got %d", MaxUint48, offset)
	}
	if size > MaxUint24 {
		return fmt.Errorf("size is too large; max is %d, but got %d", MaxUint24, size)
	}
	key := pk.Bytes()
	value := append(Uint48tob(offset), Uint24tob(uint32(size))...)
	return w.index.Insert(key, value)
}

func (w *PubkeyToOffsetAndSize_Writer) Seal(ctx context.Context, dstDir string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}

	filepath := filepath.Join(dstDir, FormatFilename_PubkeyToOffsetAndSize(w.meta.Epoch, w.meta.RootCid, w.meta.Network))
	return w.SealWithFilename(ctx, filepath)
}

func (w *PubkeyToOffsetAndSize_Writer) SealWithFilename(ctx context.Context, dstFilepath string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}

	filepath := dstFilepath
	w.finalPath = filepath

	defer os.Rename(filepath+".tmp", filepath)

	file, err := os.Create(filepath + ".tmp")
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

func (w *PubkeyToOffsetAndSize_Writer) Close() error {
	if !w.sealed {
		return fmt.Errorf("attempted to close a pubkey-to-offset-and-size index that was not sealed")
	}
	return w.index.Close()
}

// GetFilepath returns the path to the sealed index file.
func (w *PubkeyToOffsetAndSize_Writer) GetFilepath() string {
	return w.finalPath
}

type PubkeyToOffsetAndSize_Reader struct {
	file  io.Closer
	meta  *Metadata
	index *compactindexsized.DB
}

func Open_PubkeyToOffsetAndSize(file string) (*PubkeyToOffsetAndSize_Reader, error) {
	reader, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	return OpenWithReader_PubkeyToOffsetAndSize(reader)
}

func OpenWithReader_PubkeyToOffsetAndSize(reader ReaderAtCloser) (*PubkeyToOffsetAndSize_Reader, error) {
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
	if err := meta.AssertIndexKind(Kind_PubkeyToOffsetAndSize); err != nil {
		return nil, err
	}
	return &PubkeyToOffsetAndSize_Reader{
		file:  reader,
		meta:  meta,
		index: index,
	}, nil
}

func (r *PubkeyToOffsetAndSize_Reader) Get(pk solana.PublicKey) (*OffsetAndSize, error) {
	key := pk.Bytes()
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

func (r *PubkeyToOffsetAndSize_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *PubkeyToOffsetAndSize_Reader) Meta() *Metadata {
	return r.meta
}

func (r *PubkeyToOffsetAndSize_Reader) Prefetch(b bool) {
	r.index.Prefetch(b)
}
