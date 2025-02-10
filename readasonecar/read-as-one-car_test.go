package readasonecar

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
	carv1 "github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/carreader"
)

func TestReader(t *testing.T) {
	// create 3 car files
	file1 := writeDummyCar(t, 0, "file1.car")
	file2 := writeDummyCar(t, 2, "file2.car")
	file3 := writeDummyCar(t, 4, "file3.car")

	// create a multi reader
	mr, err := NewMultiReader(file1, file2, file3)
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	// read the first car
	{
		sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
		if !ok {
			t.Fatalf("expected true, got false")
		}
		_, sectionSize, data, err := mr.NextNodeBytes()
		if err != nil {
			t.Fatal(err)
		}
		thisData := newData(0)
		expectedSectionSize := sizeOfSection(thisData)
		if sectionSize != expectedSectionSize {
			t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
		}
		if !bytes.Equal(data, thisData) {
			t.Fatalf("expected data %s, got %s", thisData, data)
		}
		{
			section := make([]byte, expectedSectionSize)
			_, err = mr.ReadAt(section, int64(sectionOffset))
			if err != nil {
				t.Fatal(err)
			}
			_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotData, data) {
				t.Fatalf("expected data %s, got %s", data, gotData)
			}
			if !bytes.Equal(thisData, data) {
				t.Fatalf("expected data %s, got %s", thisData, gotData)
			}
		}
	}
	{
		sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
		if !ok {
			t.Fatalf("expected true, got false")
		}
		_, sectionSize, data, err := mr.NextNodeBytes()
		if err != nil {
			t.Fatal(err)
		}
		thisData := newData(1)
		expectedSectionSize := sizeOfSection(thisData)
		if sectionSize != expectedSectionSize {
			t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
		}
		if !bytes.Equal(data, thisData) {
			t.Fatalf("expected data %s, got %s", thisData, data)
		}
		{
			section := make([]byte, expectedSectionSize)
			_, err = mr.ReadAt(section, int64(sectionOffset))
			if err != nil {
				t.Fatal(err)
			}
			_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotData, data) {
				t.Fatalf("expected data %s, got %s", data, gotData)
			}
			if !bytes.Equal(thisData, data) {
				t.Fatalf("expected data %s, got %s", thisData, gotData)
			}
		}
	}
	{
		{
			sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
			if !ok {
				t.Fatalf("expected true, got false")
			}
			_, sectionSize, data, err := mr.NextNodeBytes()
			if err != nil {
				t.Fatal(err)
			}
			thisData := newData(2)
			expectedSectionSize := sizeOfSection(thisData)
			if sectionSize != expectedSectionSize {
				t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
			}
			if !bytes.Equal(data, thisData) {
				t.Fatalf("expected data %s, got %s", thisData, data)
			}

			{
				section := make([]byte, expectedSectionSize)
				_, err = mr.ReadAt(section, int64(sectionOffset))
				if err != nil {
					t.Fatal(err)
				}
				_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotData, data) {
					t.Fatalf("expected data %s, got %s", data, gotData)
				}
				if !bytes.Equal(thisData, data) {
					t.Fatalf("expected data %s, got %s", thisData, gotData)
				}
			}
		}
		{
			sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
			if !ok {
				t.Fatalf("expected true, got false")
			}
			_, sectionSize, data, err := mr.NextNodeBytes()
			if err != nil {
				t.Fatal(err)
			}
			thisData := newData(3)
			expectedSectionSize := sizeOfSection(thisData)
			if sectionSize != expectedSectionSize {
				t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
			}
			if !bytes.Equal(data, thisData) {
				t.Fatalf("expected data %s, got %s", thisData, data)
			}

			{
				section := make([]byte, expectedSectionSize)
				_, err = mr.ReadAt(section, int64(sectionOffset))
				if err != nil {
					t.Fatal(err)
				}
				_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotData, data) {
					t.Fatalf("expected data %s, got %s", data, gotData)
				}
				if !bytes.Equal(thisData, data) {
					t.Fatalf("expected data %s, got %s", thisData, gotData)
				}
			}
		}
	}
	{
		{
			sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
			if !ok {
				t.Fatalf("expected true, got false")
			}
			_, sectionSize, data, err := mr.NextNodeBytes()
			if err != nil {
				t.Fatal(err)
			}
			thisData := newData(4)
			expectedSectionSize := sizeOfSection(thisData)
			if sectionSize != expectedSectionSize {
				t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
			}
			if !bytes.Equal(data, thisData) {
				t.Fatalf("expected data %s, got %s", thisData, data)
			}

			{
				section := make([]byte, expectedSectionSize)
				_, err = mr.ReadAt(section, int64(sectionOffset))
				if err != nil {
					t.Fatal(err)
				}
				_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotData, data) {
					t.Fatalf("expected data %s, got %s", data, gotData)
				}
				if !bytes.Equal(thisData, data) {
					t.Fatalf("expected data %s, got %s", thisData, gotData)
				}
			}
		}
		{
			sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
			if !ok {
				t.Fatalf("expected true, got false")
			}
			_, sectionSize, data, err := mr.NextNodeBytes()
			if err != nil {
				t.Fatal(err)
			}
			thisData := newData(5)
			expectedSectionSize := sizeOfSection(thisData)
			if sectionSize != expectedSectionSize {
				t.Fatalf("expected size %d, got %d", expectedSectionSize, sectionSize)
			}
			if !bytes.Equal(data, thisData) {
				t.Fatalf("expected data %s, got %s", thisData, data)
			}

			{
				section := make([]byte, expectedSectionSize)
				_, err = mr.ReadAt(section, int64(sectionOffset))
				if err != nil {
					t.Fatal(err)
				}
				_, _, gotData, err := carreader.ReadNodeInfoWithData(bufio.NewReader(bytes.NewReader(section)))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotData, data) {
					t.Fatalf("expected data %s, got %s", data, gotData)
				}
				if !bytes.Equal(thisData, data) {
					t.Fatalf("expected data %s, got %s", thisData, gotData)
				}
			}
		}
	}

	// eof
	{
		sectionOffset, ok := mr.GetGlobalOffsetForNextRead()
		if ok {
			t.Fatalf("expected false, got true")
		}
		// must be 0
		expect := 137 + 137 + 137
		if sectionOffset != uint64(expect) {
			t.Fatalf("expected %d, got %d", expect, sectionOffset)
		}
		_, _, _, err := mr.NextNodeBytes()
		// expect EOF
		if err == nil || err.Error() != "EOF" {
			t.Fatalf("expected EOF, got %v", err)
		}
	}
}

func writeDummyCar(t *testing.T, start int, name string) string {
	data, err := newDummyCar(start)
	if err != nil {
		t.Fatal(err)
	}
	return writeData(t, data, name)
}

func writeData(t *testing.T, data []byte, name string) string {
	// create a temporary file
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	expectedHeaderSize = uint64(59)
)

var CBOR_SHA256_DUMMY_CID = cid.MustParse("bafyreics5uul5lbtxslcigtoa5fkba7qgwu7cyb7ih7z6fzsh4lgfgraau")

func newDummyCar(start int) ([]byte, error) {
	var buf bytes.Buffer
	header := &carv1.CarHeader{
		Roots:   []cid.Cid{CBOR_SHA256_DUMMY_CID},
		Version: 1,
	}
	if err := carv1.WriteHeader(header, &buf); err != nil {
		return nil, fmt.Errorf("failed to write header: %w", err)
	}
	headerSize := uint64(buf.Len())
	if headerSize != expectedHeaderSize {
		return nil, fmt.Errorf("header size is not %d, got %d", expectedHeaderSize, headerSize)
	}

	blocksData := [][]byte{
		newData(start),
		newData(start + 1),
	}

	for _, data := range blocksData {
		b := blocks.NewBlock(data)
		if err := writeSection(&buf, b); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func sizeOfSection(data []byte) uint64 {
	b := blocks.NewBlock(data)
	buf := new(bytes.Buffer)
	if err := writeSection(buf, b); err != nil {
		panic(err)
	}
	return uint64(buf.Len())
}

func newData(i int) []byte {
	return []byte("foo" + strconv.Itoa(i))
}

func writeSection(dst *bytes.Buffer, b *blocks.BasicBlock) error {
	// write a dummy block
	length := b.Cid().ByteLen() + len(b.RawData())
	if _, err := dst.Write(leb128.FromUInt64(uint64(length))); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}
	if _, err := dst.Write(b.Cid().Bytes()); err != nil {
		return fmt.Errorf("failed to write cid: %w", err)
	}
	_, err := dst.Write(b.RawData())
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	return nil
}
