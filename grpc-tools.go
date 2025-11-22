package main

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"k8s.io/klog/v2"
)

type StreamTransactionsFilterExecutable struct {
	Vote            *bool
	Failed          *bool
	AccountInclude  solana.PublicKeySlice
	AccountExclude  solana.PublicKeySlice
	AccountRequired solana.PublicKeySlice
}

func fromStreamTransactionsFilter(filter *old_faithful_grpc.StreamTransactionsFilter) (*StreamTransactionsFilterExecutable, error) {
	if filter == nil {
		klog.V(4).Info("StreamTransactions filter is nil, no filtering will be applied")
		return nil, nil
	}

	klog.V(4).Infof("Parsing StreamTransactions filter: vote=%v, failed=%v, account_include=%v, account_exclude=%v, account_required=%v",
		filter.Vote, filter.Failed, filter.AccountInclude, filter.AccountExclude, filter.AccountRequired)

	out := &StreamTransactionsFilterExecutable{}
	if filter.Vote != nil {
		out.Vote = ptrToBool(*filter.Vote)
		klog.V(4).Infof("Set vote filter: %v", *out.Vote)
	}
	if filter.Failed != nil {
		out.Failed = ptrToBool(*filter.Failed)
		klog.V(4).Infof("Set failed filter: %v", *out.Failed)
	}
	var err error
	out.AccountInclude, err = stringSliceToPublicKeySlice(filter.AccountInclude)
	if err != nil {
		klog.Errorf("Failed to parse AccountInclude filter %v: %v", filter.AccountInclude, err)
		return nil, fmt.Errorf("failed to parse AccountInclude: %w", err)
	}
	if len(out.AccountInclude) > 0 {
		klog.V(4).Infof("Set account_include filter with %d accounts: %v", len(out.AccountInclude), out.AccountInclude)
	}

	out.AccountExclude, err = stringSliceToPublicKeySlice(filter.AccountExclude)
	if err != nil {
		klog.Errorf("Failed to parse AccountExclude filter %v: %v", filter.AccountExclude, err)
		return nil, fmt.Errorf("failed to parse AccountExclude: %w", err)
	}
	if len(out.AccountExclude) > 0 {
		klog.V(4).Infof("Set account_exclude filter with %d accounts: %v", len(out.AccountExclude), out.AccountExclude)
	}

	out.AccountRequired, err = stringSliceToPublicKeySlice(filter.AccountRequired)
	if err != nil {
		klog.Errorf("Failed to parse AccountRequired filter %v: %v", filter.AccountRequired, err)
		return nil, fmt.Errorf("failed to parse AccountRequired: %w", err)
	}
	if len(out.AccountRequired) > 0 {
		klog.V(4).Infof("Set account_required filter with %d accounts: %v", len(out.AccountRequired), out.AccountRequired)
	}

	klog.V(4).Info("Successfully parsed StreamTransactions filter")
	return out, nil
}

// StreamTransactionsFilterExecutable.CompileExclusion
func (f *StreamTransactionsFilterExecutable) CompileExclusion() (assertionSlice, error) {
	asserts := make(assertionSlice, 0)
	if f == nil {
		klog.V(4).Info("Filter is nil, no exclusion rules will be compiled")
		return asserts, nil
	}

	klog.V(4).Info("Compiling StreamTransactions filter exclusion rules")

	if f.Vote != nil && !*f.Vote { // If vote is false, we should filter out vote transactions
		klog.V(4).Info("Adding vote exclusion filter (excluding vote transactions)")
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If is vote, then exclude=true
			isVote := IsSimpleVoteTransaction(tx)
			if isVote {
				klog.V(5).Info("Excluding vote transaction")
			}
			return isVote
		})
	}
	if f.Failed != nil && !*f.Failed { // If failed is false, we should filter out failed transactions
		klog.V(4).Info("Adding failed exclusion filter (excluding failed transactions)")
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			if meta == nil {
				klog.V(5).Info("Including transaction with no meta (can't determine if failed)")
				return false // No meta means we can't determine if it failed, so we include it
			}
			isFailed := meta.IsErr()
			if isFailed {
				klog.V(5).Info("Excluding failed transaction")
				return true
			}
			klog.V(5).Info("Including successful transaction")
			return false
		})
	}
	if len(f.AccountInclude) > 0 {
		klog.V(4).Infof("Adding account_include filter for %d accounts: %v", len(f.AccountInclude), f.AccountInclude)
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has any of the included accounts, then exclude=false
			hasIncludedAccount := allAccounts.ContainsAny(f.AccountInclude...)
			if hasIncludedAccount {
				klog.V(5).Infof("Including transaction - found included account(s) in: %v", allAccounts)
				return false // Found an included account, so we do not exclude this transaction
			}
			klog.V(5).Infof("Excluding transaction - no included accounts found in: %v", allAccounts)
			return true // None of the included accounts were found, so we exclude this transaction
		})
	}
	if len(f.AccountExclude) > 0 {
		klog.V(4).Infof("Adding account_exclude filter for %d accounts: %v", len(f.AccountExclude), f.AccountExclude)
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has any of the excluded accounts, then exclude=true
			hasExcludedAccount := allAccounts.ContainsAny(f.AccountExclude...)
			if hasExcludedAccount {
				klog.V(5).Infof("Excluding transaction - found excluded account(s) in: %v", allAccounts)
				return true // Found an excluded account, so we exclude this transaction
			}
			klog.V(5).Infof("Including transaction - no excluded accounts found in: %v", allAccounts)
			return false // None of the excluded accounts were found, so we do not exclude this transaction
		})
	}
	if len(f.AccountRequired) > 0 {
		klog.V(4).Infof("Adding account_required filter for %d accounts: %v", len(f.AccountRequired), f.AccountRequired)
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has all of the required accounts, then exclude=false
			hasAllRequired := allAccounts.ContainsAll(f.AccountRequired)
			if hasAllRequired {
				klog.V(5).Infof("Including transaction - found all required accounts in: %v", allAccounts)
				return false // All required accounts were found, so we do not exclude this transaction
			}
			klog.V(5).Infof("Excluding transaction - missing required account(s) in: %v", allAccounts)
			return true // Not all required accounts were found, so we exclude this transaction
		})
	}
	// If no assertions were added, we return an empty slice
	if len(asserts) == 0 {
		klog.V(2).Info("No assertions compiled for StreamTransactionsFilterExecutable - no filtering will be applied")
	} else {
		klog.V(4).Infof("Compiled %d filter assertion(s) for StreamTransactions", len(asserts))
	}

	return asserts, nil
}

func getAllAccountsFromTransaction(
	tx *solana.Transaction,
	meta *solanatxmetaparsers.TransactionStatusMetaContainer,
) solana.PublicKeySlice {
	allAccounts := tx.Message.AccountKeys
	if meta != nil {
		writable, readonly := meta.GetLoadedAccounts()
		allAccounts = append(allAccounts, writable...)
		allAccounts = append(allAccounts, readonly...)
	}
	return allAccounts
}

type assertionSlice []func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool

func (s assertionSlice) Do(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer) bool {
	if len(s) == 0 {
		return false
	}

	allAccounts := getAllAccountsFromTransaction(tx, meta)
	for _, assert := range s {
		if assert(tx, meta, allAccounts) {
			return true
		}
	}
	return false
}

func ptrToBool(b bool) *bool {
	return &b
}

func blockContainsAccounts(block *old_faithful_grpc.BlockResponse, accounts solana.PublicKeySlice) bool {
	for _, tx := range block.Transactions {
		solTx, err := solana.TransactionFromBytes(tx.GetTransaction())
		if err != nil {
			klog.Errorf("Failed to decode transaction: %v", err)
			continue
		}

		if accounts.ContainsAny(solTx.Message.AccountKeys...) {
			return true
		}

		meta, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(tx.Meta)
		if err != nil {
			klog.Errorf("Failed to parse transaction meta: %v", err)
			continue
		}

		writable, readonly := meta.GetLoadedAccounts()
		if writable.ContainsAny(accounts...) || readonly.ContainsAny(accounts...) {
			return true
		}
	}

	return false
}

func stringSliceToPublicKeySlice(accounts []string) (solana.PublicKeySlice, error) {
	if len(accounts) == 0 {
		klog.V(5).Info("No accounts to parse")
		return nil, nil
	}

	klog.V(4).Infof("Parsing %d account addresses: %v", len(accounts), accounts)
	publicKeys := make(solana.PublicKeySlice, len(accounts))
	for i, acc := range accounts {
		klog.V(5).Infof("Parsing account address %d: %s", i, acc)
		pk, err := solana.PublicKeyFromBase58(acc)
		if err != nil {
			klog.Errorf("Failed to parse account address '%s' at index %d: %v", acc, i, err)
			return nil, fmt.Errorf("failed to parse account %s: %w", acc, err)
		}
		publicKeys[i] = pk
		klog.V(5).Infof("Successfully parsed account %d: %s -> %s", i, acc, pk.String())
	}
	klog.V(4).Infof("Successfully parsed %d account addresses", len(publicKeys))
	return publicKeys, nil
}
