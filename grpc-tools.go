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
		return nil, nil
	}

	out := &StreamTransactionsFilterExecutable{}
	if filter.Vote != nil {
		out.Vote = ptrToBool(*filter.Vote)
	}
	if filter.Failed != nil {
		out.Failed = ptrToBool(*filter.Failed)
	}
	var err error
	out.AccountInclude, err = stringSliceToPublicKeySlice(filter.AccountInclude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountInclude: %w", err)
	}
	out.AccountExclude, err = stringSliceToPublicKeySlice(filter.AccountExclude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountExclude: %w", err)
	}
	out.AccountRequired, err = stringSliceToPublicKeySlice(filter.AccountRequired)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountRequired: %w", err)
	}
	return out, nil
}

// StreamTransactionsFilterExecutable.CompileExclusion
func (f *StreamTransactionsFilterExecutable) CompileExclusion() (assertionSlice, error) {
	asserts := make(assertionSlice, 0)
	if f == nil {
		return asserts, nil
	}
	if f.Vote != nil && !*f.Vote { // If vote is false, we should filter out vote transactions
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If is vote, then exclude=true
			return IsSimpleVoteTransaction(tx)
		})
	}
	if f.Failed != nil && !*f.Failed { // If failed is false, we should filter out failed transactions
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If is not failed, then exclude=true
			if meta == nil {
				return true // No meta means we can't determine if it failed, so we include it
			}
			return !meta.IsErr()
		})
	}
	if len(f.AccountInclude) > 0 {
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has any of the included accounts, then exclude=false
			if allAccounts.ContainsAny(f.AccountInclude...) {
				return false // Found an included account, so we do not exclude this transaction
			}
			return true // None of the included accounts were found, so we exclude this transaction
		})
	}
	if len(f.AccountExclude) > 0 {
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has any of the excluded accounts, then exclude=true
			if allAccounts.ContainsAny(f.AccountExclude...) {
				return true // Found an excluded account, so we exclude this transaction
			}
			// If none of the excluded accounts were found, then exclude=false
			return false // None of the excluded accounts were found, so we do not exclude this transaction
		})
	}
	if len(f.AccountRequired) > 0 {
		asserts = append(asserts, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) bool {
			// If has all of the required accounts, then exclude=false
			if allAccounts.ContainsAll(f.AccountRequired) {
				return false // All required accounts were found, so we do not exclude this transaction
			}
			return false // All required accounts were found, so we do not exclude this transaction
		})
	}
	// If no assertions were added, we return an empty slice
	if len(asserts) == 0 {
		klog.V(2).Info("No assertions compiled for StreamTransactionsFilterExecutable")
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
	allAccounts := getAllAccountsFromTransaction(tx, meta)
	for _, assert := range s {
		if !assert(tx, meta, allAccounts) {
			return false
		}
	}
	return true
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
		return nil, nil
	}

	publicKeys := make(solana.PublicKeySlice, len(accounts))
	for i, acc := range accounts {
		pk, err := solana.PublicKeyFromBase58(acc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse account %s: %w", acc, err)
		}
		publicKeys[i] = pk
	}
	return publicKeys, nil
}
