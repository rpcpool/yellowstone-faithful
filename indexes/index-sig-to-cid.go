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
	"github.com/rpcpool/yellowstone-faithful/deprecated/compactindex36"
)

type SigToCid_Writer struct {
	sealed    bool
	tmpDir    string
	finalPath string
	meta      *Metadata
	index     *compactindexsized.Builder
}

const (
	// 36 bytes for cid
	IndexValueSize_SigToCid = 36
)

func formatFilename_SigToCid(epoch uint64, rootCid cid.Cid, network Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"sig-to-cid.index",
	)
}

var Kind_SigToCid = []byte("sig-to-cid")

func NewWriter_SigToCid(
	epoch uint64,
	rootCid cid.Cid,
	network Network,
	tmpDir string, // Where to put the temporary index files; WILL BE DELETED.
	numItems uint64,
) (*SigToCid_Writer, error) {
	if !IsValidNetwork(network) {
		return nil, ErrInvalidNetwork
	}
	if rootCid == cid.Undef {
		return nil, ErrInvalidRootCid
	}
	index, err := compactindexsized.NewBuilderSized(
		tmpDir,
		uint(numItems),
		IndexValueSize_SigToCid,
	)
	if err != nil {
		return nil, err
	}
	meta := &Metadata{
		Epoch:     epoch,
		RootCid:   rootCid,
		Network:   network,
		IndexKind: Kind_SigToCid,
	}
	return &SigToCid_Writer{
		tmpDir: tmpDir,
		meta:   meta,
		index:  index,
	}, nil
}

func (w *SigToCid_Writer) Put(sig solana.Signature, cid_ cid.Cid) error {
	if w.sealed {
		return fmt.Errorf("cannot put to sealed writer")
	}
	if cid_ == cid.Undef {
		return fmt.Errorf("cid is undefined")
	}
	key := sig[:]
	value := cid_.Bytes()
	return w.index.Insert(key, value)
}

func (w *SigToCid_Writer) Seal(ctx context.Context, dstDir string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}
	if err := setDefaultMetadata(w.index, w.meta); err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	filepath := filepath.Join(dstDir, formatFilename_SigToCid(w.meta.Epoch, w.meta.RootCid, w.meta.Network))
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

func (w *SigToCid_Writer) Close() error {
	if !w.sealed {
		return fmt.Errorf("attempted to close a sig-to-cid index that was not sealed")
	}
	return w.index.Close()
}

// GetFilepath returns the path to the sealed index file.
func (w *SigToCid_Writer) GetFilepath() string {
	return w.finalPath
}

type SigToCid_Reader struct {
	file            io.Closer
	meta            *Metadata
	index           *compactindexsized.DB
	deprecatedIndex *compactindex36.DB
}

func Open_SigToCid(filepath string) (*SigToCid_Reader, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	return OpenWithReader_SigToCid(file)
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

func OpenWithReader_SigToCid(reader ReaderAtCloser) (*SigToCid_Reader, error) {
	isOld, err := IsFileOldFormat(reader)
	if err != nil {
		return nil, err
	}
	if isOld {
		return OpenWithReader_SigToCid_Deprecated(reader)
	}
	index, err := compactindexsized.Open(reader)
	if err != nil {
		return nil, err
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
	if err := meta.AssertIndexKind(Kind_SigToCid); err != nil {
		return nil, err
	}
	return &SigToCid_Reader{
		file:  reader,
		meta:  meta,
		index: index,
	}, nil
}

func OpenWithReader_SigToCid_Deprecated(reader ReaderAtCloser) (*SigToCid_Reader, error) {
	index, err := compactindex36.Open(reader)
	if err != nil {
		return nil, err
	}
	return &SigToCid_Reader{
		file:            reader,
		deprecatedIndex: index,
	}, nil
}

func (r *SigToCid_Reader) IsDeprecatedOldVersion() bool {
	return r.deprecatedIndex != nil
}

func (r *SigToCid_Reader) Get(sig solana.Signature) (cid.Cid, error) {
	if sig.IsZero() {
		return cid.Undef, fmt.Errorf("sig is undefined")
	}
	if r.IsDeprecatedOldVersion() {
		key := sig[:]
		value, err := r.deprecatedIndex.Lookup(key)
		if err != nil {
			return cid.Undef, err
		}
		_, c, err := cid.CidFromBytes(value[:])
		if err != nil {
			return cid.Undef, err
		}
		return c, nil
	}
	key := sig[:]
	value, err := r.index.Lookup(key)
	if err != nil {
		return cid.Undef, err
	}
	_, c, err := cid.CidFromBytes(value[:])
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}

func (r *SigToCid_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *SigToCid_Reader) Meta() *Metadata {
	return r.meta
}

func (r *SigToCid_Reader) Prefetch(b bool) {
	if r.IsDeprecatedOldVersion() {
		r.deprecatedIndex.Prefetch(b)
		return
	}
	r.index.Prefetch(b)
}
