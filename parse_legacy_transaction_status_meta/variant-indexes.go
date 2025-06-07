package transaction_status_meta_serde_agave

func (obj *TransactionError__AccountInUse) GetVariantIndex() int { return 0 }

func (obj *TransactionError__AccountLoadedTwice) GetVariantIndex() int { return 1 }

func (obj *TransactionError__AccountNotFound) GetVariantIndex() int { return 2 }

func (obj *TransactionError__ProgramAccountNotFound) GetVariantIndex() int { return 3 }

func (obj *TransactionError__InsufficientFundsForFee) GetVariantIndex() int { return 4 }

func (obj *TransactionError__InvalidAccountForFee) GetVariantIndex() int { return 5 }

func (obj *TransactionError__AlreadyProcessed) GetVariantIndex() int { return 6 }

func (obj *TransactionError__BlockhashNotFound) GetVariantIndex() int { return 7 }

func (obj *TransactionError__InstructionError) GetVariantIndex() int { return 8 }

func (obj *TransactionError__CallChainTooDeep) GetVariantIndex() int { return 9 }

func (obj *TransactionError__MissingSignatureForFee) GetVariantIndex() int { return 10 }

func (obj *TransactionError__InvalidAccountIndex) GetVariantIndex() int { return 11 }

func (obj *TransactionError__SignatureFailure) GetVariantIndex() int { return 12 }

func (obj *TransactionError__InvalidProgramForExecution) GetVariantIndex() int { return 13 }

func (obj *TransactionError__SanitizeFailure) GetVariantIndex() int { return 14 }

func (obj *TransactionError__ClusterMaintenance) GetVariantIndex() int { return 15 }

func (obj *TransactionError__AccountBorrowOutstanding) GetVariantIndex() int { return 16 }

func (obj *TransactionError__WouldExceedMaxBlockCostLimit) GetVariantIndex() int { return 17 }

func (obj *TransactionError__UnsupportedVersion) GetVariantIndex() int { return 18 }

func (obj *TransactionError__InvalidWritableAccount) GetVariantIndex() int { return 19 }

func (obj *TransactionError__WouldExceedMaxAccountCostLimit) GetVariantIndex() int { return 20 }

func (obj *TransactionError__WouldExceedAccountDataBlockLimit) GetVariantIndex() int { return 21 }

func (obj *TransactionError__TooManyAccountLocks) GetVariantIndex() int { return 22 }

func (obj *TransactionError__AddressLookupTableNotFound) GetVariantIndex() int { return 23 }

func (obj *TransactionError__InvalidAddressLookupTableOwner) GetVariantIndex() int { return 24 }

func (obj *TransactionError__InvalidAddressLookupTableData) GetVariantIndex() int { return 25 }

func (obj *TransactionError__InvalidAddressLookupTableIndex) GetVariantIndex() int { return 26 }

func (obj *TransactionError__InvalidRentPayingAccount) GetVariantIndex() int { return 27 }

func (obj *TransactionError__WouldExceedMaxVoteCostLimit) GetVariantIndex() int { return 28 }

func (obj *TransactionError__WouldExceedAccountDataTotalLimit) GetVariantIndex() int { return 29 }

func (obj *TransactionError__DuplicateInstruction) GetVariantIndex() int { return 30 }

func (obj *TransactionError__InsufficientFundsForRent) GetVariantIndex() int { return 31 }

func (obj *TransactionError__MaxLoadedAccountsDataSizeExceeded) GetVariantIndex() int { return 32 }

func (obj *TransactionError__InvalidLoadedAccountsDataSizeLimit) GetVariantIndex() int { return 33 }

func (obj *TransactionError__ResanitizationNeeded) GetVariantIndex() int { return 34 }

func (obj *TransactionError__ProgramExecutionTemporarilyRestricted) GetVariantIndex() int { return 35 }

func (obj *TransactionError__UnbalancedTransaction) GetVariantIndex() int { return 36 }

func (obj *TransactionError__ProgramCacheHitMaxLimit) GetVariantIndex() int { return 37 }

func (obj *TransactionError__CommitCancelled) GetVariantIndex() int { return 38 }

func (obj *InstructionError__GenericError) GetVariantIndex() int { return 0 }

func (obj *InstructionError__InvalidArgument) GetVariantIndex() int { return 1 }

func (obj *InstructionError__InvalidInstructionData) GetVariantIndex() int { return 2 }

func (obj *InstructionError__InvalidAccountData) GetVariantIndex() int { return 3 }

func (obj *InstructionError__AccountDataTooSmall) GetVariantIndex() int { return 4 }

func (obj *InstructionError__InsufficientFunds) GetVariantIndex() int { return 5 }

func (obj *InstructionError__IncorrectProgramId) GetVariantIndex() int { return 6 }

func (obj *InstructionError__MissingRequiredSignature) GetVariantIndex() int { return 7 }

func (obj *InstructionError__AccountAlreadyInitialized) GetVariantIndex() int { return 8 }

func (obj *InstructionError__UninitializedAccount) GetVariantIndex() int { return 9 }

func (obj *InstructionError__UnbalancedInstruction) GetVariantIndex() int { return 10 }

func (obj *InstructionError__ModifiedProgramId) GetVariantIndex() int { return 11 }

func (obj *InstructionError__ExternalAccountLamportSpend) GetVariantIndex() int { return 12 }

func (obj *InstructionError__ExternalAccountDataModified) GetVariantIndex() int { return 13 }

func (obj *InstructionError__ReadonlyLamportChange) GetVariantIndex() int { return 14 }

func (obj *InstructionError__ReadonlyDataModified) GetVariantIndex() int { return 15 }

func (obj *InstructionError__DuplicateAccountIndex) GetVariantIndex() int { return 16 }

func (obj *InstructionError__ExecutableModified) GetVariantIndex() int { return 17 }

func (obj *InstructionError__RentEpochModified) GetVariantIndex() int { return 18 }

func (obj *InstructionError__NotEnoughAccountKeys) GetVariantIndex() int { return 19 }

func (obj *InstructionError__AccountDataSizeChanged) GetVariantIndex() int { return 20 }

func (obj *InstructionError__AccountNotExecutable) GetVariantIndex() int { return 21 }

func (obj *InstructionError__AccountBorrowFailed) GetVariantIndex() int { return 22 }

func (obj *InstructionError__AccountBorrowOutstanding) GetVariantIndex() int { return 23 }

func (obj *InstructionError__DuplicateAccountOutOfSync) GetVariantIndex() int { return 24 }

func (obj *InstructionError__Custom) GetVariantIndex() int { return 25 }

func (obj *InstructionError__InvalidError) GetVariantIndex() int { return 26 }

func (obj *InstructionError__ExecutableDataModified) GetVariantIndex() int { return 27 }

func (obj *InstructionError__ExecutableLamportChange) GetVariantIndex() int { return 28 }

func (obj *InstructionError__ExecutableAccountNotRentExempt) GetVariantIndex() int { return 29 }

func (obj *InstructionError__UnsupportedProgramId) GetVariantIndex() int { return 30 }

func (obj *InstructionError__CallDepth) GetVariantIndex() int { return 31 }

func (obj *InstructionError__MissingAccount) GetVariantIndex() int { return 32 }

func (obj *InstructionError__ReentrancyNotAllowed) GetVariantIndex() int { return 33 }

func (obj *InstructionError__MaxSeedLengthExceeded) GetVariantIndex() int { return 34 }

func (obj *InstructionError__InvalidSeeds) GetVariantIndex() int { return 35 }

func (obj *InstructionError__InvalidRealloc) GetVariantIndex() int { return 36 }

func (obj *InstructionError__ComputationalBudgetExceeded) GetVariantIndex() int { return 37 }

func (obj *InstructionError__PrivilegeEscalation) GetVariantIndex() int { return 38 }

func (obj *InstructionError__ProgramEnvironmentSetupFailure) GetVariantIndex() int { return 39 }

func (obj *InstructionError__ProgramFailedToComplete) GetVariantIndex() int { return 40 }

func (obj *InstructionError__ProgramFailedToCompile) GetVariantIndex() int { return 41 }

func (obj *InstructionError__Immutable) GetVariantIndex() int { return 42 }

func (obj *InstructionError__IncorrectAuthority) GetVariantIndex() int { return 43 }

func (obj *InstructionError__BorshIoError) GetVariantIndex() int { return 44 }

func (obj *InstructionError__AccountNotRentExempt) GetVariantIndex() int { return 45 }

func (obj *InstructionError__InvalidAccountOwner) GetVariantIndex() int { return 46 }

func (obj *InstructionError__ArithmeticOverflow) GetVariantIndex() int { return 47 }

func (obj *InstructionError__UnsupportedSysvar) GetVariantIndex() int { return 48 }

func (obj *InstructionError__IllegalOwner) GetVariantIndex() int { return 49 }

func (obj *InstructionError__MaxAccountsDataAllocationsExceeded) GetVariantIndex() int { return 50 }

func (obj *InstructionError__MaxAccountsExceeded) GetVariantIndex() int { return 51 }

func (obj *InstructionError__MaxInstructionTraceLengthExceeded) GetVariantIndex() int { return 52 }

func (obj *InstructionError__BuiltinProgramsMustConsumeComputeUnits) GetVariantIndex() int { return 53 }
