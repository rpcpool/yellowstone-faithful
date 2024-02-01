package bucketteer

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/mmap"
)

func TestBucketteer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-bucketteer")
	wr, err := NewWriter(path)
	require.NoError(t, err)
	firstSig := [64]byte{1, 2, 3, 4}
	wr.Put(firstSig)

	if !wr.Has(firstSig) {
		t.Fatal("expected to have firstSig")
	}
	{
		sig := [64]byte{1, 2, 3, 5}
		require.False(t, wr.Has(sig))
		wr.Put(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{1, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Put(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{22, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Put(sig)
		require.True(t, wr.Has(sig))
	}
	{
		sig := [64]byte{99, 2, 3, 6}
		require.False(t, wr.Has(sig))
		wr.Put(sig)
		require.True(t, wr.Has(sig))
	}
	require.Equal(t, 65536, len(wr.prefixToHashes))
	{
		meta := indexmeta.Meta{}
		meta.Add([]byte("epoch"), []byte("test"))
		gotSize, err := wr.Seal(meta)
		require.NoError(t, err)
		require.NoError(t, wr.Close())
		realSize, err := getFizeSize(path)
		require.NoError(t, err)
		require.Equal(t, realSize, gotSize)

		fileContent, err := os.ReadFile(path)
		require.NoError(t, err)

		reader := bin.NewBorshDecoder(fileContent)

		// read header size:
		headerSize, err := reader.ReadUint32(bin.LE)
		require.NoError(t, err)
		require.Equal(t, uint32(8+8+8+(1+(1+5)+(1+4))+(65536*(2+8))), headerSize)

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
		{
			// read meta:
			numMeta, err := reader.ReadUint8()
			require.NoError(t, err)
			require.Equal(t, uint8(1), numMeta)

			keyLen, err := reader.ReadUint8()
			require.NoError(t, err)
			require.Equal(t, uint8(5), keyLen)

			key := make([]byte, keyLen)
			_, err = reader.Read(key)
			require.NoError(t, err)
			require.Equal(t, "epoch", string(key))

			valueLen, err := reader.ReadUint8()
			require.NoError(t, err)
			require.Equal(t, uint8(4), valueLen)

			value := make([]byte, valueLen)
			_, err = reader.Read(value)
			require.NoError(t, err)
			require.Equal(t, "test", string(value))
		}
		// numPrefixes:
		numPrefixes, err := reader.ReadUint64(bin.LE)
		require.NoError(t, err)
		require.Equal(t, uint64(65536), numPrefixes)
		// prefix -> offset:
		prefixToOffset := [math.MaxUint16 + 1]uint64{}
		{
			for i := 0; i < int(numPrefixes); i++ {
				var prefix [2]byte
				_, err := reader.Read(prefix[:])
				require.NoError(t, err)
				offset, err := reader.ReadUint64(bin.LE)
				require.NoError(t, err)
				prefixToOffset[prefixToUint16(prefix)] = offset
			}
		}
		{
			require.Equal(t, 65536, len(prefixToOffset))

			require.Equal(t, uint64(0x804), prefixToOffset[prefixToUint16([2]byte{0x1, 0x2})])
			require.Equal(t, uint64(0x870), prefixToOffset[prefixToUint16([2]byte{0x16, 0x2})])
			require.Equal(t, uint64(0x9ac), prefixToOffset[prefixToUint16([2]byte{0x63, 0x2})])
		}
		contentBuf, err := reader.ReadNBytes(reader.Remaining())
		require.NoError(t, err)
		// require.Equal(t,
		// 	[]byte{
		// 		0x3, 0x0, 0x0, 0x0, // num entries
		// 		0x49, 0xd7, 0xaf, 0x9e, 0x94, 0x4d, 0x9a, 0x6f,
		// 		0x2f, 0x12, 0xdb, 0x5b, 0x1, 0x62, 0xae, 0x1a,
		// 		0x3b, 0xb6, 0x71, 0x5f, 0x4, 0x4f, 0x36, 0xf2,
		// 		0x1, 0x0, 0x0, 0x0, // num entries
		// 		0x58, 0xe1, 0x9d, 0xde, 0x7c, 0xfb, 0xeb, 0x5a,
		// 		0x1, 0x0, 0x0, 0x0, // num entries
		// 		0x4c, 0xbd, 0xa3, 0xed, 0xd3, 0x8b, 0xa8, 0x44,
		// 	},
		// 	contentBuf,
		// )
		contentReader := bin.NewBorshDecoder(contentBuf)
		{
			for prefixAsNumber, offset := range prefixToOffset {
				prefix := uint16ToPrefix(uint16(prefixAsNumber))
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
						for _, h := range wr.prefixToHashes[prefixToUint16(prefix)] {
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

			got, ok := reader.Meta().Get(indexmeta.MetadataKey_Epoch)
			require.True(t, ok)
			require.Equal(t, []byte("test"), got)
		}
	}
}

func getFizeSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
