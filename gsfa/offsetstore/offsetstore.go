package offsetstore

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/gagliardetto/solana-go"
	store "github.com/rpcpool/yellowstone-faithful/store"
	storetypes "github.com/rpcpool/yellowstone-faithful/store/types"
)

type errorType string

func (e errorType) Error() string {
	return string(e)
}

const (
	ErrNotSupported = errorType("Operation not supported")
	ErrWrongHash    = errorType("Wrong hash")
)

type OffsetStore struct {
	store *store.Store
}

type Locs struct {
	OffsetToLatest uint64
}

// Open opens a HashedBlockstore with the default index size
func Open(ctx context.Context, indexPath string, dataPath string, options ...store.Option) (*OffsetStore, error) {
	store, err := store.OpenStore(
		ctx,
		store.GsfaPrimary,
		dataPath,
		indexPath,
		options...,
	)
	if err != nil {
		return nil, err
	}
	return &OffsetStore{store}, nil
}

func (as *OffsetStore) Start() {
	as.store.Start()
}

func (as *OffsetStore) Close() error {
	return as.store.Close()
}

func (as *OffsetStore) Delete(ctx context.Context, pk solana.PublicKey) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err := as.store.Remove(pk[:])
	return err
}

// Has indicates if an account exists in the store.
func (as *OffsetStore) Has(ctx context.Context, pk solana.PublicKey) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return as.store.Has(pk[:])
}

// Get returns an account from the store.
func (as *OffsetStore) Get(ctx context.Context, pk solana.PublicKey) (Locs, error) {
	if ctx.Err() != nil {
		return Locs{}, ctx.Err()
	}
	value, found, err := as.store.Get(pk[:])
	if err != nil {
		return Locs{}, err
	}
	if !found {
		return Locs{}, ErrNotFound{PubKey: pk}
	}
	parsed, err := parseLocs(value)
	if err != nil {
		return Locs{}, err
	}
	return parsed, nil
}

func parseLocs(value []byte) (Locs, error) {
	if len(value) != 8 {
		return Locs{}, errors.New("invalid Loc size")
	}
	return Locs{
		readUint64(value[0:8]),
	}, nil
}

// Encode returns the encoded bytes of an account.
func (a Locs) Bytes() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf[0:8], a.OffsetToLatest)
	return buf
}

func readUint64(data []byte) uint64 {
	return binary.LittleEndian.Uint64(data)
}

// GetSize returns the size of an account in the store.
func (as *OffsetStore) GetSize(ctx context.Context, pk solana.PublicKey) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	// unoptimized implementation for now
	size, found, err := as.store.GetSize(pk[:])
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrNotFound{PubKey: pk}
	}
	return int(size), nil
}

// Put puts a given account in the underlying store.
func (as *OffsetStore) Put(ctx context.Context, pk solana.PublicKey, loc Locs) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	err := as.store.Put(pk[:], loc.Bytes())
	// suppress key exist error because this is not expected behavior for a blockstore
	if err == storetypes.ErrKeyExists {
		// TODO: can we make the store mutable?
		return nil
	}
	return err
}

func (as *OffsetStore) Flush() error {
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
func (as *OffsetStore) AllKeysChan(ctx context.Context) (<-chan solana.PublicKey, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	iter := as.store.NewIterator()

	ch := make(chan solana.PublicKey)
	go func() {
		defer close(ch)
		for keyHash, val, err := iter.Next(); err == nil; keyHash, _, err = iter.Next() {
			_ = keyHash
			// parse val[:32] as a pubkey
			pubkey := solana.PublicKeyFromBytes(val[:32])
			select {
			case ch <- pubkey:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (as *OffsetStore) AllValuesChan(ctx context.Context) (<-chan Locs, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	iter := as.store.NewIterator()

	ch := make(chan Locs)
	go func() {
		defer close(ch)
		for keyHash, value, err := iter.Next(); err == nil; keyHash, _, err = iter.Next() {
			_ = keyHash
			parsed, err := parseLocs(value)
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
// The Cid field can be filled in to provide additional context.
type ErrNotFound struct {
	PubKey solana.PublicKey
}

// Error implements the error interface and returns a human-readable
// message for this error.
func (e ErrNotFound) Error() string {
	if e.PubKey.IsZero() {
		return "not found"
	}

	return "could not find entries for " + e.PubKey.String()
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
