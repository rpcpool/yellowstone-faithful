package nodetools

import (
	"encoding/binary"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
)

func appendSection(t *testing.T, dst []byte, data []byte) ([]byte, cid.Cid) {
	t.Helper()
	c, err := cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: 32,
	}.Sum(data)
	if err != nil {
		t.Fatalf("sum cid: %v", err)
	}
	body := append(append([]byte{}, c.Bytes()...), data...)
	dst = binary.AppendUvarint(dst, uint64(len(body)))
	return append(dst, body...), c
}

func buildSections(t *testing.T) []byte {
	t.Helper()
	var sections []byte
	for _, data := range []string{"tx one", "tx two", "tx three"} {
		sections, _ = appendSection(t, sections, []byte(data))
	}
	return sections
}

func TestSplitIntoDataAndCidsAcceptsIntactSections(t *testing.T) {
	nodes, err := SplitIntoDataAndCids(buildSections(t))
	if err != nil {
		t.Fatalf("intact sections rejected: %v", err)
	}
	defer nodes.Put()
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	if got := string(nodes[1].Data.Bytes()); got != "tx two" {
		t.Fatalf("wrong data for node 1: %q", got)
	}
}

func TestSplitIntoDataAndCidsRejectsOneCorruptedNode(t *testing.T) {
	var sections []byte
	sections, _ = appendSection(t, sections, []byte("tx one"))
	sections, _ = appendSection(t, sections, []byte("tx two"))
	// Flip the last data byte of the middle node; its header stays intact.
	sections[len(sections)-1] ^= 0x01
	sections, _ = appendSection(t, sections, []byte("tx three"))
	if _, err := SplitIntoDataAndCids(sections); err == nil {
		t.Fatal("corrupted node was accepted; the CID is a content address, not a label")
	}
}

func TestSplitIntoDataAndCidsRejectsTruncatedSection(t *testing.T) {
	sections := buildSections(t)
	if _, err := SplitIntoDataAndCids(sections[:len(sections)-2]); err == nil {
		t.Fatal("truncated section was accepted")
	}
}
