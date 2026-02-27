package main

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/stretchr/testify/assert"
)

func TestFilterStateMachine(t *testing.T) {
	pk1 := solana.MustPublicKeyFromBase58("675kSimBbs975WvSsc9S77XpTf4zgp4vjGNo4S9C69r")
	pk2 := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	pk3 := solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")

	tests := []struct {
		name           string
		filter         *StreamTransactionsFilterExecutable
		tx             *solana.Transaction
		meta           *solanatxmetaparsers.TransactionStatusMetaContainer
		expectExcluded bool
	}{
		{
			name:           "Empty filter should include everything",
			filter:         &StreamTransactionsFilterExecutable{},
			tx:             makeMockTx([]solana.PublicKey{pk1}, false),
			meta:           nil,
			expectExcluded: false,
		},
		{
			name: "Vote strict match - Vote=false, Tx=Vote -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				Vote: ptrToBool(false),
			},
			tx:             makeMockTx([]solana.PublicKey{pk1}, true),
			meta:           nil,
			expectExcluded: true,
		},
		{
			name: "Vote strict match - Vote=true, Tx=NonVote -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				Vote: ptrToBool(true),
			},
			tx:             makeMockTx([]solana.PublicKey{pk1}, false),
			meta:           nil,
			expectExcluded: true,
		},
		{
			name: "Failed strict match - Failed=false, Tx=Failed -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				Failed: ptrToBool(false),
			},
			tx:             makeMockTx([]solana.PublicKey{pk1}, false),
			meta:           makeMockMeta(true, nil, nil),
			expectExcluded: true,
		},
		{
			name: "Account Include - No Match -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				AccountInclude: map[solana.PublicKey]struct{}{
					pk1: {},
				},
			},
			tx:             makeMockTx([]solana.PublicKey{pk2}, false),
			expectExcluded: true,
		},
		{
			name: "Account Exclude - Match -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				AccountExclude: map[solana.PublicKey]struct{}{
					pk2: {},
				},
			},
			tx:             makeMockTx([]solana.PublicKey{pk1, pk2}, false),
			expectExcluded: true,
		},
		{
			name: "Account Required - Missing one -> Exclude",
			filter: &StreamTransactionsFilterExecutable{
				AccountRequired: solana.PublicKeySlice{pk1, pk2},
			},
			tx:             makeMockTx([]solana.PublicKey{pk1, pk3}, false),
			expectExcluded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := tt.filter.CompileExclusion()
			assert.NoError(t, err)

			isExcluded := pipeline.Do(tt.tx, tt.meta)
			assert.Equal(t, tt.expectExcluded, isExcluded)
		})
	}
}

func TestFromStreamTransactionsFilter(t *testing.T) {
	pkStr := "675kSimBbs975WvSsc9S77XpTf4zgp4vjGNo4S9C69r"
	grpcFilter := &old_faithful_grpc.StreamTransactionsFilter{
		Vote:           ptrToBool(false),
		AccountInclude: []string{pkStr},
	}

	exec, err := fromStreamTransactionsFilter(grpcFilter)
	assert.NoError(t, err)
	assert.NotNil(t, exec)
	assert.False(t, *exec.Vote)
	assert.Equal(t, 1, len(exec.AccountInclude))
}

// Helpers

func makeMockTx(keys []solana.PublicKey, isVote bool) *solana.Transaction {
	return makeMockTxWithSig(keys, isVote, solana.Signature{})
}

func makeMockTxWithSig(keys []solana.PublicKey, isVote bool, sig solana.Signature) *solana.Transaction {
	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: keys,
		},
		Signatures: []solana.Signature{sig},
	}

	if isVote {
		tx.Message.AccountKeys = append(tx.Message.AccountKeys, solana.VoteProgramID)
		tx.Message.Instructions = []solana.CompiledInstruction{
			{
				ProgramIDIndex: uint16(len(tx.Message.AccountKeys) - 1),
			},
		}
	} else {
		otherProgram := solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")
		tx.Message.AccountKeys = append(tx.Message.AccountKeys, otherProgram)
		tx.Message.Instructions = []solana.CompiledInstruction{
			{
				ProgramIDIndex: uint16(len(tx.Message.AccountKeys) - 1),
			},
		}
	}
	return tx
}

func makeMockMeta(isErr bool, writable []solana.PublicKey, readonly []solana.PublicKey) *solanatxmetaparsers.TransactionStatusMetaContainer {
	if isErr {
		return solanatxmetaparsers.MakeMockTxErrTxMetaContainer(writable, readonly)
	}
	return solanatxmetaparsers.MakeMockTxSuccessTxMetaContainer(writable, readonly)
}
