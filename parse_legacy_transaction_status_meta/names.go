package transaction_status_meta_serde_agave

func (*InstructionError__GenericError) String() string             { return "GenericError" }
func (*InstructionError__InvalidArgument) String() string          { return "InvalidArgument" }
func (*InstructionError__InvalidInstructionData) String() string   { return "InvalidInstructionData" }
func (*InstructionError__InvalidAccountData) String() string       { return "InvalidAccountData" }
func (*InstructionError__AccountDataTooSmall) String() string      { return "AccountDataTooSmall" }
func (*InstructionError__InsufficientFunds) String() string        { return "InsufficientFunds" }
func (*InstructionError__IncorrectProgramId) String() string       { return "IncorrectProgramId" }
func (*InstructionError__MissingRequiredSignature) String() string { return "MissingRequiredSignature" }
func (*InstructionError__AccountAlreadyInitialized) String() string {
	return "AccountAlreadyInitialized"
}
func (*InstructionError__UninitializedAccount) String() string  { return "UninitializedAccount" }
func (*InstructionError__UnbalancedInstruction) String() string { return "UnbalancedInstruction" }
func (*InstructionError__ModifiedProgramId) String() string     { return "ModifiedProgramId" }
func (*InstructionError__ExternalAccountLamportSpend) String() string {
	return "ExternalAccountLamportSpend"
}

func (*InstructionError__ExternalAccountDataModified) String() string {
	return "ExternalAccountDataModified"
}
func (*InstructionError__ReadonlyLamportChange) String() string    { return "ReadonlyLamportChange" }
func (*InstructionError__ReadonlyDataModified) String() string     { return "ReadonlyDataModified" }
func (*InstructionError__DuplicateAccountIndex) String() string    { return "DuplicateAccountIndex" }
func (*InstructionError__ExecutableModified) String() string       { return "ExecutableModified" }
func (*InstructionError__RentEpochModified) String() string        { return "RentEpochModified" }
func (*InstructionError__NotEnoughAccountKeys) String() string     { return "NotEnoughAccountKeys" }
func (*InstructionError__AccountDataSizeChanged) String() string   { return "AccountDataSizeChanged" }
func (*InstructionError__AccountNotExecutable) String() string     { return "AccountNotExecutable" }
func (*InstructionError__AccountBorrowFailed) String() string      { return "AccountBorrowFailed" }
func (*InstructionError__AccountBorrowOutstanding) String() string { return "AccountBorrowOutstanding" }
func (*InstructionError__DuplicateAccountOutOfSync) String() string {
	return "DuplicateAccountOutOfSync"
}
func (*InstructionError__Custom) String() string                  { return "Custom" }
func (*InstructionError__InvalidError) String() string            { return "InvalidError" }
func (*InstructionError__ExecutableDataModified) String() string  { return "ExecutableDataModified" }
func (*InstructionError__ExecutableLamportChange) String() string { return "ExecutableLamportChange" }
func (*InstructionError__ExecutableAccountNotRentExempt) String() string {
	return "ExecutableAccountNotRentExempt"
}
func (*InstructionError__UnsupportedProgramId) String() string  { return "UnsupportedProgramId" }
func (*InstructionError__CallDepth) String() string             { return "CallDepth" }
func (*InstructionError__MissingAccount) String() string        { return "MissingAccount" }
func (*InstructionError__ReentrancyNotAllowed) String() string  { return "ReentrancyNotAllowed" }
func (*InstructionError__MaxSeedLengthExceeded) String() string { return "MaxSeedLengthExceeded" }
func (*InstructionError__InvalidSeeds) String() string          { return "InvalidSeeds" }
func (*InstructionError__InvalidRealloc) String() string        { return "InvalidRealloc" }
func (*InstructionError__ComputationalBudgetExceeded) String() string {
	return "ComputationalBudgetExceeded"
}
func (*InstructionError__PrivilegeEscalation) String() string { return "PrivilegeEscalation" }
func (*InstructionError__ProgramEnvironmentSetupFailure) String() string {
	return "ProgramEnvironmentSetupFailure"
}
func (*InstructionError__ProgramFailedToComplete) String() string { return "ProgramFailedToComplete" }
func (*InstructionError__ProgramFailedToCompile) String() string  { return "ProgramFailedToCompile" }
func (*InstructionError__Immutable) String() string               { return "Immutable" }
func (*InstructionError__IncorrectAuthority) String() string      { return "IncorrectAuthority" }
func (*InstructionError__BorshIoError) String() string            { return "BorshIoError" }
func (*InstructionError__AccountNotRentExempt) String() string    { return "AccountNotRentExempt" }
func (*InstructionError__InvalidAccountOwner) String() string     { return "InvalidAccountOwner" }
func (*InstructionError__ArithmeticOverflow) String() string      { return "ArithmeticOverflow" }
func (*InstructionError__UnsupportedSysvar) String() string       { return "UnsupportedSysvar" }
func (*InstructionError__IllegalOwner) String() string            { return "IllegalOwner" }
func (*InstructionError__MaxAccountsDataAllocationsExceeded) String() string {
	return "MaxAccountsDataAllocationsExceeded"
}
func (*InstructionError__MaxAccountsExceeded) String() string { return "MaxAccountsExceeded" }
func (*InstructionError__MaxInstructionTraceLengthExceeded) String() string {
	return "MaxInstructionTraceLengthExceeded"
}

func (*InstructionError__BuiltinProgramsMustConsumeComputeUnits) String() string {
	return "BuiltinProgramsMustConsumeComputeUnits"
}
func (*TransactionError__AccountInUse) String() string            { return "AccountInUse" }
func (*TransactionError__AccountLoadedTwice) String() string      { return "AccountLoadedTwice" }
func (*TransactionError__AccountNotFound) String() string         { return "AccountNotFound" }
func (*TransactionError__ProgramAccountNotFound) String() string  { return "ProgramAccountNotFound" }
func (*TransactionError__InsufficientFundsForFee) String() string { return "InsufficientFundsForFee" }
func (*TransactionError__InvalidAccountForFee) String() string    { return "InvalidAccountForFee" }
func (*TransactionError__AlreadyProcessed) String() string        { return "AlreadyProcessed" }
func (*TransactionError__BlockhashNotFound) String() string       { return "BlockhashNotFound" }
func (*TransactionError__InstructionError) String() string        { return "InstructionError" }
func (*TransactionError__CallChainTooDeep) String() string        { return "CallChainTooDeep" }
func (*TransactionError__MissingSignatureForFee) String() string  { return "MissingSignatureForFee" }
func (*TransactionError__InvalidAccountIndex) String() string     { return "InvalidAccountIndex" }
func (*TransactionError__SignatureFailure) String() string        { return "SignatureFailure" }
func (*TransactionError__InvalidProgramForExecution) String() string {
	return "InvalidProgramForExecution"
}
func (*TransactionError__SanitizeFailure) String() string          { return "SanitizeFailure" }
func (*TransactionError__ClusterMaintenance) String() string       { return "ClusterMaintenance" }
func (*TransactionError__AccountBorrowOutstanding) String() string { return "AccountBorrowOutstanding" }
func (*TransactionError__WouldExceedMaxBlockCostLimit) String() string {
	return "WouldExceedMaxBlockCostLimit"
}
func (*TransactionError__UnsupportedVersion) String() string     { return "UnsupportedVersion" }
func (*TransactionError__InvalidWritableAccount) String() string { return "InvalidWritableAccount" }
func (*TransactionError__WouldExceedMaxAccountCostLimit) String() string {
	return "WouldExceedMaxAccountCostLimit"
}

func (*TransactionError__WouldExceedAccountDataBlockLimit) String() string {
	return "WouldExceedAccountDataBlockLimit"
}
func (*TransactionError__TooManyAccountLocks) String() string { return "TooManyAccountLocks" }
func (*TransactionError__AddressLookupTableNotFound) String() string {
	return "AddressLookupTableNotFound"
}

func (*TransactionError__InvalidAddressLookupTableOwner) String() string {
	return "InvalidAddressLookupTableOwner"
}

func (*TransactionError__InvalidAddressLookupTableData) String() string {
	return "InvalidAddressLookupTableData"
}

func (*TransactionError__InvalidAddressLookupTableIndex) String() string {
	return "InvalidAddressLookupTableIndex"
}
func (*TransactionError__InvalidRentPayingAccount) String() string { return "InvalidRentPayingAccount" }
func (*TransactionError__WouldExceedMaxVoteCostLimit) String() string {
	return "WouldExceedMaxVoteCostLimit"
}

func (*TransactionError__WouldExceedAccountDataTotalLimit) String() string {
	return "WouldExceedAccountDataTotalLimit"
}
func (*TransactionError__DuplicateInstruction) String() string     { return "DuplicateInstruction" }
func (*TransactionError__InsufficientFundsForRent) String() string { return "InsufficientFundsForRent" }
func (*TransactionError__MaxLoadedAccountsDataSizeExceeded) String() string {
	return "MaxLoadedAccountsDataSizeExceeded"
}

func (*TransactionError__InvalidLoadedAccountsDataSizeLimit) String() string {
	return "InvalidLoadedAccountsDataSizeLimit"
}
func (*TransactionError__ResanitizationNeeded) String() string { return "ResanitizationNeeded" }
func (*TransactionError__ProgramExecutionTemporarilyRestricted) String() string {
	return "ProgramExecutionTemporarilyRestricted"
}
func (*TransactionError__UnbalancedTransaction) String() string   { return "UnbalancedTransaction" }
func (*TransactionError__ProgramCacheHitMaxLimit) String() string { return "ProgramCacheHitMaxLimit" }
func (*TransactionError__CommitCancelled) String() string         { return "CommitCancelled" }
