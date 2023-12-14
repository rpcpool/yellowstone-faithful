package indexes_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/stretchr/testify/require"
)

func TestOffsetAndSize(t *testing.T) {
	v := indexes.OffsetAndSize{
		Offset: 123,
		Size:   456,
	}
	encoded := v.Bytes()
	require.Equal(t, []byte{0x7b, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc8, 0x01, 0x00}, encoded)

	var decoded indexes.OffsetAndSize
	require.NoError(t, decoded.FromBytes(encoded))

	require.Equal(t, v, decoded)
}

func TestCidToOffsetAndSize(t *testing.T) {
	// create a new index
	// write some data to it
	// close it
	// open it
	// read the data back
	// assert that the data is correct
	epoch := uint64(123)
	cstr := "bafyreids2hw6eynl4vag3cdp535sxz6zp6tedhuv6xu3k3rze3fskqy4yy"
	rootCid, err := cid.Parse(cstr)
	require.NoError(t, err)
	numItems := uint64(10000)

	dstDir := t.TempDir()
	writer, err := indexes.NewWriter_CidToOffsetAndSize(
		epoch,
		rootCid,
		indexes.NetworkMainnet,
		"",
		numItems,
	)
	require.NoError(t, err)
	require.NotNil(t, writer)

	// write some data to the index
	cid1_, err := cid.Parse("bafyreibwvjchy4qq6tqeqg4olawpzs3cphr7nqp5gz2ch5bnttt2ajg6p4")
	require.NoError(t, err)
	require.NoError(t, writer.Put(cid1_, 123, 456))

	cid2_, err := cid.Parse("bafyreibqlzq4vrezlbgn7qqgz36tx5itaelyxw4v2xyjho5fqqlrslf2vq")
	require.NoError(t, err)
	require.NoError(t, writer.Put(cid2_, 123456, 456789))

	cid3_, err := cid.Parse("bafyreiciqiiofeu74nt4drrw6pysqaethngzjtlbsyskvjmntqzx4fzv7q")
	require.NoError(t, err)
	require.NoError(t, writer.Put(cid3_, 123456789, 4567))

	{
		// add other 997 items
		for i := uint64(0); i < numItems-3; i++ {
			cid_ := cid.NewCidV1(cid.Raw, []byte(fmt.Sprintf("cid-%d", i)))
			require.NoError(t, err)
			require.NoError(t, writer.Put(cid_, i, i))
		}
	}
	{
		// if try to close the index before sealing it, it should fail
		require.Error(t, writer.Close())
	}

	// seal the index
	require.NoError(t, writer.Seal(context.TODO(), dstDir))
	t.Log(writer.GetFilepath())
	{
		files, err := os.ReadDir(dstDir)
		require.NoError(t, err)
		// should contain the index file
		has := false
		for _, file := range files {
			// check if file exists
			completePath := filepath.Join(dstDir, file.Name())
			file, err := os.Stat(completePath)
			require.NoError(t, err)
			// check if file is not empty
			require.NotZero(t, file.Size())

			if completePath == writer.GetFilepath() {
				has = true
			}
		}
		require.True(t, has)
	}

	finalFilepath := writer.GetFilepath()
	require.NotEmpty(t, finalFilepath)

	// close the index
	require.NoError(t, writer.Close())

	// open the index
	reader, err := indexes.Open_CidToOffsetAndSize(finalFilepath)
	require.NoError(t, err)
	require.NotNil(t, reader)

	// read the data back
	{
		offsetAndSize, err := reader.Get(cid1_)
		require.NoError(t, err)
		require.NotNil(t, offsetAndSize)

		require.Equal(t, uint64(123), offsetAndSize.Offset)
		require.Equal(t, uint64(456), offsetAndSize.Size)
	}
	{
		offsetAndSize, err := reader.Get(cid2_)
		require.NoError(t, err)
		require.NotNil(t, offsetAndSize)

		require.Equal(t, uint64(123456), offsetAndSize.Offset)
		require.Equal(t, uint64(456789), offsetAndSize.Size)
	}
	{
		offsetAndSize, err := reader.Get(cid3_)
		require.NoError(t, err)
		require.NotNil(t, offsetAndSize)

		require.Equal(t, uint64(123456789), offsetAndSize.Offset)
		require.Equal(t, uint64(4567), offsetAndSize.Size)
	}
	// check metadata
	{
		metadata := reader.Meta()
		require.NotNil(t, metadata)

		require.Equal(t, epoch, metadata.Epoch)
		require.Equal(t, rootCid, metadata.RootCid)
		require.Equal(t, indexes.NetworkMainnet, metadata.Network)
		require.Equal(t, indexes.Kind_CidToOffsetAndSize, metadata.IndexKind)
	}
}
