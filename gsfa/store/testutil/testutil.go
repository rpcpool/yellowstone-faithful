package testutil

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"crypto/rand"

	"github.com/gagliardetto/solana-go"
)

// RandomBytes returns a byte array of the given size with random values.
func RandomBytes(n int64) []byte {
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return data
}

type Entry struct {
	Key   solana.PublicKey
	Value []byte // 8 bytes
}

// RawValue returns the Value of the Entry.
func (e *Entry) RawValue() []byte {
	return e.Value
}

func GenerateEntries(n int) []Entry {
	generatedEntries := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		key := solana.NewWallet().PublicKey()
		value := RandomBytes(8) // The value is 8 bytes long (uint64 little-endian).
		generatedEntries = append(generatedEntries, Entry{
			Key:   key,
			Value: value,
		})
	}
	return generatedEntries
}
