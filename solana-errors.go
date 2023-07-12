package main

type TransactionErrorType int32

const (
	TransactionErrorType_ACCOUNT_IN_USE                           TransactionErrorType = 0
	TransactionErrorType_ACCOUNT_LOADED_TWICE                     TransactionErrorType = 1
	TransactionErrorType_ACCOUNT_NOT_FOUND                        TransactionErrorType = 2
	TransactionErrorType_PROGRAM_ACCOUNT_NOT_FOUND                TransactionErrorType = 3
	TransactionErrorType_INSUFFICIENT_FUNDS_FOR_FEE               TransactionErrorType = 4
	TransactionErrorType_INVALID_ACCOUNT_FOR_FEE                  TransactionErrorType = 5
	TransactionErrorType_ALREADY_PROCESSED                        TransactionErrorType = 6
	TransactionErrorType_BLOCKHASH_NOT_FOUND                      TransactionErrorType = 7
	TransactionErrorType_INSTRUCTION_ERROR                        TransactionErrorType = 8
	TransactionErrorType_CALL_CHAIN_TOO_DEEP                      TransactionErrorType = 9
	TransactionErrorType_MISSING_SIGNATURE_FOR_FEE                TransactionErrorType = 10
	TransactionErrorType_INVALID_ACCOUNT_INDEX                    TransactionErrorType = 11
	TransactionErrorType_SIGNATURE_FAILURE                        TransactionErrorType = 12
	TransactionErrorType_INVALID_PROGRAM_FOR_EXECUTION            TransactionErrorType = 13
	TransactionErrorType_SANITIZE_FAILURE                         TransactionErrorType = 14
	TransactionErrorType_CLUSTER_MAINTENANCE                      TransactionErrorType = 15
	TransactionErrorType_ACCOUNT_BORROW_OUTSTANDING_TX            TransactionErrorType = 16
	TransactionErrorType_WOULD_EXCEED_MAX_BLOCK_COST_LIMIT        TransactionErrorType = 17
	TransactionErrorType_UNSUPPORTED_VERSION                      TransactionErrorType = 18
	TransactionErrorType_INVALID_WRITABLE_ACCOUNT                 TransactionErrorType = 19
	TransactionErrorType_WOULD_EXCEED_MAX_ACCOUNT_COST_LIMIT      TransactionErrorType = 20
	TransactionErrorType_WOULD_EXCEED_ACCOUNT_DATA_BLOCK_LIMIT    TransactionErrorType = 21
	TransactionErrorType_TOO_MANY_ACCOUNT_LOCKS                   TransactionErrorType = 22
	TransactionErrorType_ADDRESS_LOOKUP_TABLE_NOT_FOUND           TransactionErrorType = 23
	TransactionErrorType_INVALID_ADDRESS_LOOKUP_TABLE_OWNER       TransactionErrorType = 24
	TransactionErrorType_INVALID_ADDRESS_LOOKUP_TABLE_DATA        TransactionErrorType = 25
	TransactionErrorType_INVALID_ADDRESS_LOOKUP_TABLE_INDEX       TransactionErrorType = 26
	TransactionErrorType_INVALID_RENT_PAYING_ACCOUNT              TransactionErrorType = 27
	TransactionErrorType_WOULD_EXCEED_MAX_VOTE_COST_LIMIT         TransactionErrorType = 28
	TransactionErrorType_WOULD_EXCEED_ACCOUNT_DATA_TOTAL_LIMIT    TransactionErrorType = 29
	TransactionErrorType_DUPLICATE_INSTRUCTION                    TransactionErrorType = 30
	TransactionErrorType_INSUFFICIENT_FUNDS_FOR_RENT              TransactionErrorType = 31
	TransactionErrorType_MAX_LOADED_ACCOUNTS_DATA_SIZE_EXCEEDED   TransactionErrorType = 32
	TransactionErrorType_INVALID_LOADED_ACCOUNTS_DATA_SIZE_LIMIT  TransactionErrorType = 33
	TransactionErrorType_RESANITIZATION_NEEDED                    TransactionErrorType = 34
	TransactionErrorType_PROGRAM_EXECUTION_TEMPORARILY_RESTRICTED TransactionErrorType = 35
)

// Enum value maps for TransactionErrorType.
var (
	TransactionErrorType_name = map[int32]string{
		0:  "ACCOUNT_IN_USE",
		1:  "ACCOUNT_LOADED_TWICE",
		2:  "ACCOUNT_NOT_FOUND",
		3:  "PROGRAM_ACCOUNT_NOT_FOUND",
		4:  "INSUFFICIENT_FUNDS_FOR_FEE",
		5:  "INVALID_ACCOUNT_FOR_FEE",
		6:  "ALREADY_PROCESSED",
		7:  "BLOCKHASH_NOT_FOUND",
		8:  "INSTRUCTION_ERROR",
		9:  "CALL_CHAIN_TOO_DEEP",
		10: "MISSING_SIGNATURE_FOR_FEE",
		11: "INVALID_ACCOUNT_INDEX",
		12: "SIGNATURE_FAILURE",
		13: "INVALID_PROGRAM_FOR_EXECUTION",
		14: "SANITIZE_FAILURE",
		15: "CLUSTER_MAINTENANCE",
		16: "ACCOUNT_BORROW_OUTSTANDING_TX",
		17: "WOULD_EXCEED_MAX_BLOCK_COST_LIMIT",
		18: "UNSUPPORTED_VERSION",
		19: "INVALID_WRITABLE_ACCOUNT",
		20: "WOULD_EXCEED_MAX_ACCOUNT_COST_LIMIT",
		21: "WOULD_EXCEED_ACCOUNT_DATA_BLOCK_LIMIT",
		22: "TOO_MANY_ACCOUNT_LOCKS",
		23: "ADDRESS_LOOKUP_TABLE_NOT_FOUND",
		24: "INVALID_ADDRESS_LOOKUP_TABLE_OWNER",
		25: "INVALID_ADDRESS_LOOKUP_TABLE_DATA",
		26: "INVALID_ADDRESS_LOOKUP_TABLE_INDEX",
		27: "INVALID_RENT_PAYING_ACCOUNT",
		28: "WOULD_EXCEED_MAX_VOTE_COST_LIMIT",
		29: "WOULD_EXCEED_ACCOUNT_DATA_TOTAL_LIMIT",
		30: "DUPLICATE_INSTRUCTION",
		31: "INSUFFICIENT_FUNDS_FOR_RENT",
		32: "MAX_LOADED_ACCOUNTS_DATA_SIZE_EXCEEDED",
		33: "INVALID_LOADED_ACCOUNTS_DATA_SIZE_LIMIT",
		34: "RESANITIZATION_NEEDED",
		35: "PROGRAM_EXECUTION_TEMPORARILY_RESTRICTED",
	}
)

// Enum value maps for InstructionErrorType.
var (
	InstructionErrorType_name = map[int32]string{
		0:  "GENERIC_ERROR",
		1:  "INVALID_ARGUMENT",
		2:  "INVALID_INSTRUCTION_DATA",
		3:  "INVALID_ACCOUNT_DATA",
		4:  "ACCOUNT_DATA_TOO_SMALL",
		5:  "INSUFFICIENT_FUNDS",
		6:  "INCORRECT_PROGRAM_ID",
		7:  "MISSING_REQUIRED_SIGNATURE",
		8:  "ACCOUNT_ALREADY_INITIALIZED",
		9:  "UNINITIALIZED_ACCOUNT",
		10: "UNBALANCED_INSTRUCTION",
		11: "MODIFIED_PROGRAM_ID",
		12: "EXTERNAL_ACCOUNT_LAMPORT_SPEND",
		13: "EXTERNAL_ACCOUNT_DATA_MODIFIED",
		14: "READONLY_LAMPORT_CHANGE",
		15: "READONLY_DATA_MODIFIED",
		16: "DUPLICATE_ACCOUNT_INDEX",
		17: "EXECUTABLE_MODIFIED",
		18: "RENT_EPOCH_MODIFIED",
		19: "NOT_ENOUGH_ACCOUNT_KEYS",
		20: "ACCOUNT_DATA_SIZE_CHANGED",
		21: "ACCOUNT_NOT_EXECUTABLE",
		22: "ACCOUNT_BORROW_FAILED",
		23: "ACCOUNT_BORROW_OUTSTANDING",
		24: "DUPLICATE_ACCOUNT_OUT_OF_SYNC",
		25: "CUSTOM",
		26: "INVALID_ERROR",
		27: "EXECUTABLE_DATA_MODIFIED",
		28: "EXECUTABLE_LAMPORT_CHANGE",
		29: "EXECUTABLE_ACCOUNT_NOT_RENT_EXEMPT",
		30: "UNSUPPORTED_PROGRAM_ID",
		31: "CALL_DEPTH",
		32: "MISSING_ACCOUNT",
		33: "REENTRANCY_NOT_ALLOWED",
		34: "MAX_SEED_LENGTH_EXCEEDED",
		35: "INVALID_SEEDS",
		36: "INVALID_REALLOC",
		37: "COMPUTATIONAL_BUDGET_EXCEEDED",
		38: "PRIVILEGE_ESCALATION",
		39: "PROGRAM_ENVIRONMENT_SETUP_FAILURE",
		40: "PROGRAM_FAILED_TO_COMPLETE",
		41: "PROGRAM_FAILED_TO_COMPILE",
		42: "IMMUTABLE",
		43: "INCORRECT_AUTHORITY",
		44: "BORSH_IO_ERROR",
		45: "ACCOUNT_NOT_RENT_EXEMPT",
		46: "INVALID_ACCOUNT_OWNER",
		47: "ARITHMETIC_OVERFLOW",
		48: "UNSUPPORTED_SYSVAR",
		49: "ILLEGAL_OWNER",
		50: "MAX_ACCOUNTS_DATA_ALLOCATIONS_EXCEEDED",
		51: "MAX_ACCOUNTS_EXCEEDED",
		52: "MAX_INSTRUCTION_TRACE_LENGTH_EXCEEDED",
		53: "BUILTIN_PROGRAMS_MUST_CONSUME_COMPUTE_UNITS",
	}
)

type InstructionErrorType int32

const (
	InstructionErrorType_GENERIC_ERROR                               InstructionErrorType = 0
	InstructionErrorType_INVALID_ARGUMENT                            InstructionErrorType = 1
	InstructionErrorType_INVALID_INSTRUCTION_DATA                    InstructionErrorType = 2
	InstructionErrorType_INVALID_ACCOUNT_DATA                        InstructionErrorType = 3
	InstructionErrorType_ACCOUNT_DATA_TOO_SMALL                      InstructionErrorType = 4
	InstructionErrorType_INSUFFICIENT_FUNDS                          InstructionErrorType = 5
	InstructionErrorType_INCORRECT_PROGRAM_ID                        InstructionErrorType = 6
	InstructionErrorType_MISSING_REQUIRED_SIGNATURE                  InstructionErrorType = 7
	InstructionErrorType_ACCOUNT_ALREADY_INITIALIZED                 InstructionErrorType = 8
	InstructionErrorType_UNINITIALIZED_ACCOUNT                       InstructionErrorType = 9
	InstructionErrorType_UNBALANCED_INSTRUCTION                      InstructionErrorType = 10
	InstructionErrorType_MODIFIED_PROGRAM_ID                         InstructionErrorType = 11
	InstructionErrorType_EXTERNAL_ACCOUNT_LAMPORT_SPEND              InstructionErrorType = 12
	InstructionErrorType_EXTERNAL_ACCOUNT_DATA_MODIFIED              InstructionErrorType = 13
	InstructionErrorType_READONLY_LAMPORT_CHANGE                     InstructionErrorType = 14
	InstructionErrorType_READONLY_DATA_MODIFIED                      InstructionErrorType = 15
	InstructionErrorType_DUPLICATE_ACCOUNT_INDEX                     InstructionErrorType = 16
	InstructionErrorType_EXECUTABLE_MODIFIED                         InstructionErrorType = 17
	InstructionErrorType_RENT_EPOCH_MODIFIED                         InstructionErrorType = 18
	InstructionErrorType_NOT_ENOUGH_ACCOUNT_KEYS                     InstructionErrorType = 19
	InstructionErrorType_ACCOUNT_DATA_SIZE_CHANGED                   InstructionErrorType = 20
	InstructionErrorType_ACCOUNT_NOT_EXECUTABLE                      InstructionErrorType = 21
	InstructionErrorType_ACCOUNT_BORROW_FAILED                       InstructionErrorType = 22
	InstructionErrorType_ACCOUNT_BORROW_OUTSTANDING                  InstructionErrorType = 23
	InstructionErrorType_DUPLICATE_ACCOUNT_OUT_OF_SYNC               InstructionErrorType = 24
	InstructionErrorType_CUSTOM                                      InstructionErrorType = 25
	InstructionErrorType_INVALID_ERROR                               InstructionErrorType = 26
	InstructionErrorType_EXECUTABLE_DATA_MODIFIED                    InstructionErrorType = 27
	InstructionErrorType_EXECUTABLE_LAMPORT_CHANGE                   InstructionErrorType = 28
	InstructionErrorType_EXECUTABLE_ACCOUNT_NOT_RENT_EXEMPT          InstructionErrorType = 29
	InstructionErrorType_UNSUPPORTED_PROGRAM_ID                      InstructionErrorType = 30
	InstructionErrorType_CALL_DEPTH                                  InstructionErrorType = 31
	InstructionErrorType_MISSING_ACCOUNT                             InstructionErrorType = 32
	InstructionErrorType_REENTRANCY_NOT_ALLOWED                      InstructionErrorType = 33
	InstructionErrorType_MAX_SEED_LENGTH_EXCEEDED                    InstructionErrorType = 34
	InstructionErrorType_INVALID_SEEDS                               InstructionErrorType = 35
	InstructionErrorType_INVALID_REALLOC                             InstructionErrorType = 36
	InstructionErrorType_COMPUTATIONAL_BUDGET_EXCEEDED               InstructionErrorType = 37
	InstructionErrorType_PRIVILEGE_ESCALATION                        InstructionErrorType = 38
	InstructionErrorType_PROGRAM_ENVIRONMENT_SETUP_FAILURE           InstructionErrorType = 39
	InstructionErrorType_PROGRAM_FAILED_TO_COMPLETE                  InstructionErrorType = 40
	InstructionErrorType_PROGRAM_FAILED_TO_COMPILE                   InstructionErrorType = 41
	InstructionErrorType_IMMUTABLE                                   InstructionErrorType = 42
	InstructionErrorType_INCORRECT_AUTHORITY                         InstructionErrorType = 43
	InstructionErrorType_BORSH_IO_ERROR                              InstructionErrorType = 44
	InstructionErrorType_ACCOUNT_NOT_RENT_EXEMPT                     InstructionErrorType = 45
	InstructionErrorType_INVALID_ACCOUNT_OWNER                       InstructionErrorType = 46
	InstructionErrorType_ARITHMETIC_OVERFLOW                         InstructionErrorType = 47
	InstructionErrorType_UNSUPPORTED_SYSVAR                          InstructionErrorType = 48
	InstructionErrorType_ILLEGAL_OWNER                               InstructionErrorType = 49
	InstructionErrorType_MAX_ACCOUNTS_DATA_ALLOCATIONS_EXCEEDED      InstructionErrorType = 50
	InstructionErrorType_MAX_ACCOUNTS_EXCEEDED                       InstructionErrorType = 51
	InstructionErrorType_MAX_INSTRUCTION_TRACE_LENGTH_EXCEEDED       InstructionErrorType = 52
	InstructionErrorType_BUILTIN_PROGRAMS_MUST_CONSUME_COMPUTE_UNITS InstructionErrorType = 53
)
