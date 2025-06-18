package transaction_status_meta_serde_agave

import "encoding/json"

func (obj TransactionError__AccountInUse) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountInUse"`), nil
}

func (obj TransactionError__AccountLoadedTwice) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountLoadedTwice"`), nil
}

func (obj TransactionError__AccountNotFound) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountNotFound"`), nil
}

func (obj TransactionError__ProgramAccountNotFound) MarshalJSON() ([]byte, error) {
	return []byte(`"ProgramAccountNotFound"`), nil
}

func (obj TransactionError__InsufficientFundsForFee) MarshalJSON() ([]byte, error) {
	return []byte(`"InsufficientFundsForFee"`), nil
}

func (obj TransactionError__InvalidAccountForFee) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAccountForFee"`), nil
}

func (obj TransactionError__AlreadyProcessed) MarshalJSON() ([]byte, error) {
	return []byte(`"AlreadyProcessed"`), nil
}

func (obj TransactionError__BlockhashNotFound) MarshalJSON() ([]byte, error) {
	return []byte(`"BlockhashNotFound"`), nil
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
	return []byte(`"CallChainTooDeep"`), nil
}

func (obj TransactionError__MissingSignatureForFee) MarshalJSON() ([]byte, error) {
	return []byte(`"MissingSignatureForFee"`), nil
}

func (obj TransactionError__InvalidAccountIndex) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAccountIndex"`), nil
}

func (obj TransactionError__SignatureFailure) MarshalJSON() ([]byte, error) {
	return []byte(`"SignatureFailure"`), nil
}

func (obj TransactionError__InvalidProgramForExecution) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidProgramForExecution"`), nil
}

func (obj TransactionError__SanitizeFailure) MarshalJSON() ([]byte, error) {
	return []byte(`"SanitizeFailure"`), nil
}

func (obj TransactionError__ClusterMaintenance) MarshalJSON() ([]byte, error) {
	return []byte(`"ClusterMaintenance"`), nil
}

func (obj TransactionError__AccountBorrowOutstanding) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountBorrowOutstanding"`), nil
}

func (obj TransactionError__WouldExceedMaxBlockCostLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"WouldExceedMaxBlockCostLimit"`), nil
}

func (obj TransactionError__UnsupportedVersion) MarshalJSON() ([]byte, error) {
	return []byte(`"UnsupportedVersion"`), nil
}

func (obj TransactionError__InvalidWritableAccount) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidWritableAccount"`), nil
}

func (obj TransactionError__WouldExceedMaxAccountCostLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"WouldExceedMaxAccountCostLimit"`), nil
}

func (obj TransactionError__WouldExceedAccountDataBlockLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"WouldExceedAccountDataBlockLimit"`), nil
}

func (obj TransactionError__TooManyAccountLocks) MarshalJSON() ([]byte, error) {
	return []byte(`"TooManyAccountLocks"`), nil
}

func (obj TransactionError__AddressLookupTableNotFound) MarshalJSON() ([]byte, error) {
	return []byte(`"AddressLookupTableNotFound"`), nil
}

func (obj TransactionError__InvalidAddressLookupTableOwner) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAddressLookupTableOwner"`), nil
}

func (obj TransactionError__InvalidAddressLookupTableData) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAddressLookupTableData"`), nil
}

func (obj TransactionError__InvalidAddressLookupTableIndex) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAddressLookupTableIndex"`), nil
}

func (obj TransactionError__InvalidRentPayingAccount) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidRentPayingAccount"`), nil
}

func (obj TransactionError__WouldExceedMaxVoteCostLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"WouldExceedMaxVoteCostLimit"`), nil
}

func (obj TransactionError__WouldExceedAccountDataTotalLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"WouldExceedAccountDataTotalLimit"`), nil
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
	return []byte(`"MaxLoadedAccountsDataSizeExceeded"`), nil
}

func (obj TransactionError__InvalidLoadedAccountsDataSizeLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidLoadedAccountsDataSizeLimit"`), nil
}

func (obj TransactionError__ResanitizationNeeded) MarshalJSON() ([]byte, error) {
	return []byte(`"ResanitizationNeeded"`), nil
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
	return []byte(`"UnbalancedTransaction"`), nil
}

func (obj TransactionError__ProgramCacheHitMaxLimit) MarshalJSON() ([]byte, error) {
	return []byte(`"ProgramCacheHitMaxLimit"`), nil
}

func (obj TransactionError__CommitCancelled) MarshalJSON() ([]byte, error) {
	return []byte(`"CommitCancelled"`), nil
}

func (obj InstructionError__GenericError) MarshalJSON() ([]byte, error) {
	return []byte(`"GenericError"`), nil
}

func (obj InstructionError__InvalidArgument) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidArgument"`), nil
}

func (obj InstructionError__InvalidInstructionData) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidInstructionData"`), nil
}

func (obj InstructionError__InvalidAccountData) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAccountData"`), nil
}

func (obj InstructionError__AccountDataTooSmall) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountDataTooSmall"`), nil
}

func (obj InstructionError__InsufficientFunds) MarshalJSON() ([]byte, error) {
	return []byte(`"InsufficientFunds"`), nil
}

func (obj InstructionError__IncorrectProgramId) MarshalJSON() ([]byte, error) {
	return []byte(`"IncorrectProgramId"`), nil
}

func (obj InstructionError__MissingRequiredSignature) MarshalJSON() ([]byte, error) {
	return []byte(`"MissingRequiredSignature"`), nil
}

func (obj InstructionError__AccountAlreadyInitialized) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountAlreadyInitialized"`), nil
}

func (obj InstructionError__UninitializedAccount) MarshalJSON() ([]byte, error) {
	return []byte(`"UninitializedAccount"`), nil
}

func (obj InstructionError__UnbalancedInstruction) MarshalJSON() ([]byte, error) {
	return []byte(`"UnbalancedInstruction"`), nil
}

func (obj InstructionError__ModifiedProgramId) MarshalJSON() ([]byte, error) {
	return []byte(`"ModifiedProgramId"`), nil
}

func (obj InstructionError__ExternalAccountLamportSpend) MarshalJSON() ([]byte, error) {
	return []byte(`"ExternalAccountLamportSpend"`), nil
}

func (obj InstructionError__ExternalAccountDataModified) MarshalJSON() ([]byte, error) {
	return []byte(`"ExternalAccountDataModified"`), nil
}

func (obj InstructionError__ReadonlyLamportChange) MarshalJSON() ([]byte, error) {
	return []byte(`"ReadonlyLamportChange"`), nil
}

func (obj InstructionError__ReadonlyDataModified) MarshalJSON() ([]byte, error) {
	return []byte(`"ReadonlyDataModified"`), nil
}

func (obj InstructionError__DuplicateAccountIndex) MarshalJSON() ([]byte, error) {
	return []byte(`"DuplicateAccountIndex"`), nil
}

func (obj InstructionError__ExecutableModified) MarshalJSON() ([]byte, error) {
	return []byte(`"ExecutableModified"`), nil
}

func (obj InstructionError__RentEpochModified) MarshalJSON() ([]byte, error) {
	return []byte(`"RentEpochModified"`), nil
}

func (obj InstructionError__NotEnoughAccountKeys) MarshalJSON() ([]byte, error) {
	return []byte(`"NotEnoughAccountKeys"`), nil
}

func (obj InstructionError__AccountDataSizeChanged) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountDataSizeChanged"`), nil
}

func (obj InstructionError__AccountNotExecutable) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountNotExecutable"`), nil
}

func (obj InstructionError__AccountBorrowFailed) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountBorrowFailed"`), nil
}

func (obj InstructionError__AccountBorrowOutstanding) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountBorrowOutstanding"`), nil
}

func (obj InstructionError__DuplicateAccountOutOfSync) MarshalJSON() ([]byte, error) {
	return []byte(`"DuplicateAccountOutOfSync"`), nil
}

func (obj InstructionError__Custom) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"Custom": uint32(obj),
		},
	)
}

func (obj InstructionError__InvalidError) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidError"`), nil
}

func (obj InstructionError__ExecutableDataModified) MarshalJSON() ([]byte, error) {
	return []byte(`"ExecutableDataModified"`), nil
}

func (obj InstructionError__ExecutableLamportChange) MarshalJSON() ([]byte, error) {
	return []byte(`"ExecutableLamportChange"`), nil
}

func (obj InstructionError__ExecutableAccountNotRentExempt) MarshalJSON() ([]byte, error) {
	return []byte(`"ExecutableAccountNotRentExempt"`), nil
}

func (obj InstructionError__UnsupportedProgramId) MarshalJSON() ([]byte, error) {
	return []byte(`"UnsupportedProgramId"`), nil
}

func (obj InstructionError__CallDepth) MarshalJSON() ([]byte, error) {
	return []byte(`"CallDepth"`), nil
}

func (obj InstructionError__MissingAccount) MarshalJSON() ([]byte, error) {
	return []byte(`"MissingAccount"`), nil
}

func (obj InstructionError__ReentrancyNotAllowed) MarshalJSON() ([]byte, error) {
	return []byte(`"ReentrancyNotAllowed"`), nil
}

func (obj InstructionError__MaxSeedLengthExceeded) MarshalJSON() ([]byte, error) {
	return []byte(`"MaxSeedLengthExceeded"`), nil
}

func (obj InstructionError__InvalidSeeds) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidSeeds"`), nil
}

func (obj InstructionError__InvalidRealloc) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidRealloc"`), nil
}

func (obj InstructionError__ComputationalBudgetExceeded) MarshalJSON() ([]byte, error) {
	return []byte(`"ComputationalBudgetExceeded"`), nil
}

func (obj InstructionError__PrivilegeEscalation) MarshalJSON() ([]byte, error) {
	return []byte(`"PrivilegeEscalation"`), nil
}

func (obj InstructionError__ProgramEnvironmentSetupFailure) MarshalJSON() ([]byte, error) {
	return []byte(`"ProgramEnvironmentSetupFailure"`), nil
}

func (obj InstructionError__ProgramFailedToComplete) MarshalJSON() ([]byte, error) {
	return []byte(`"ProgramFailedToComplete"`), nil
}

func (obj InstructionError__ProgramFailedToCompile) MarshalJSON() ([]byte, error) {
	return []byte(`"ProgramFailedToCompile"`), nil
}

func (obj InstructionError__Immutable) MarshalJSON() ([]byte, error) {
	return []byte(`"Immutable"`), nil
}

func (obj InstructionError__IncorrectAuthority) MarshalJSON() ([]byte, error) {
	return []byte(`"IncorrectAuthority"`), nil
}

func (obj InstructionError__BorshIoError) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		map[string]any{
			"BorshIoError": string(obj),
		},
	)
}

func (obj InstructionError__AccountNotRentExempt) MarshalJSON() ([]byte, error) {
	return []byte(`"AccountNotRentExempt"`), nil
}

func (obj InstructionError__InvalidAccountOwner) MarshalJSON() ([]byte, error) {
	return []byte(`"InvalidAccountOwner"`), nil
}

func (obj InstructionError__ArithmeticOverflow) MarshalJSON() ([]byte, error) {
	return []byte(`"ArithmeticOverflow"`), nil
}

func (obj InstructionError__UnsupportedSysvar) MarshalJSON() ([]byte, error) {
	return []byte(`"UnsupportedSysvar"`), nil
}

func (obj InstructionError__IllegalOwner) MarshalJSON() ([]byte, error) {
	return []byte(`"IllegalOwner"`), nil
}

func (obj InstructionError__MaxAccountsDataAllocationsExceeded) MarshalJSON() ([]byte, error) {
	return []byte(`"MaxAccountsDataAllocationsExceeded"`), nil
}

func (obj InstructionError__MaxAccountsExceeded) MarshalJSON() ([]byte, error) {
	return []byte(`"MaxAccountsExceeded"`), nil
}

func (obj InstructionError__MaxInstructionTraceLengthExceeded) MarshalJSON() ([]byte, error) {
	return []byte(`"MaxInstructionTraceLengthExceeded"`), nil
}

func (obj InstructionError__BuiltinProgramsMustConsumeComputeUnits) MarshalJSON() ([]byte, error) {
	return []byte(`"BuiltinProgramsMustConsumeComputeUnits"`), nil
}
