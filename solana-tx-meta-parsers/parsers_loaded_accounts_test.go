package solanatxmetaparsers

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestGetLoadedAccountsNilContainer(t *testing.T) {
	var container *TransactionStatusMetaContainer

	rawWritable, rawReadonly := container.GetLoadedAccountsRaw()
	if len(rawWritable) != 0 || len(rawReadonly) != 0 {
		t.Fatalf("expected no raw loaded accounts, got %d writable and %d readonly", len(rawWritable), len(rawReadonly))
	}

	writable, readonly := container.GetLoadedAccounts()
	if len(writable) != 0 || len(readonly) != 0 {
		t.Fatalf("expected no loaded accounts, got %d writable and %d readonly", len(writable), len(readonly))
	}
}

func TestGetLoadedAccountsFromProtobuf(t *testing.T) {
	writableKey := solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111")
	readonlyKey := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")

	container := MakeMockTxSuccessTxMetaContainer(
		[]solana.PublicKey{writableKey},
		[]solana.PublicKey{readonlyKey},
	)

	writable, readonly := container.GetLoadedAccounts()
	if len(writable) != 1 || writable[0] != writableKey {
		t.Fatalf("unexpected writable keys: %v", writable)
	}
	if len(readonly) != 1 || readonly[0] != readonlyKey {
		t.Fatalf("unexpected readonly keys: %v", readonly)
	}
}
