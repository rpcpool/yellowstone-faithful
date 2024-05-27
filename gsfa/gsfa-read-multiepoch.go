package gsfa

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/rpcpool/yellowstone-faithful/gsfa/linkedlog"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"k8s.io/klog/v2"
)

type GsfaReaderMultiepoch struct {
	epochs []*GsfaReader
}

func NewGsfaReaderMultiepoch(epochs []*GsfaReader) (*GsfaReaderMultiepoch, error) {
	// Check that the epoch is set:
	for i, epoch := range epochs {
		if epoch.epoch == nil {
			return nil, fmt.Errorf("epoch is not set for the #%d provided gsfa reader", i)
		}
	}

	return &GsfaReaderMultiepoch{
		epochs: epochs,
	}, nil
}

// Close closes all the gsfa readers.
func (gsfa *GsfaReaderMultiepoch) Close() error {
	var errs []error
	for _, epoch := range gsfa.epochs {
		err := epoch.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Get gets the signatures for the given public key.
func (gsfa *GsfaReaderMultiepoch) Get(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	fetcher func(uint64, linkedlog.OffsetAndSizeAndBlocktime) (*ipldbindcode.Transaction, error),
) (EpochToTransactionObjects, error) {
	if limit <= 0 {
		return nil, nil
	}
	sigs := make(EpochToTransactionObjects)
	currentLimit := limit
epochLoop:
	for _, epoch := range gsfa.epochs {
		epochSigs, err := epoch.Get(ctx, pk, currentLimit)
		if err != nil {
			return nil, err
		}
		epochNum, ok := epoch.GetEpoch()
		if !ok {
			return nil, fmt.Errorf("epoch is not set for the provided gsfa reader")
		}
		for _, txLoc := range epochSigs {
			tx, err := fetcher(epochNum, txLoc)
			if err != nil {
				return nil, fmt.Errorf("error while fetching signature: %w", err)
			}
			sigs[epochNum] = append(sigs[epochNum], tx)
			currentLimit--
			if currentLimit <= 0 {
				break epochLoop
			}
		}
		if currentLimit <= 0 {
			break epochLoop
		}
	}
	return sigs, nil
}

type EpochToTransactionObjects map[uint64][]*ipldbindcode.Transaction

// Count returns the number of signatures in the EpochToSignatures.
func (e EpochToTransactionObjects) Count() int {
	var count int
	for _, sigs := range e {
		count += len(sigs)
	}
	return count
}

func (multi *GsfaReaderMultiepoch) GetBeforeUntil(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	before *solana.Signature, // Before this signature, exclusive (i.e. get signatures older than this signature, excluding it).
	until *solana.Signature, // Until this signature, inclusive (i.e. stop at this signature, including it).
	fetcher func(uint64, linkedlog.OffsetAndSizeAndBlocktime) (*ipldbindcode.Transaction, error),
) (EpochToTransactionObjects, error) {
	if limit <= 0 {
		return make(EpochToTransactionObjects), nil
	}
	return multi.iterBeforeUntil(ctx, pk, limit, before, until, fetcher)
}

// GetBeforeUntil gets the signatures for the given public key,
// before the given slot.
func (multi *GsfaReaderMultiepoch) iterBeforeUntil(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	before *solana.Signature, // Before this signature, exclusive (i.e. get signatures older than this signature, excluding it).
	until *solana.Signature, // Until this signature, inclusive (i.e. stop at this signature, including it).
	fetcher func(uint64, linkedlog.OffsetAndSizeAndBlocktime) (*ipldbindcode.Transaction, error),
) (EpochToTransactionObjects, error) {
	if limit <= 0 {
		return make(EpochToTransactionObjects), nil
	}

	transactions := make(EpochToTransactionObjects)
	reachedBefore := false
	if before == nil {
		reachedBefore = true
	}

epochLoop:
	for readerIndex, index := range multi.epochs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		epochNum, ok := index.GetEpoch()
		if !ok {
			return nil, fmt.Errorf("epoch is not set for the #%d provided gsfa reader", readerIndex)
		}

		locsStartedAt := time.Now()
		locs, err := index.offsets.Get(pk)
		if err != nil {
			if compactindexsized.IsNotFound(err) {
				continue epochLoop
			}
			return nil, fmt.Errorf("error while getting initial offset: %w", err)
		}
		klog.V(5).Infof("locs.OffsetToFirst took %s", time.Since(locsStartedAt))
		debugln("locs.OffsetToFirst:", locs)

		next := locs // Start from the latest, and go back in time.

		for {
			if next == nil || next.IsZero() { // no previous.
				continue epochLoop
			}
			if limit > 0 && transactions.Count() >= limit {
				break epochLoop
			}
			startedReadAt := time.Now()
			locations, newNext, err := index.ll.ReadWithSize(next.Offset, next.Size)
			if err != nil {
				return nil, fmt.Errorf("error while reading linked log with next=%v: %w", next, err)
			}
			klog.V(5).Infof("ReadWithSize took %s to get %d locs", time.Since(startedReadAt), len(locations))
			if len(locations) == 0 {
				continue epochLoop
			}
			debugln("sigIndexes:", locations, "newNext:", newNext)
			next = &newNext
			for locIndex, txLoc := range locations {
				tx, err := fetcher(epochNum, txLoc)
				if err != nil {
					return nil, fmt.Errorf("error while getting signature at index=%v: %w", txLoc, err)
				}
				sig, err := tx.Signature()
				if err != nil {
					return nil, fmt.Errorf("error while getting signature: %w", err)
				}
				klog.V(5).Infoln(locIndex, "sig:", sig, "epoch:", epochNum)
				if !reachedBefore && sig == *before {
					reachedBefore = true
					continue
				}
				if !reachedBefore {
					continue
				}
				if limit > 0 && transactions.Count() >= limit {
					break epochLoop
				}
				transactions[epochNum] = append(transactions[epochNum], tx)
				if until != nil && sig == *until {
					break epochLoop
				}
			}
		}
	}
	return transactions, nil
}
