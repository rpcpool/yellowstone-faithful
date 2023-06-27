package inmemory_test

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"testing"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/primary/inmemory"
	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	aa := [2][]byte{[]byte("aa"), {0x10}}
	yy := [2][]byte{[]byte("yy"), {0x11}}
	efg := [2][]byte{[]byte("efg"), {0x12}}
	storage := inmemory.New([][2][]byte{aa, yy, efg})

	key, value, err := storage.Get(types.Block{Offset: 0})
	require.NoError(t, err)
	result_aa := [2][]byte{key, value}
	require.Equal(t, result_aa, aa)
	key, value, err = storage.Get(types.Block{Offset: 2})
	require.NoError(t, err)
	result_efg := [2][]byte{key, value}

	require.Equal(t, result_efg, efg)
	key, value, err = storage.Get(types.Block{Offset: 1})
	require.NoError(t, err)
	result_yy := [2][]byte{key, value}

	require.Equal(t, result_yy, yy)
}

func TestPut(t *testing.T) {
	aa := [2][]byte{[]byte("aa"), {0x10}}
	yy := [2][]byte{[]byte("yy"), {0x11}}
	efg := [2][]byte{[]byte("efg"), {0x12}}
	storage := inmemory.New([][2][]byte{})

	put_aa, err := storage.Put(aa[0], aa[1])
	require.NoError(t, err)
	require.Equal(t, put_aa, types.Block{Offset: 0, Size: 1})
	put_yy, err := storage.Put(yy[0], yy[1])
	require.NoError(t, err)
	require.Equal(t, put_yy, types.Block{Offset: 1, Size: 1})
	put_efg, err := storage.Put(efg[0], efg[1])
	require.NoError(t, err)
	require.Equal(t, put_efg, types.Block{Offset: 2, Size: 1})

	key, value, err := storage.Get(types.Block{Offset: 0})
	require.NoError(t, err)
	result_aa := [2][]byte{key, value}

	require.Equal(t, result_aa, aa)
	key, value, err = storage.Get(types.Block{Offset: 2})
	require.NoError(t, err)
	result_efg := [2][]byte{key, value}

	require.Equal(t, result_efg, efg)
	key, value, err = storage.Get(types.Block{Offset: 1})
	require.NoError(t, err)
	result_yy := [2][]byte{key, value}

	require.Equal(t, result_yy, yy)
}
