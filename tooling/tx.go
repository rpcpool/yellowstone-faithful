package tooling

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// messageVersionV1Prefix is the first byte of a v1 (SIMD-0385) transaction.
// It is 0x80|1; a legacy/v0 transaction begins with a compact-u16 signature
// count whose first byte is always < 0x80 (a transaction can have at most a
// handful of signatures), so this byte unambiguously discriminates v1.
const messageVersionV1Prefix = 0x81

// ReadFirstSignature returns the first signature of a serialized transaction.
//
// Legacy and v0 transactions are laid out as
//
//	[compact-u16 numSignatures][signature 0][signature 1]...[message]
//
// so the first signature can be read cheaply straight off the front. v1
// transactions (SIMD-0385) invert this: the wire layout is
//
//	[0x81][message][signature 0][signature 1]...
//
// with the signatures at the *end* and no leading count, so the first signature
// cannot be located without decoding the message. For v1 we therefore fall back
// to the version-aware transaction decoder. (A Solana transaction is capped at
// the 1232-byte packet size, so the whole transaction — including the trailing
// signatures — always fits in the single data frame handed to us here.)
func ReadFirstSignature(buf []byte) (solana.Signature, error) {
	if len(buf) == 0 {
		return solana.Signature{}, fmt.Errorf("empty transaction bytes")
	}

	// v1 (SIMD-0385): message first, signatures last. Decode to locate them.
	if buf[0] == messageVersionV1Prefix {
		tx, err := decodeVersionedTransaction(buf)
		if err != nil {
			return solana.Signature{}, err
		}
		return tx.Signatures[0], nil
	}

	// legacy / v0: signatures are at the front, prefixed by a compact-u16 count.
	decoder := bin.NewCompactU16Decoder(buf)
	numSigs, err := decoder.ReadCompactU16()
	if err != nil {
		return solana.Signature{}, err
	}
	if numSigs == 0 {
		return solana.Signature{}, fmt.Errorf("no signatures")
	}
	// check that there is at least 64 bytes left:
	if decoder.Remaining() < 64 {
		return solana.Signature{}, fmt.Errorf("not enough bytes left to read a signature")
	}

	var sig solana.Signature
	numRead, err := decoder.Read(sig[:])
	if err != nil {
		return sig, err
	}
	if numRead != 64 {
		return sig, fmt.Errorf("unexpected signature length %d", numRead)
	}
	return sig, nil
}

// ReadAllSignatures returns every signature of a serialized transaction. Like
// ReadFirstSignature it is version-aware: legacy/v0 transactions carry their
// signatures at the front behind a compact-u16 count, while v1 (SIMD-0385)
// transactions carry them at the end after the message.
func ReadAllSignatures(buf []byte) ([]solana.Signature, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty transaction bytes")
	}

	// v1 (SIMD-0385): message first, signatures last. Decode to locate them.
	if buf[0] == messageVersionV1Prefix {
		tx, err := decodeVersionedTransaction(buf)
		if err != nil {
			return nil, err
		}
		return tx.Signatures, nil
	}

	// legacy / v0: [compact-u16 numSignatures][signature 0][signature 1]...
	decoder := bin.NewCompactU16Decoder(buf)
	numSigs, err := decoder.ReadCompactU16()
	if err != nil {
		return nil, err
	}
	if numSigs == 0 {
		return nil, fmt.Errorf("no signatures")
	}
	// check that there is at least 64 bytes * numSigs left:
	if decoder.Remaining() < (64 * numSigs) {
		return nil, fmt.Errorf("not enough bytes left to read %d signatures", numSigs)
	}

	sigs := make([]solana.Signature, numSigs)
	for i := 0; i < numSigs; i++ {
		numRead, err := decoder.Read(sigs[i][:])
		if err != nil {
			return nil, err
		}
		if numRead != 64 {
			return nil, fmt.Errorf("unexpected signature length %d", numRead)
		}
	}
	return sigs, nil
}

// decodeVersionedTransaction decodes a full transaction with the version-aware
// decoder (needed for v1, whose signatures live after the message) and ensures
// it has at least one signature.
func decodeVersionedTransaction(buf []byte) (*solana.Transaction, error) {
	tx, err := solana.TransactionFromBytes(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode v1 transaction: %w", err)
	}
	if len(tx.Signatures) == 0 {
		return nil, fmt.Errorf("no signatures")
	}
	return tx, nil
}
