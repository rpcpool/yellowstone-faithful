package jsonparsed

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestFromTransaction_AccountKeySource(t *testing.T) {
	static0 := solana.NewWallet().PublicKey()
	static1 := solana.NewWallet().PublicKey()
	loaded0 := solana.NewWallet().PublicKey()

	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: solana.PublicKeySlice{static0, static1, loaded0},
		},
	}

	got, err := FromTransaction(tx, 2)
	if err != nil {
		t.Fatalf("FromTransaction: %v", err)
	}
	if len(got.Message.AccountKeys) != 3 {
		t.Fatalf("expected 3 account keys, got %d", len(got.Message.AccountKeys))
	}
	if got.Message.AccountKeys[0].Source != "transaction" {
		t.Fatalf("static key 0 source = %q, want transaction", got.Message.AccountKeys[0].Source)
	}
	if got.Message.AccountKeys[1].Source != "transaction" {
		t.Fatalf("static key 1 source = %q, want transaction", got.Message.AccountKeys[1].Source)
	}
	if got.Message.AccountKeys[2].Source != "lookupTable" {
		t.Fatalf("loaded key source = %q, want lookupTable", got.Message.AccountKeys[2].Source)
	}
}

func TestFromTransaction_LegacyUsesTransactionSource(t *testing.T) {
	key := solana.NewWallet().PublicKey()
	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: solana.PublicKeySlice{key},
		},
	}

	got, err := FromTransaction(tx, 1)
	if err != nil {
		t.Fatalf("FromTransaction: %v", err)
	}
	if got.Message.AccountKeys[0].Source != "transaction" {
		t.Fatalf("legacy key source = %q, want transaction", got.Message.AccountKeys[0].Source)
	}
}
