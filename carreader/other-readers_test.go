package carreader

import (
	"encoding/binary"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/valyala/bytebufferpool"
)

func buildSection(t *testing.T, data []byte) ([]byte, cid.Cid) {
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
	hdr := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(hdr, uint64(len(body)))
	return append(hdr[:n], body...), c
}

func TestParseNodeFromSectionBufferAcceptsIntactData(t *testing.T) {
	section, c := buildSection(t, []byte("a solana block's bytes"))
	got, err := ParseNodeFromSectionBuffer(&bytebufferpool.ByteBuffer{B: section}, &c)
	if err != nil {
		t.Fatalf("intact section rejected: %v", err)
	}
	if string(got.B) != "a solana block's bytes" {
		t.Fatalf("wrong data returned: %q", got.B)
	}
}

func TestParseNodeFromSectionBufferRejectsCorruptedData(t *testing.T) {
	section, c := buildSection(t, []byte("a solana block's bytes"))
	section[len(section)-1] ^= 0x01
	if _, err := ParseNodeFromSectionBuffer(&bytebufferpool.ByteBuffer{B: section}, &c); err == nil {
		t.Fatal("corrupted data was accepted; the CID is a content address, not a label")
	}
}

func TestParseNodeFromSectionBufferNilCidSkipsVerification(t *testing.T) {
	section, _ := buildSection(t, []byte("a solana block's bytes"))
	section[len(section)-1] ^= 0x01
	if _, err := ParseNodeFromSectionBuffer(&bytebufferpool.ByteBuffer{B: section}, nil); err != nil {
		t.Fatalf("nil wantedCid must not verify: %v", err)
	}
}
