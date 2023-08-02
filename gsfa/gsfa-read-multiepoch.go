package gsfa

import (
	"context"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/gsfa/offsetstore"
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
) (EpochToSignatures, error) {
	if limit <= 0 {
		return nil, nil
	}
	sigs := make(EpochToSignatures)
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
		for _, sig := range epochSigs {
			sigs[epochNum] = append(sigs[epochNum], sig)
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

type EpochToSignatures map[uint64][]solana.Signature

// Count returns the number of signatures in the EpochToSignatures.
func (e EpochToSignatures) Count() int {
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
) (EpochToSignatures, error) {
	if limit <= 0 {
		return make(EpochToSignatures), nil
	}
	return multi.iterBeforeUntil(ctx, pk, limit, before, until)
}

// GetBeforeUntil gets the signatures for the given public key,
// before the given slot.
func (multi *GsfaReaderMultiepoch) iterBeforeUntil(
	ctx context.Context,
	pk solana.PublicKey,
	limit int,
	before *solana.Signature, // Before this signature, exclusive (i.e. get signatures older than this signature, excluding it).
	until *solana.Signature, // Until this signature, inclusive (i.e. stop at this signature, including it).
) (EpochToSignatures, error) {
	if limit <= 0 {
		return make(EpochToSignatures), nil
	}

	sigs := make(EpochToSignatures)
	reachedBefore := false
	if before == nil {
		reachedBefore = true
	}

epochLoop:
	for readerIndex, index := range multi.epochs {
		epochNum, ok := index.GetEpoch()
		if !ok {
			return nil, fmt.Errorf("epoch is not set for the #%d provided gsfa reader", readerIndex)
		}

		locs, err := index.offsets.Get(context.Background(), pk)
		if err != nil {
			if offsetstore.IsNotFound(err) {
				continue epochLoop
			}
			return nil, fmt.Errorf("error while getting initial offset: %w", err)
		}
		debugln("locs.OffsetToFirst:", locs)

		next := locs.OffsetToLatest // Start from the latest, and go back in time.

		for {
			if next == 0 {
				continue epochLoop
			}
			if limit > 0 && sigs.Count() >= limit {
				break epochLoop
			}
			sigIndexes, newNext, err := index.ll.Read(next)
			if err != nil {
				return nil, fmt.Errorf("error while reading linked log with next=%d: %w", next, err)
			}
			debugln("sigIndexes:", sigIndexes, "newNext:", newNext)
			next = newNext
			for _, sigIndex := range sigIndexes {
				sig, err := index.sff.Get(sigIndex)
				if err != nil {
					return nil, fmt.Errorf("error while getting signature at index=%d: %w", sigIndex, err)
				}
				if !reachedBefore && sig == *before {
					reachedBefore = true
					continue
				}
				if !reachedBefore {
					continue
				}
				if limit > 0 && sigs.Count() >= limit {
					break epochLoop
				}
				sigs[epochNum] = append(sigs[epochNum], sig)
				if until != nil && sig == *until {
					break epochLoop
				}
			}
		}
	}
	return sigs, nil
}
