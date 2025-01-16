package main

import (
	"github.com/gagliardetto/solana-go"
)

// IsSimpleVoteTransaction checks if a transaction is a simple vote transaction.
// A simple vote transaction meets these conditions:
// 1. has 1 or 2 signatures
// 2. is legacy message (this is implicit in solana-go as it mainly handles legacy messages)
// 3. has only one instruction
// 4. which must be Vote instruction
func IsSimpleVoteTransaction(tx *solana.Transaction) bool {
    // Check signature count (condition 1)
    if len(tx.Signatures) == 0 || len(tx.Signatures) > 2 {
        return false
    }

    // Check instruction count (condition 3)
    instructions := tx.Message.Instructions
    if len(instructions) != 1 {
        return false
    }

    // Get the program ID for the instruction
    programID := tx.Message.AccountKeys[instructions[0].ProgramIDIndex]

    // Check if it's a Vote instruction (condition 4)
    // Note: This is the Vote Program ID on Solana mainnet
    voteProgram := solana.VoteProgramID // This is a built-in constant in solana-go

    return programID.Equals(voteProgram)
}
