package testutil

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
	Value []byte // 8 + 8 bytes
}

// RawValue returns the Value of the Entry.
func (e *Entry) RawValue() []byte {
	return e.Value
}

func GenerateEntries(n int) []Entry {
	generatedEntries := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		key := solana.NewWallet().PublicKey()
		value := RandomBytes(16)
		generatedEntries = append(generatedEntries, Entry{
			Key:   key,
			Value: value,
		})
	}
	return generatedEntries
}
