package main

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"k8s.io/klog/v2"
)

// FilterAction defines the desired state of a transaction.
type FilterAction int

const (
	ActionNeutral FilterAction = iota
	ActionInclude
	ActionExclude
)

// FilterFlow defines whether the pipeline should continue or stop.
type FilterFlow int

const (
	FlowContinue FilterFlow = iota
	FlowStop
)

// StreamTransactionsFilterExecutable holds the parsed filtering criteria.
type StreamTransactionsFilterExecutable struct {
	Vote            *bool
	Failed          *bool
	AccountInclude  map[solana.PublicKey]struct{}
	AccountExclude  map[solana.PublicKey]struct{}
	AccountRequired solana.PublicKeySlice
}

// fromStreamTransactionsFilter converts the gRPC filter proto into an executable filtering struct.
func fromStreamTransactionsFilter(filter *old_faithful_grpc.StreamTransactionsFilter) (*StreamTransactionsFilterExecutable, error) {
	if filter == nil {
		klog.V(4).Info("StreamTransactions filter is nil, no filtering will be applied")
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
	include, err := stringSliceToPublicKeySlice(filter.AccountInclude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountInclude: %w", err)
	}
	if len(include) > 0 {
		out.AccountInclude = sliceToMap(include)
	}

	exclude, err := stringSliceToPublicKeySlice(filter.AccountExclude)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountExclude: %w", err)
	}
	if len(exclude) > 0 {
		out.AccountExclude = sliceToMap(exclude)
	}

	out.AccountRequired, err = stringSliceToPublicKeySlice(filter.AccountRequired)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AccountRequired: %w", err)
	}

	return out, nil
}

func sliceToMap(slice solana.PublicKeySlice) map[solana.PublicKey]struct{} {
	m := make(map[solana.PublicKey]struct{}, len(slice))
	for _, pk := range slice {
		m[pk] = struct{}{}
	}
	return m
}

// FilterStep is a single unit of logic in the state machine.
type FilterStep func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow)

// filterPipeline is a collection of FilterSteps.
type filterPipeline []FilterStep

// CompileExclusion builds the state machine pipeline matching Rust filter logic.
func (f *StreamTransactionsFilterExecutable) CompileExclusion() (filterPipeline, error) {
	pipeline := make(filterPipeline, 0)
	if f == nil {
		return pipeline, nil
	}

	// 2. Vote check (Strict equality matching Rust logic)
	if f.Vote != nil {
		pipeline = append(pipeline, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow) {
			if *f.Vote != IsSimpleVoteTransaction(tx) {
				return ActionExclude, FlowStop
			}
			return ActionNeutral, FlowContinue
		})
	}

	// 3. Failed check (Strict equality matching Rust logic)
	if f.Failed != nil {
		pipeline = append(pipeline, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow) {
			isFailed := meta != nil && meta.IsErr()
			if *f.Failed != isFailed {
				return ActionExclude, FlowStop
			}
			return ActionNeutral, FlowContinue
		})
	}

	// 4. Account Include (Whitelist / Intersection)
	if len(f.AccountInclude) > 0 {
		pipeline = append(pipeline, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow) {
			found := false
			for _, acc := range allAccounts {
				if _, ok := f.AccountInclude[acc]; ok {
					found = true
					break
				}
			}
			if !found {
				return ActionExclude, FlowStop
			}
			return ActionNeutral, FlowContinue
		})
	}

	// 5. Account Exclude (Blacklist / Negative Intersection)
	if len(f.AccountExclude) > 0 {
		pipeline = append(pipeline, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow) {
			for _, acc := range allAccounts {
				if _, ok := f.AccountExclude[acc]; ok {
					return ActionExclude, FlowStop
				}
			}
			return ActionNeutral, FlowContinue
		})
	}

	// 6. Account Required (Mandatory Subset)
	if len(f.AccountRequired) > 0 {
		pipeline = append(pipeline, func(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer, allAccounts solana.PublicKeySlice) (FilterAction, FilterFlow) {
			if !allAccounts.ContainsAll(f.AccountRequired) {
				return ActionExclude, FlowStop
			}
			return ActionNeutral, FlowContinue
		})
	}

	return pipeline, nil
}

// Do executes the state machine. Returns true if EXCLUDED, false if INCLUDED.
func (p filterPipeline) Do(tx *solana.Transaction, meta *solanatxmetaparsers.TransactionStatusMetaContainer) bool {
	allAccounts := getAllAccountsFromTransaction(tx, meta)

	// Default state is Include. Any mismatches in the pipeline guards transition to Exclude.
	currentAction := ActionInclude
	for _, step := range p {
		action, flow := step(tx, meta, allAccounts)
		if action != ActionNeutral {
			currentAction = action
		}
		if flow == FlowStop {
			break
		}
	}

	return currentAction == ActionExclude
}

// getAllAccountsFromTransaction aggregates keys from message and meta.
func getAllAccountsFromTransaction(
	tx *solana.Transaction,
	meta *solanatxmetaparsers.TransactionStatusMetaContainer,
) solana.PublicKeySlice {
	if meta == nil {
		return tx.Message.AccountKeys
	}

	writable, readonly := meta.GetLoadedAccounts()
	total := len(tx.Message.AccountKeys) + len(writable) + len(readonly)

	allAccounts := make(solana.PublicKeySlice, 0, total)
	allAccounts = append(allAccounts, tx.Message.AccountKeys...)
	allAccounts = append(allAccounts, writable...)
	allAccounts = append(allAccounts, readonly...)

	return allAccounts
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

func ptrToBool(b bool) *bool {
	return &b
}
