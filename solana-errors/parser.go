package solanaerrors

// InstructionError(u8, InstructionError),
// where InstructionError is `Custom(u32),` (i.e. a tuple of 2 elements)

//   "err": {
//     "InstructionError": [
//       2,
//       {
//         "Custom": 6302
//       }
//     ]
//   },

// InstructionError(u8, InstructionError),
// where InstructionError is `InvalidInstructionData,` (i.e. a string)
//   "err": {
//     "InstructionError": [
//       2,
//       "InvalidInstructionData"
//     ]
//   },

// InstructionError(u8, InstructionError),
// where InstructionError is `IncorrectProgramId,` (i.e. a string)
//   "err": {
//     "InstructionError": [
//       0,
//       "IncorrectProgramId"
//     ]
//   },

// NOTE:
// - InstructionError(u8, InstructionError),
// - DuplicateInstruction(u8),
// - InsufficientFundsForRent { account_index: u8 },
// - ProgramExecutionTemporarilyRestricted { account_index: u8 },
// pub enum TransactionError {
const (
	// AccountInUse,
	AccountInUse = "AccountInUse"

	// AccountLoadedTwice,
	AccountLoadedTwice = "AccountLoadedTwice"

	// AccountNotFound,
	AccountNotFound = "AccountNotFound"

	// ProgramAccountNotFound,
	ProgramAccountNotFound = "ProgramAccountNotFound"

	// InsufficientFundsForFee,
	InsufficientFundsForFee = "InsufficientFundsForFee"

	// InvalidAccountForFee,
	InvalidAccountForFee = "InvalidAccountForFee"

	// AlreadyProcessed,
	AlreadyProcessed = "AlreadyProcessed"

	// BlockhashNotFound,
	BlockhashNotFound = "BlockhashNotFound"

	// InstructionError(u8, InstructionError),
	InstructionError = "InstructionError"

	// CallChainTooDeep,
	CallChainTooDeep = "CallChainTooDeep"

	// MissingSignatureForFee,
	MissingSignatureForFee = "MissingSignatureForFee"

	// InvalidAccountIndex,
	InvalidAccountIndex = "InvalidAccountIndex"

	// SignatureFailure,
	SignatureFailure = "SignatureFailure"

	// InvalidProgramForExecution,
	InvalidProgramForExecution = "InvalidProgramForExecution"

	// SanitizeFailure,
	SanitizeFailure = "SanitizeFailure"

	// ClusterMaintenance,
	ClusterMaintenance = "ClusterMaintenance"

	// AccountBorrowOutstanding,
	AccountBorrowOutstanding = "AccountBorrowOutstanding"

	// WouldExceedMaxBlockCostLimit,
	WouldExceedMaxBlockCostLimit = "WouldExceedMaxBlockCostLimit"

	// UnsupportedVersion,
	UnsupportedVersion = "UnsupportedVersion"

	// InvalidWritableAccount,
	InvalidWritableAccount = "InvalidWritableAccount"

	// WouldExceedMaxAccountCostLimit,
	WouldExceedMaxAccountCostLimit = "WouldExceedMaxAccountCostLimit"

	// WouldExceedAccountDataBlockLimit,
	WouldExceedAccountDataBlockLimit = "WouldExceedAccountDataBlockLimit"

	// TooManyAccountLocks,
	TooManyAccountLocks = "TooManyAccountLocks"

	// AddressLookupTableNotFound,
	AddressLookupTableNotFound = "AddressLookupTableNotFound"

	// InvalidAddressLookupTableOwner,
	InvalidAddressLookupTableOwner = "InvalidAddressLookupTableOwner"

	// InvalidAddressLookupTableData,
	InvalidAddressLookupTableData = "InvalidAddressLookupTableData"

	// InvalidAddressLookupTableIndex,
	InvalidAddressLookupTableIndex = "InvalidAddressLookupTableIndex"

	// InvalidRentPayingAccount,
	InvalidRentPayingAccount = "InvalidRentPayingAccount"

	// WouldExceedMaxVoteCostLimit,
	WouldExceedMaxVoteCostLimit = "WouldExceedMaxVoteCostLimit"

	// WouldExceedAccountDataTotalLimit,
	WouldExceedAccountDataTotalLimit = "WouldExceedAccountDataTotalLimit"

	// DuplicateInstruction(u8),
	DuplicateInstruction = "DuplicateInstruction"

	// InsufficientFundsForRent { account_index: u8 },
	InsufficientFundsForRent = "InsufficientFundsForRent"

	// MaxLoadedAccountsDataSizeExceeded,
	MaxLoadedAccountsDataSizeExceeded = "MaxLoadedAccountsDataSizeExceeded"

	// InvalidLoadedAccountsDataSizeLimit,
	InvalidLoadedAccountsDataSizeLimit = "InvalidLoadedAccountsDataSizeLimit"

	// ResanitizationNeeded,
	ResanitizationNeeded = "ResanitizationNeeded"

	// ProgramExecutionTemporarilyRestricted { account_index: u8 },
	ProgramExecutionTemporarilyRestricted = "ProgramExecutionTemporarilyRestricted"

	// UnbalancedTransaction,
	UnbalancedTransaction = "UnbalancedTransaction"

	//	ProgramCacheHitMaxLimit,
	ProgramCacheHitMaxLimit = "ProgramCacheHitMaxLimit"
)

// NOTE:
// - Custom(u32),
// - BorshIoError(String),

// pub enum InstructionError {
const (
	// GenericError,
	GenericError = "GenericError"

	// InvalidArgument,
	InvalidArgument = "InvalidArgument"

	// InvalidInstructionData,
	InvalidInstructionData = "InvalidInstructionData"

	// InvalidAccountData,
	InvalidAccountData = "InvalidAccountData"

	// AccountDataTooSmall,
	AccountDataTooSmall = "AccountDataTooSmall"

	// InsufficientFunds,
	InsufficientFunds = "InsufficientFunds"

	// IncorrectProgramId,
	IncorrectProgramId = "IncorrectProgramId"

	// MissingRequiredSignature,
	MissingRequiredSignature = "MissingRequiredSignature"

	// AccountAlreadyInitialized,
	AccountAlreadyInitialized = "AccountAlreadyInitialized"

	// UninitializedAccount,
	UninitializedAccount = "UninitializedAccount"

	// UnbalancedInstruction,
	UnbalancedInstruction = "UnbalancedInstruction"

	// ModifiedProgramId,
	ModifiedProgramId = "ModifiedProgramId"

	// ExternalAccountLamportSpend,
	ExternalAccountLamportSpend = "ExternalAccountLamportSpend"

	// ExternalAccountDataModified,
	ExternalAccountDataModified = "ExternalAccountDataModified"

	// ReadonlyLamportChange,
	ReadonlyLamportChange = "ReadonlyLamportChange"

	// ReadonlyDataModified,
	ReadonlyDataModified = "ReadonlyDataModified"

	// DuplicateAccountIndex,
	DuplicateAccountIndex = "DuplicateAccountIndex"

	// ExecutableModified,
	ExecutableModified = "ExecutableModified"

	// RentEpochModified,
	RentEpochModified = "RentEpochModified"

	// NotEnoughAccountKeys,
	NotEnoughAccountKeys = "NotEnoughAccountKeys"

	// AccountDataSizeChanged,
	AccountDataSizeChanged = "AccountDataSizeChanged"

	// AccountNotExecutable,
	AccountNotExecutable = "AccountNotExecutable"

	// AccountBorrowFailed,
	AccountBorrowFailed = "AccountBorrowFailed"

	// AccountBorrowOutstanding,
	Instruction_AccountBorrowOutstanding = "AccountBorrowOutstanding"

	// DuplicateAccountOutOfSync,
	DuplicateAccountOutOfSync = "DuplicateAccountOutOfSync"

	// Custom(u32),
	Custom = "Custom"

	// InvalidError,
	InvalidError = "InvalidError"

	// ExecutableDataModified,
	ExecutableDataModified = "ExecutableDataModified"

	// ExecutableLamportChange,
	ExecutableLamportChange = "ExecutableLamportChange"

	// ExecutableAccountNotRentExempt,
	ExecutableAccountNotRentExempt = "ExecutableAccountNotRentExempt"

	// UnsupportedProgramId,
	UnsupportedProgramId = "UnsupportedProgramId"

	// CallDepth,
	CallDepth = "CallDepth"

	// MissingAccount,
	MissingAccount = "MissingAccount"

	// ReentrancyNotAllowed,
	ReentrancyNotAllowed = "ReentrancyNotAllowed"

	// MaxSeedLengthExceeded,
	MaxSeedLengthExceeded = "MaxSeedLengthExceeded"

	// InvalidSeeds,
	InvalidSeeds = "InvalidSeeds"

	// InvalidRealloc,
	InvalidRealloc = "InvalidRealloc"

	// ComputationalBudgetExceeded,
	ComputationalBudgetExceeded = "ComputationalBudgetExceeded"

	// PrivilegeEscalation,
	PrivilegeEscalation = "PrivilegeEscalation"

	// ProgramEnvironmentSetupFailure,
	ProgramEnvironmentSetupFailure = "ProgramEnvironmentSetupFailure"

	// ProgramFailedToComplete,
	ProgramFailedToComplete = "ProgramFailedToComplete"

	// ProgramFailedToCompile,
	ProgramFailedToCompile = "ProgramFailedToCompile"

	// Immutable,
	Immutable = "Immutable"

	// IncorrectAuthority,
	IncorrectAuthority = "IncorrectAuthority"

	// BorshIoError(String),
	BorshIoError = "BorshIoError"

	// AccountNotRentExempt,
	AccountNotRentExempt = "AccountNotRentExempt"

	// InvalidAccountOwner,
	InvalidAccountOwner = "InvalidAccountOwner"

	// ArithmeticOverflow,
	ArithmeticOverflow = "ArithmeticOverflow"

	// UnsupportedSysvar,
	UnsupportedSysvar = "UnsupportedSysvar"

	// IllegalOwner,
	IllegalOwner = "IllegalOwner"

	// MaxAccountsDataAllocationsExceeded,
	MaxAccountsDataAllocationsExceeded = "MaxAccountsDataAllocationsExceeded"

	// MaxAccountsExceeded,
	MaxAccountsExceeded = "MaxAccountsExceeded"

	// MaxInstructionTraceLengthExceeded,
	MaxInstructionTraceLengthExceeded = "MaxInstructionTraceLengthExceeded"

	// BuiltinProgramsMustConsumeComputeUnits,
	BuiltinProgramsMustConsumeComputeUnits = "BuiltinProgramsMustConsumeComputeUnits"
)
