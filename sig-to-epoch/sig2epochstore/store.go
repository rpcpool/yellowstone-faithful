package sig2epochstore

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/gagliardetto/solana-go"
	store "github.com/rpcpool/yellowstone-faithful/store"
	storetypes "github.com/rpcpool/yellowstone-faithful/store/types"
)

type Store struct {
	store *store.Store
}

type Epoch struct {
	Epoch uint16
}

// Open opens a HashedBlockstore with the default index size
func Open(ctx context.Context, indexPath string, dataPath string, options ...store.Option) (*Store, error) {
	store, err := store.OpenStore(
		ctx,
		store.SigToEpochPrimary,
		dataPath,
		indexPath,
		options...,
	)
	if err != nil {
		return nil, err
	}
	return &Store{store}, nil
}

func (as *Store) Start() {
	as.store.Start()
}

func (as *Store) Close() error {
	return as.store.Close()
}

func (as *Store) Delete(ctx context.Context, sig solana.Signature) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err := as.store.Remove(sig[:])
	return err
}

func (as *Store) Has(ctx context.Context, sig solana.Signature) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return as.store.Has(sig[:])
}

func (as *Store) Get(ctx context.Context, sig solana.Signature) (Epoch, error) {
	if ctx.Err() != nil {
		return Epoch{}, ctx.Err()
	}
	value, found, err := as.store.Get(sig[:])
	if err != nil {
		return Epoch{}, err
	}
	if !found {
		return Epoch{}, ErrNotFound{Signature: sig}
	}
	parsed, err := parseEpoch(value)
	if err != nil {
		return Epoch{}, err
	}
	return parsed, nil
}

func parseEpoch(value []byte) (Epoch, error) {
	if len(value) != 2 {
		return Epoch{}, errors.New("invalid epoch size")
	}
	return Epoch{
		readUint16(value[0:2]),
	}, nil
}

// Encode returns the encoded bytes of an account.
func (a Epoch) Bytes() []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf[0:2], a.Epoch)
	return buf
}

func readUint16(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data)
}

// GetSize returns the size of an account in the store.
func (as *Store) GetSize(ctx context.Context, sig solana.Signature) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	// unoptimized implementation for now
	size, found, err := as.store.GetSize(sig[:])
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrNotFound{Signature: sig}
	}
	return int(size), nil
}

// Put puts a given account in the underlying store.
func (as *Store) Put(ctx context.Context, sig solana.Signature, loc Epoch) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	err := as.store.Put(sig[:], loc.Bytes())
	// suppress key exist error because this is not expected behavior for a blockstore
	if err == storetypes.ErrKeyExists {
		// TODO: can we make the store mutable?
		return nil
	}
	return err
}

func (as *Store) Flush() error {
	if err := as.store.Flush(); err != nil {
		return err
	}
	if _, err := as.store.Primary().Flush(); err != nil {
		return err
	}
	if err := as.store.Primary().Sync(); err != nil {
		return err
	}
	return nil
}

// AllKeysChan returns a channel from which
// the pubkeys in the AccountStore can be read. It should respect
// the given context, closing the channel if it becomes Done.
func (as *Store) AllKeysChan(ctx context.Context) (<-chan solana.Signature, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	iter := as.store.NewIterator()

	ch := make(chan solana.Signature)
	go func() {
		defer close(ch)
		for key, _, err := iter.Next(); err == nil; key, _, err = iter.Next() {
			sig := solana.SignatureFromBytes(key[:64])
			select {
			case ch <- sig:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (as *Store) AllValuesChan(ctx context.Context) (<-chan Epoch, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	iter := as.store.NewIterator()

	ch := make(chan Epoch)
	go func() {
		defer close(ch)
		for key, value, err := iter.Next(); err == nil; key, _, err = iter.Next() {
			_ = key
			parsed, err := parseEpoch(value)
			if err != nil {
				// TODO: log error
				continue
			}
			select {
			case ch <- parsed:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// ErrNotFound is used to signal when a Node could not be found. The specific
// meaning will depend on the DAGService implementation, which may be trying
// to read nodes locally but also, trying to find them remotely.
//
// The Signature field can be filled in to provide additional context.
type ErrNotFound struct {
	Signature solana.Signature
}

// Error implements the error interface and returns a human-readable
// message for this error.
func (e ErrNotFound) Error() string {
	if e.Signature.IsZero() {
		return "not found"
	}

	return "could not find entries for " + e.Signature.String()
}

// Is allows to check whether any error is of this ErrNotFound type.
// Do not use this directly, but rather errors.Is(yourError, ErrNotFound).
func (e ErrNotFound) Is(err error) bool {
	switch err.(type) {
	case ErrNotFound:
		return true
	default:
		return false
	}
}

// NotFound returns true.
func (e ErrNotFound) NotFound() bool {
	return true
}

// IsNotFound returns if the given error is or wraps an ErrNotFound
// (equivalent to errors.Is(err, ErrNotFound{}))
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound{})
}
