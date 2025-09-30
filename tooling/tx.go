package tooling

import (
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

func ReadFirstSignature(buf []byte) (solana.Signature, error) {
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
