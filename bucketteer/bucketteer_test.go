package bucketteer

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/mmap"
)

func TestBucketteer(t *testing.T) {
	wr := NewWriter()
	firstSig := [64]byte{1, 2, 3, 4}
	wr.Push(firstSig)

	if !wr.Has(firstSig) {
		t.Fatal("expected to have firstSig")
	}
	{
		sig := [64]byte{1, 2, 3, 5}
		require.False(t, wr.Has(sig))
		wr.Push(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{1, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Push(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{22, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Push(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{99, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Push(sig)
		require.True(t, wr.Has(sig))
	}
	require.Equal(t, 3, len(wr.prefixToHashes))
	{
		fileContent := NewWriterWriterAtBuffer()
		_, err := wr.WriteTo(fileContent)
		require.NoError(t, err)
		reader := bin.NewBorshDecoder(fileContent.Bytes())

		// read header size:
		headerSize, err := reader.ReadUint32(bin.LE)
		require.NoError(t, err)
		_ = headerSize

		// magic:
		{
			magicBuf := [8]byte{}
			_, err := reader.Read(magicBuf[:])
			require.NoError(t, err)
			require.Equal(t, _Magic, magicBuf)
		}
		// version:
		{
			got, err := reader.ReadUint64(bin.LE)
			require.NoError(t, err)
			require.Equal(t, Version, got)
		}
		// numPrefixes:
		numPrefixes, err := reader.ReadUint64(bin.LE)
		require.NoError(t, err)
		require.Equal(t, uint64(3), numPrefixes)
		// prefix -> offset:
		prefixToOffset := make(map[[2]byte]uint64)
		{
			for i := 0; i < int(numPrefixes); i++ {
				var prefix [2]byte
				_, err := reader.Read(prefix[:])
				require.NoError(t, err)
				offset, err := reader.ReadUint64(bin.LE)
				require.NoError(t, err)
				prefixToOffset[prefix] = offset
			}
		}
		{
			require.Equal(t,
				map[[2]uint8]uint64{
					{0x1, 0x2}:  0x0,
					{0x16, 0x2}: 0x1c,
					{0x63, 0x2}: 0x28,
				}, prefixToOffset)
		}
		contentBuf, err := reader.ReadNBytes(reader.Remaining())
		require.NoError(t, err)
		contentReader := bin.NewBorshDecoder(contentBuf)
		{
			for prefix, offset := range prefixToOffset {
				// Now read the bucket:
				{
					err := contentReader.SetPosition(uint(offset))
					require.NoError(t, err)
					numHashes, err := contentReader.ReadUint32(bin.LE)
					require.NoError(t, err)
					switch prefix {
					case [2]byte{1, 2}:
						require.Equal(t, uint32(3), numHashes)
					case [2]byte{22, 2}:
						require.Equal(t, uint32(1), numHashes)
					case [2]byte{99, 2}:
						require.Equal(t, uint32(1), numHashes)
					}

					for i := 0; i < int(numHashes); i++ {
						hash, err := contentReader.ReadUint64(bin.LE)
						require.NoError(t, err)
						found := false
						for _, h := range wr.prefixToHashes[prefix] {
							if h == hash {
								found = true
								break
							}
						}
						require.True(t, found)
					}
				}
			}
		}
		{
			// create temp file:
			path := filepath.Join(os.TempDir(), "test-bucketteer")
			{
				fWrite, err := os.Create(path)
				require.NoError(t, err)
				_, err = fileContent.WriteTo(fWrite)
				require.NoError(t, err)
				fWrite.Close()
			}
			// read temp file:
			require.NoError(t, err)
			mmr, err := mmap.Open(path)
			require.NoError(t, err)
			defer mmr.Close()
			reader, err := NewReader(mmr)
			require.NoError(t, err)
			ok, err := reader.Has(firstSig)
			require.NoError(t, err)
			require.True(t, ok)
		}
	}
}

type WriterWriterAtBuffer struct {
	buf []byte
}

func NewWriterWriterAtBuffer() *WriterWriterAtBuffer {
	return &WriterWriterAtBuffer{}
}

func (w *WriterWriterAtBuffer) WriteAt(p []byte, off int64) (n int, err error) {
	if off+int64(len(p)) > int64(len(w.buf)) {
		w.buf = append(w.buf, make([]byte, int(off)+len(p)-len(w.buf))...)
	}
	copy(w.buf[off:], p)
	return len(p), nil
}

// Write implements io.Writer.
func (w *WriterWriterAtBuffer) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *WriterWriterAtBuffer) Bytes() []byte {
	return w.buf
}

func (w *WriterWriterAtBuffer) Close() error {
	return nil
}

// WriteTo
func (w *WriterWriterAtBuffer) WriteTo(out io.Writer) (int64, error) {
	n, err := out.Write(w.buf)
	return int64(n), err
}
