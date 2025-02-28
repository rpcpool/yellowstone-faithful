package main

import (
	"github.com/gagliardetto/solana-go"
)

// Taken from https://github.com/anza-xyz/agave/blob/a03b8704ab964a1402d772ccc1a369816bdb4ee6/sdk/transaction/src/simple_vote_transaction_checker.rs#L6-L28
func IsVote(tx *solana.Transaction) bool {
	isLegacy := !tx.Message.IsVersioned()
	programs := getPrograms(tx)

	return is_simple_vote_transaction_impl(
		tx.Signatures,
		isLegacy,
		programs,
	)
}

func getPrograms(tx *solana.Transaction) []solana.PublicKey {
	programs := make([]solana.PublicKey, 0)
	for _, inst := range tx.Message.Instructions {
		progKey, err := tx.ResolveProgramIDIndex(inst.ProgramIDIndex)
		if err == nil {
			programs = append(programs, progKey)
		}
	}
	return programs
}

/// Simple vote transaction meets these conditions:
/// 1. has 1 or 2 signatures;
/// 2. is legacy message;
/// 3. has only one instruction;
/// 4. which must be Vote instruction;
// #[inline]
// pub fn is_simple_vote_transaction_impl<'a>(
//     signatures: &[Signature],
//     is_legacy_message: bool,
//     mut instruction_programs: impl Iterator<Item = &'a Pubkey>,
// ) -> bool {
//     signatures.len() < 3
//         && is_legacy_message
//         && instruction_programs
//             .next()
//             .xor(instruction_programs.next())
//             .map(|program_id| program_id == &solana_sdk_ids::vote::ID)
//             .unwrap_or(false)
// }

func is_simple_vote_transaction_impl(
	signatures []solana.Signature,
	is_legacy_message bool,
	instruction_programs []solana.PublicKey,
) bool {
	return len(signatures) < 3 &&
		is_legacy_message &&
		(len(instruction_programs) == 1 || len(instruction_programs) == 2) &&
		(instruction_programs[0] == solana.VoteProgramID || instruction_programs[1] == solana.VoteProgramID)
}

func IsSimpleVoteTransaction(tx *solana.Transaction) bool {
	return IsVote(tx)
}
