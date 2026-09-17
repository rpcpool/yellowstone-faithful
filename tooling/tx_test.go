package tooling

import (
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/stretchr/testify/require"
)

// signedTx builds and signs a minimal transfer transaction in the requested
// message version.
func signedTx(t *testing.T, version solana.MessageVersion) *solana.Transaction {
	t.Helper()
	payer := solana.NewWallet()
	recipient := solana.NewWallet()
	ix := system.NewTransferInstruction(1000, payer.PublicKey(), recipient.PublicKey()).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		solana.Hash{1, 2, 3, 4}, // arbitrary recent blockhash; not validated here
		solana.TransactionPayer(payer.PublicKey()),
		solana.TransactionMessageVersion(version),
	)
	require.NoError(t, err)

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer.PrivateKey
		}
		return nil
	})
	require.NoError(t, err)
	return tx
}

// oldFrontRead reproduces the pre-fix logic: read a compact-u16 count off the
// front, then take the next 64 bytes as the "signature". For a v1 transaction
// this reads message bytes, not the real signature.
func oldFrontRead(t *testing.T, buf []byte) solana.Signature {
	t.Helper()
	dec := bin.NewCompactU16Decoder(buf)
	_, err := dec.ReadCompactU16()
	require.NoError(t, err)
	var s solana.Signature
	_, err = dec.Read(s[:])
	require.NoError(t, err)
	return s
}

func TestReadFirstSignature_Legacy(t *testing.T) {
	tx := signedTx(t, solana.MessageVersionLegacy)
	wire, err := tx.MarshalBinary()
	require.NoError(t, err)

	// Legacy layout begins with a compact-u16 signature count (< 0x80).
	require.Less(t, wire[0], byte(0x80))

	got, err := ReadFirstSignature(wire)
	require.NoError(t, err)
	require.Equal(t, tx.Signatures[0], got)
}

func TestReadFirstSignature_V0(t *testing.T) {
	tx := signedTx(t, solana.MessageVersionV0)
	wire, err := tx.MarshalBinary()
	require.NoError(t, err)

	// v0 shares the legacy signature layout: a compact-u16 count (< 0x80) at the
	// front, then the signatures. Only the message (inside) carries the version.
	require.Less(t, wire[0], byte(0x80))

	got, err := ReadFirstSignature(wire)
	require.NoError(t, err)
	require.Equal(t, tx.Signatures[0], got)
}

func TestReadFirstSignature_V1(t *testing.T) {
	tx := signedTx(t, solana.MessageVersionV1)
	wire, err := tx.MarshalBinary()
	require.NoError(t, err)

	// v1 (SIMD-0385): discriminator byte, message first, signatures last.
	require.Equal(t, byte(messageVersionV1Prefix), wire[0])

	got, err := ReadFirstSignature(wire)
	require.NoError(t, err)
	require.Equal(t, tx.Signatures[0], got, "must return the real first signature for a v1 transaction")

	// Regression guard: the pre-fix front-reading logic returns message bytes,
	// not the signature. If these were ever equal, the fix would be a no-op.
	require.NotEqual(t, tx.Signatures[0], oldFrontRead(t, wire),
		"the old logic must misread v1 (otherwise this test proves nothing)")
}

func TestReadFirstSignature_Empty(t *testing.T) {
	_, err := ReadFirstSignature(nil)
	require.Error(t, err)
}
