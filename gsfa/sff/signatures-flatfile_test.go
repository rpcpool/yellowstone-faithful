package sff

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignaturesFlatFile(t *testing.T) {
	tmpFilePath := t.TempDir() + "/signatures-flatfile_test"

	sfl, err := NewSignaturesFlatFile(tmpFilePath)
	require.NoError(t, err)
	require.NotNil(t, sfl)
	require.Equal(t, uint64(0), sfl.NumSignatures())

	// Add a signature
	{
		sig := newRandomSignature()
		index, err := sfl.Put(sig)
		require.NoError(t, err)
		require.Equal(t, uint64(0), index)
		require.Equal(t, uint64(1), sfl.NumSignatures())
		require.NoError(t, sfl.Flush())
		got, err := sfl.Get(index)
		require.NoError(t, err)
		require.Equal(t, sig, got)
	}
	{
		sig := newRandomSignature()
		index, err := sfl.Put(sig)
		require.NoError(t, err)
		require.Equal(t, uint64(1), index)
		require.Equal(t, uint64(2), sfl.NumSignatures())
		require.NoError(t, sfl.Flush())
		got, err := sfl.Get(index)
		require.NoError(t, err)
		require.Equal(t, sig, got)
	}
	{
		// non-existent index
		_, err := sfl.Get(2)
		require.Error(t, err)
	}
}

func newRandomSignature() [SignatureSize]byte {
	var sig [SignatureSize]byte
	rand.Read(sig[:])
	return sig
}
