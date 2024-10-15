package main

import (
	"encoding/json"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

type GetSignaturesForAddressParams struct {
	Address solana.PublicKey
	Limit   int
	Before  *solana.Signature
	Until   *solana.Signature
	// TODO: add more params
}

func parseGetSignaturesForAddressParams(raw *json.RawMessage) (*GetSignaturesForAddressParams, error) {
	var params []any
	if err := fasterJson.Unmarshal(*raw, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}
	if len(params) < 1 {
		return nil, fmt.Errorf("expected at least 1 param")
	}
	sigRaw, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a string")
	}

	out := &GetSignaturesForAddressParams{}
	pk, err := solana.PublicKeyFromBase58(sigRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pubkey from base58: %w", err)
	}
	out.Address = pk

	if len(params) > 1 {
		// the second param should be a map[string]interface{}
		// with the optional params
		if m, ok := params[1].(map[string]interface{}); ok {
			if limit, ok := m["limit"]; ok {
				if limit, ok := limit.(float64); ok {
					out.Limit = int(limit)
				}
			}
			if before, ok := m["before"]; ok {
				if before, ok := before.(string); ok {
					sig, err := solana.SignatureFromBase58(before)
					if err != nil {
						return nil, fmt.Errorf("failed to parse signature from base58: %w", err)
					}
					out.Before = &sig
				}
			}
			if after, ok := m["until"]; ok {
				if after, ok := after.(string); ok {
					sig, err := solana.SignatureFromBase58(after)
					if err != nil {
						return nil, fmt.Errorf("failed to parse signature from base58: %w", err)
					}
					out.Until = &sig
				}
			}
		}
	}
	if out.Limit <= 0 || out.Limit > 1000 {
		// default limit
		out.Limit = 1000
	}
	return out, nil
}

func getMemoInstructionDataFromTransaction(tx *solana.Transaction) []byte {
	for _, instruction := range tx.Message.Instructions {
		prog, err := tx.ResolveProgramIDIndex(instruction.ProgramIDIndex)
		if err != nil {
			continue
		}
		if prog.IsAnyOf(memoProgramIDV1, memoProgramIDV2) {
			return instruction.Data
		}
	}
	return nil
}

var (
	memoProgramIDV1 = solana.MPK("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")
	memoProgramIDV2 = solana.MPK("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
)
