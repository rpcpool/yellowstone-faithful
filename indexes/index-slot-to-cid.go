package indexes

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/deprecated/compactindex36"
)

type SlotToCid_Writer struct {
	sealed    bool
	tmpDir    string
	finalPath string
	meta      *Metadata
	index     *compactindexsized.Builder
}

const (
	// 36 bytes for cid
	IndexValueSize_SlotToCid = 36
)

func formatFilename_SlotToCid(epoch uint64, rootCid cid.Cid, network Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"slot-to-cid.index",
	)
}

var Kind_SlotToCid = []byte("slot-to-cid")

const SLOTS_PER_EPOCH = 432000

func NewWriter_SlotToCid(
	epoch uint64,
	rootCid cid.Cid,
	network Network,
	tmpDir string, // Where to put the temporary index files; WILL BE DELETED.
) (*SlotToCid_Writer, error) {
	if !IsValidNetwork(network) {
		return nil, ErrInvalidNetwork
	}
	if rootCid == cid.Undef {
		return nil, ErrInvalidRootCid
	}
	index, err := compactindexsized.NewBuilderSized(
		tmpDir,
		uint(SLOTS_PER_EPOCH),
		IndexValueSize_SlotToCid,
	)
	if err != nil {
		return nil, err
	}
	meta := &Metadata{
		Epoch:     epoch,
		RootCid:   rootCid,
		Network:   network,
		IndexKind: Kind_SlotToCid,
	}
	if err := setDefaultMetadata(index, meta); err != nil {
		return nil, err
	}
	return &SlotToCid_Writer{
		tmpDir: tmpDir,
		meta:   meta,
		index:  index,
	}, nil
}

func (w *SlotToCid_Writer) Put(slot uint64, cid_ cid.Cid) error {
	if w.sealed {
		return fmt.Errorf("cannot put to sealed writer")
	}
	if cid_ == cid.Undef {
		return fmt.Errorf("cid is undefined")
	}
	key := Uint64tob(slot)
	value := cid_.Bytes()
	return w.index.Insert(key, value)
}

func (w *SlotToCid_Writer) Seal(ctx context.Context, dstDir string) error {
	if w.sealed {
		return fmt.Errorf("already sealed")
	}

	filepath := filepath.Join(dstDir, formatFilename_SlotToCid(w.meta.Epoch, w.meta.RootCid, w.meta.Network))
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

func (w *SlotToCid_Writer) Close() error {
	if !w.sealed {
		return fmt.Errorf("attempted to close a slot-to-cid index that was not sealed")
	}
	return w.index.Close()
}

// GetFilepath returns the path to the sealed index file.
func (w *SlotToCid_Writer) GetFilepath() string {
	return w.finalPath
}

type SlotToCid_Reader struct {
	file            io.Closer
	meta            *Metadata
	index           *compactindexsized.DB
	deprecatedIndex *compactindex36.DB
}

func Open_SlotToCid(filepath string) (*SlotToCid_Reader, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	return OpenWithReader_SlotToCid(file)
}

func OpenWithReader_SlotToCid(reader ReaderAtCloser) (*SlotToCid_Reader, error) {
	isOld, err := IsFileOldFormat(reader)
	if err != nil {
		return nil, err
	}
	if isOld {
		return OpenWithReader_SlotToCid_Deprecated(reader)
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
	if err := meta.AssertIndexKind(Kind_SlotToCid); err != nil {
		return nil, err
	}
	return &SlotToCid_Reader{
		file:  reader,
		meta:  meta,
		index: index,
	}, nil
}

func OpenWithReader_SlotToCid_Deprecated(reader ReaderAtCloser) (*SlotToCid_Reader, error) {
	index, err := compactindex36.Open(reader)
	if err != nil {
		return nil, err
	}
	return &SlotToCid_Reader{
		file:            reader,
		deprecatedIndex: index,
	}, nil
}

func (r *SlotToCid_Reader) IsDeprecatedOldVersion() bool {
	return r.deprecatedIndex != nil
}

func (r *SlotToCid_Reader) Get(slot uint64) (cid.Cid, error) {
	if r.IsDeprecatedOldVersion() {
		key := Uint64tob(slot)
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
	key := Uint64tob(slot)
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

func (r *SlotToCid_Reader) Close() error {
	return r.file.Close()
}

// Meta returns the metadata for the index.
func (r *SlotToCid_Reader) Meta() *Metadata {
	return r.meta
}

func (r *SlotToCid_Reader) Prefetch(b bool) {
	if r.IsDeprecatedOldVersion() {
		r.deprecatedIndex.Prefetch(b)
		return
	}
	r.index.Prefetch(b)
}
