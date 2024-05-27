package main

import (
	metalatest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-latest"
	metaoldest "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta/v-oldest"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
)

func getErr(meta any) any {
	switch metaValue := meta.(type) {
	case *confirmed_block.TransactionStatusMeta:
		out, _ := parseTransactionError(metaValue.Err)
		return out
	case *metalatest.TransactionStatusMeta:
		switch status := metaValue.Status.(type) {
		case *metalatest.Result__Ok:
			return nil // no error
		case *metalatest.Result__Err:
			switch err_ := status.Value.(type) {
			case *metalatest.TransactionError__AccountInUse:
				return map[string]any{
					"AccountInUse": []any{0},
				}
			case *metalatest.TransactionError__AccountLoadedTwice:
				return map[string]any{
					"AccountLoadedTwice": []any{1},
				}
			case *metalatest.TransactionError__AccountNotFound:
				return map[string]any{
					"AccountNotFound": []any{2},
				}
			case *metalatest.TransactionError__ProgramAccountNotFound:
				return map[string]any{
					"ProgramAccountNotFound": []any{3},
				}
			case *metalatest.TransactionError__InsufficientFundsForFee:
				return map[string]any{
					"InsufficientFundsForFee": []any{4},
				}
			case *metalatest.TransactionError__InvalidAccountForFee:
				return map[string]any{
					"InvalidAccountForFee": []any{5},
				}
			case *metalatest.TransactionError__DuplicateSignature:
				return map[string]any{
					"DuplicateSignature": []any{6},
				}
			case *metalatest.TransactionError__BlockhashNotFound:
				return map[string]any{
					"BlockhashNotFound": []any{7},
				}
			case *metalatest.TransactionError__CallChainTooDeep:
				return map[string]any{
					"CallChainTooDeep": []any{8},
				}
			case *metalatest.TransactionError__MissingSignatureForFee:
				return map[string]any{
					"MissingSignatureForFee": []any{9},
				}
			case *metalatest.TransactionError__InvalidAccountIndex:
				return map[string]any{
					"InvalidAccountIndex": []any{10},
				}
			case *metalatest.TransactionError__SignatureFailure:
				return map[string]any{
					"SignatureFailure": []any{11},
				}
			case *metalatest.TransactionError__InvalidProgramForExecution:
				return map[string]any{
					"InvalidProgramForExecution": []any{12},
				}
			case *metalatest.TransactionError__SanitizeFailure:
				return map[string]any{
					"SanitizeFailure": []any{13},
				}
			case *metalatest.TransactionError__ClusterMaintenance:
				return map[string]any{
					"ClusterMaintenance": []any{14},
				}

			case *metalatest.TransactionError__InstructionError:
				transactionErrorType := err_.Field0
				instructionErrorType := err_.Field1

				gotInstructionErrorName := func() any {
					switch realInstructionError := instructionErrorType.(type) {
					case *metalatest.InstructionError__GenericError:
						return "GenericError"
					case *metalatest.InstructionError__InvalidArgument:
						return "InvalidArgument"
					case *metalatest.InstructionError__InvalidInstructionData:
						return "InvalidInstructionData"
					case *metalatest.InstructionError__InvalidAccountData:
						return "InvalidAccountData"
					case *metalatest.InstructionError__AccountDataTooSmall:
						return "AccountDataTooSmall"
					case *metalatest.InstructionError__InsufficientFunds:
						return "InsufficientFunds"
					case *metalatest.InstructionError__IncorrectProgramId:
						return "IncorrectProgramId"
					case *metalatest.InstructionError__MissingRequiredSignature:
						return "MissingRequiredSignature"
					case *metalatest.InstructionError__AccountAlreadyInitialized:
						return "AccountAlreadyInitialized"
					case *metalatest.InstructionError__UninitializedAccount:
						return "UninitializedAccount"
					case *metalatest.InstructionError__UnbalancedInstruction:
						return "UnbalancedInstruction"
					case *metalatest.InstructionError__ModifiedProgramId:
						return "ModifiedProgramId"
					case *metalatest.InstructionError__ExternalAccountLamportSpend:
						return "ExternalAccountLamportSpend"
					case *metalatest.InstructionError__ExternalAccountDataModified:
						return "ExternalAccountDataModified"
					case *metalatest.InstructionError__ReadonlyLamportChange:
						return "ReadonlyLamportChange"
					case *metalatest.InstructionError__ReadonlyDataModified:
						return "ReadonlyDataModified"
					case *metalatest.InstructionError__DuplicateAccountIndex:
						return "DuplicateAccountIndex"
					case *metalatest.InstructionError__ExecutableModified:
						return "ExecutableModified"
					case *metalatest.InstructionError__RentEpochModified:
						return "RentEpochModified"
					case *metalatest.InstructionError__NotEnoughAccountKeys:
						return "NotEnoughAccountKeys"
					case *metalatest.InstructionError__AccountDataSizeChanged:
						return "AccountDataSizeChanged"
					case *metalatest.InstructionError__AccountNotExecutable:
						return "AccountNotExecutable"
					case *metalatest.InstructionError__AccountBorrowFailed:
						return "AccountBorrowFailed"
					case *metalatest.InstructionError__AccountBorrowOutstanding:
						return "AccountBorrowOutstanding"
					case *metalatest.InstructionError__DuplicateAccountOutOfSync:
						return "DuplicateAccountOutOfSync"
					case *metalatest.InstructionError__Custom:
						return map[string]any{
							"Custom": realInstructionError,
						}
					case *metalatest.InstructionError__InvalidError:
						return "InvalidError"
					case *metalatest.InstructionError__ExecutableDataModified:
						return "ExecutableDataModified"
					case *metalatest.InstructionError__ExecutableLamportChange:
						return "ExecutableLamportChange"
					case *metalatest.InstructionError__ExecutableAccountNotRentExempt:
						return "ExecutableAccountNotRentExempt"
					case *metalatest.InstructionError__UnsupportedProgramId:
						return "UnsupportedProgramId"
					case *metalatest.InstructionError__CallDepth:
						return "CallDepth"
					case *metalatest.InstructionError__MissingAccount:
						return "MissingAccount"
					case *metalatest.InstructionError__ReentrancyNotAllowed:
						return "ReentrancyNotAllowed"
					case *metalatest.InstructionError__MaxSeedLengthExceeded:
						return "MaxSeedLengthExceeded"
					case *metalatest.InstructionError__InvalidSeeds:
						return "InvalidSeeds"
					case *metalatest.InstructionError__InvalidRealloc:
						return "InvalidRealloc"
					case *metalatest.InstructionError__ComputationalBudgetExceeded:
						return "ComputationalBudgetExceeded"
					default:
						return map[string]any{
							"unknown": []any{}, // unknown; could not parse
						}
					}
				}()
				return map[string]any{
					"InstructionError": []any{
						transactionErrorType,
						gotInstructionErrorName,
					},
				}
			}
		}

	case *metaoldest.TransactionStatusMeta:
		switch status := metaValue.Status.(type) {
		case *metaoldest.Result__Ok:
			return nil // no error
		case *metaoldest.Result__Err:
			switch err_ := status.Value.(type) {
			case *metaoldest.TransactionError__AccountInUse:
				return map[string]any{
					"AccountInUse": []any{0},
				}
			case *metaoldest.TransactionError__AccountLoadedTwice:
				return map[string]any{
					"AccountLoadedTwice": []any{1},
				}
			case *metaoldest.TransactionError__AccountNotFound:
				return map[string]any{
					"AccountNotFound": []any{2},
				}
			case *metaoldest.TransactionError__ProgramAccountNotFound:
				return map[string]any{
					"ProgramAccountNotFound": []any{3},
				}
			case *metaoldest.TransactionError__InsufficientFundsForFee:
				return map[string]any{
					"InsufficientFundsForFee": []any{4},
				}
			case *metaoldest.TransactionError__InvalidAccountForFee:
				return map[string]any{
					"InvalidAccountForFee": []any{5},
				}
			case *metaoldest.TransactionError__DuplicateSignature:
				return map[string]any{
					"DuplicateSignature": []any{6},
				}
			case *metaoldest.TransactionError__BlockhashNotFound:
				return map[string]any{
					"BlockhashNotFound": []any{7},
				}
			case *metaoldest.TransactionError__InstructionError:
				transactionErrorType := err_.Field0
				instructionErrorType := err_.Field1

				gotInstructionErrorName := func() any {
					switch realInstructionError := instructionErrorType.(type) {
					case *metaoldest.InstructionError__GenericError:
						return "GenericError"
					case *metaoldest.InstructionError__InvalidArgument:
						return "InvalidArgument"
					case *metaoldest.InstructionError__InvalidInstructionData:
						return "InvalidInstructionData"
					case *metaoldest.InstructionError__InvalidAccountData:
						return "InvalidAccountData"
					case *metaoldest.InstructionError__AccountDataTooSmall:
						return "AccountDataTooSmall"
					case *metaoldest.InstructionError__InsufficientFunds:
						return "InsufficientFunds"
					case *metaoldest.InstructionError__IncorrectProgramId:
						return "IncorrectProgramId"
					case *metaoldest.InstructionError__MissingRequiredSignature:
						return "MissingRequiredSignature"
					case *metaoldest.InstructionError__AccountAlreadyInitialized:
						return "AccountAlreadyInitialized"
					case *metaoldest.InstructionError__UninitializedAccount:
						return "UninitializedAccount"
					case *metaoldest.InstructionError__UnbalancedInstruction:
						return "UnbalancedInstruction"
					case *metaoldest.InstructionError__ModifiedProgramId:
						return "ModifiedProgramId"
					case *metaoldest.InstructionError__ExternalAccountLamportSpend:
						return "ExternalAccountLamportSpend"
					case *metaoldest.InstructionError__ExternalAccountDataModified:
						return "ExternalAccountDataModified"
					case *metaoldest.InstructionError__ReadonlyLamportChange:
						return "ReadonlyLamportChange"
					case *metaoldest.InstructionError__ReadonlyDataModified:
						return "ReadonlyDataModified"
					case *metaoldest.InstructionError__DuplicateAccountIndex:
						return "DuplicateAccountIndex"
					case *metaoldest.InstructionError__ExecutableModified:
						return "ExecutableModified"
					case *metaoldest.InstructionError__RentEpochModified:
						return "RentEpochModified"
					case *metaoldest.InstructionError__NotEnoughAccountKeys:
						return "NotEnoughAccountKeys"
					case *metaoldest.InstructionError__AccountDataSizeChanged:
						return "AccountDataSizeChanged"
					case *metaoldest.InstructionError__AccountNotExecutable:
						return "AccountNotExecutable"
					case *metaoldest.InstructionError__AccountBorrowFailed:
						return "AccountBorrowFailed"
					case *metaoldest.InstructionError__AccountBorrowOutstanding:
						return "AccountBorrowOutstanding"
					case *metaoldest.InstructionError__DuplicateAccountOutOfSync:
						return "DuplicateAccountOutOfSync"
					case *metaoldest.InstructionError__CustomError:
						return map[string]any{
							"Custom": realInstructionError,
						}
					case *metaoldest.InstructionError__InvalidError:
						return "InvalidError"
					default:
						return map[string]any{
							"unknown": []any{}, // unknown; could not parse
						}
					}
				}()
				return map[string]any{
					"InstructionError": []any{
						transactionErrorType,
						gotInstructionErrorName,
					},
				}
			case *metaoldest.TransactionError__CallChainTooDeep:
				return map[string]any{
					"CallChainTooDeep": []any{8},
				}
			case *metaoldest.TransactionError__MissingSignatureForFee:
				return map[string]any{
					"MissingSignatureForFee": []any{9},
				}
			case *metaoldest.TransactionError__InvalidAccountIndex:
				return map[string]any{
					"InvalidAccountIndex": []any{10},
				}
			case *metaoldest.TransactionError__SignatureFailure:
				return map[string]any{
					"SignatureFailure": []any{11},
				}
			case *metaoldest.TransactionError__InvalidProgramForExecution:
				return map[string]any{
					"InvalidProgramForExecution": []any{12},
				}
			default:
				return map[string]any{
					"unknown": []any{}, // unknown; could not parse
				}
			}
		}
	default:
		return map[string]any{
			"unknown": []any{}, // unknown; could not parse
		}
	}
	return map[string]any{
		"unknown": []any{}, // unknown; could not parse
	}
}
