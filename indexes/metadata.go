package indexes

import (
	"bytes"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
)

type Metadata struct {
	Epoch     uint64
	RootCid   cid.Cid
	Network   Network
	IndexKind []byte
}

// Assert Epoch is x.
func (m *Metadata) AssertEpoch(x uint64) error {
	if m.Epoch != x {
		return fmt.Errorf("expected epoch %d, got %d", x, m.Epoch)
	}
	return nil
}

// Assert RootCid is x.
func (m *Metadata) AssertRootCid(x cid.Cid) error {
	if !m.RootCid.Equals(x) {
		return fmt.Errorf("expected root cid %s, got %s", x, m.RootCid)
	}
	return nil
}

// Assert Network is x.
func (m *Metadata) AssertNetwork(x Network) error {
	if m.Network != x {
		return fmt.Errorf("expected network %q, got %q", x, m.Network)
	}
	return nil
}

// Assert IndexKind is x.
func (m *Metadata) AssertIndexKind(x []byte) error {
	if !bytes.Equal(m.IndexKind, x) {
		return fmt.Errorf("expected index kind %q, got %q", x, m.IndexKind)
	}
	return nil
}

func setDefaultMetadata(index *compactindexsized.Builder, metadata *Metadata) error {
	if index == nil {
		return fmt.Errorf("index is nil")
	}
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}
	setter := index.Metadata()

	if err := setter.Add(indexmeta.MetadataKey_Epoch, Uint64tob(metadata.Epoch)); err != nil {
		return err
	}

	if metadata.RootCid == cid.Undef {
		return fmt.Errorf("root cid is undefined")
	}
	if err := setter.Add(indexmeta.MetadataKey_RootCid, metadata.RootCid.Bytes()); err != nil {
		return err
	}

	if !IsValidNetwork(metadata.Network) {
		return fmt.Errorf("invalid network")
	}
	if err := setter.Add(indexmeta.MetadataKey_Network, []byte(metadata.Network)); err != nil {
		return err
	}

	if len(metadata.IndexKind) == 0 {
		return fmt.Errorf("index kind is empty")
	}
	return setter.Add(indexmeta.MetadataKey_Kind, metadata.IndexKind)
}

// getDefaultMetadata gets and validates the metadata from the index.
// Will return an error if some of the metadata is missing.
func getDefaultMetadata(index *compactindexsized.DB) (*Metadata, error) {
	out := &Metadata{}
	meta := index.Header.Metadata

	indexKind, ok := meta.Get(indexmeta.MetadataKey_Kind)
	if ok {
		out.IndexKind = indexKind
	} else {
		return nil, fmt.Errorf("metadata.kind is empty (index kind)")
	}

	epochBytes, ok := meta.Get(indexmeta.MetadataKey_Epoch)
	if ok {
		out.Epoch = BtoUint64(epochBytes)
	} else {
		return nil, fmt.Errorf("metadata.epoch is empty")
	}

	rootCidBytes, ok := meta.Get(indexmeta.MetadataKey_RootCid)
	if ok {
		var err error
		out.RootCid, err = cid.Cast(rootCidBytes)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("metadata.rootCid is empty")
	}

	networkBytes, ok := meta.Get(indexmeta.MetadataKey_Network)
	if ok {
		out.Network = Network(networkBytes)
	} else {
		return nil, fmt.Errorf("metadata.network is empty")
	}

	return out, nil
}
