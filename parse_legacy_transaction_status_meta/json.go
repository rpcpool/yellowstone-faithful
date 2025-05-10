package transaction_status_meta_serde_agave

import "encoding/json"

func (obj TransactionError__AccountInUse) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__AccountLoadedTwice) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__AccountNotFound) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__ProgramAccountNotFound) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InsufficientFundsForFee) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidAccountForFee) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__AlreadyProcessed) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__BlockhashNotFound) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InstructionError) MarshalJSON() ([]byte, error) {
	// {"InstructionError":[8,{"GenericError":{}}]}
	return json.Marshal(
		map[string]any{
			"InstructionError": []any{
				obj.ErrorCode,
				obj.Error,
			},
		},
	)
}

func (obj TransactionError__CallChainTooDeep) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__MissingSignatureForFee) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidAccountIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__SignatureFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidProgramForExecution) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__SanitizeFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__ClusterMaintenance) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__AccountBorrowOutstanding) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__WouldExceedMaxBlockCostLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__UnsupportedVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidWritableAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__WouldExceedMaxAccountCostLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__WouldExceedAccountDataBlockLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__TooManyAccountLocks) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__AddressLookupTableNotFound) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidAddressLookupTableOwner) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidAddressLookupTableData) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidAddressLookupTableIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidRentPayingAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__WouldExceedMaxVoteCostLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__WouldExceedAccountDataTotalLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__DuplicateInstruction) MarshalJSON() ([]byte, error) {
	// {"DuplicateInstruction":[3]}
	return json.Marshal(
		map[string]any{
			"DuplicateInstruction": uint8(obj),
		},
	)
}

func (obj TransactionError__InsufficientFundsForRent) MarshalJSON() ([]byte, error) {
	// {"InsufficientFundsForRent":{"account_index":4}}
	return json.Marshal(
		map[string]any{
			"InsufficientFundsForRent": map[string]any{
				"account_index": obj.AccountIndex,
			},
		},
	)
}

func (obj TransactionError__MaxLoadedAccountsDataSizeExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__InvalidLoadedAccountsDataSizeLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__ResanitizationNeeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__ProgramExecutionTemporarilyRestricted) MarshalJSON() ([]byte, error) {
	// {"ProgramExecutionTemporarilyRestricted":{"account_index":4}}
	return json.Marshal(
		map[string]any{
			"ProgramExecutionTemporarilyRestricted": map[string]any{
				"account_index": obj.AccountIndex,
			},
		},
	)
}

func (obj TransactionError__UnbalancedTransaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__ProgramCacheHitMaxLimit) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj TransactionError__CommitCancelled) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__GenericError) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidArgument) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidInstructionData) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidAccountData) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountDataTooSmall) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InsufficientFunds) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__IncorrectProgramId) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MissingRequiredSignature) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountAlreadyInitialized) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__UninitializedAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__UnbalancedInstruction) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ModifiedProgramId) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExternalAccountLamportSpend) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExternalAccountDataModified) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ReadonlyLamportChange) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ReadonlyDataModified) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__DuplicateAccountIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExecutableModified) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__RentEpochModified) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__NotEnoughAccountKeys) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountDataSizeChanged) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountNotExecutable) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountBorrowFailed) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__AccountBorrowOutstanding) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__DuplicateAccountOutOfSync) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__Custom) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"Custom": uint32(obj),
		},
	)
}

func (obj InstructionError__InvalidError) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExecutableDataModified) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExecutableLamportChange) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ExecutableAccountNotRentExempt) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__UnsupportedProgramId) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__CallDepth) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MissingAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ReentrancyNotAllowed) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MaxSeedLengthExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidSeeds) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidRealloc) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ComputationalBudgetExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__PrivilegeEscalation) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ProgramEnvironmentSetupFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ProgramFailedToComplete) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ProgramFailedToCompile) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__Immutable) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__IncorrectAuthority) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__BorshIoError) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"BorshIoError": string(obj),
		},
	)
}

func (obj InstructionError__AccountNotRentExempt) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__InvalidAccountOwner) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__ArithmeticOverflow) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__UnsupportedSysvar) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__IllegalOwner) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MaxAccountsDataAllocationsExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MaxAccountsExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__MaxInstructionTraceLengthExceeded) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}

func (obj InstructionError__BuiltinProgramsMustConsumeComputeUnits) MarshalJSON() ([]byte, error) {
	return json.Marshal(obj.String())
}
