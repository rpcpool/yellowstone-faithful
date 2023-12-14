package indexes_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/stretchr/testify/require"
)

func TestSigToCid(t *testing.T) {
	epoch := uint64(123)
	cstr := "bafyreids2hw6eynl4vag3cdp535sxz6zp6tedhuv6xu3k3rze3fskqy4yy"
	rootCid, err := cid.Parse(cstr)
	require.NoError(t, err)
	numItems := uint64(10000)

	dstDir := t.TempDir()
	writer, err := indexes.NewWriter_SigToCid(
		epoch,
		rootCid,
		indexes.NetworkDevnet,
		"",
		numItems,
	)
	require.NoError(t, err)
	require.NotNil(t, writer)

	// write some data to the index
	cid1_, err := cid.Parse("bafyreibwvjchy4qq6tqeqg4olawpzs3cphr7nqp5gz2ch5bnttt2ajg6p4")
	require.NoError(t, err)
	sig1 := newRandomSignature()
	require.NoError(t, writer.Put(sig1, cid1_))

	cid2_, err := cid.Parse("bafyreibqlzq4vrezlbgn7qqgz36tx5itaelyxw4v2xyjho5fqqlrslf2vq")
	require.NoError(t, err)
	sig2 := newRandomSignature()
	require.NoError(t, writer.Put(sig2, cid2_))

	cid3_, err := cid.Parse("bafyreiciqiiofeu74nt4drrw6pysqaethngzjtlbsyskvjmntqzx4fzv7q")
	require.NoError(t, err)
	sig3 := newRandomSignature()
	require.NoError(t, writer.Put(sig3, cid3_))

	{
		// add other 997 items
		for i := uint64(0); i < numItems-3; i++ {
			cid_ := cid.NewCidV1(cid.Raw, []byte(fmt.Sprintf("cid-%d", i)))
			sig_ := newRandomSignature()
			require.NoError(t, writer.Put(sig_, cid_))
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
	reader, err := indexes.Open_SigToCid(finalFilepath)
	require.NoError(t, err)
	require.NotNil(t, reader)

	// read the data back
	{
		cid_, err := reader.Get(sig1)
		require.NoError(t, err)
		require.Equal(t, cid1_, cid_)

		cid_, err = reader.Get(sig2)
		require.NoError(t, err)
		require.Equal(t, cid2_, cid_)

		cid_, err = reader.Get(sig3)
		require.NoError(t, err)
		require.Equal(t, cid3_, cid_)
	}

	// check metadata
	{
		metadata := reader.Meta()
		require.NotNil(t, metadata)

		require.Equal(t, epoch, metadata.Epoch)
		require.Equal(t, rootCid, metadata.RootCid)
		require.Equal(t, indexes.NetworkDevnet, metadata.Network)
		require.Equal(t, indexes.Kind_SigToCid, metadata.IndexKind)
	}
}

func newRandomSignature() solana.Signature {
	var sig solana.Signature
	rand.Read(sig[:])
	return sig
}
