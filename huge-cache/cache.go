package hugecache

import (
	"context"
	"errors"
	"strconv"

	"github.com/allegro/bigcache/v3"
	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
)

type Cache struct {
	cache *bigcache.BigCache
}

func NewWithConfig(ctx context.Context, config bigcache.Config) (*Cache, error) {
	cache, err := bigcache.New(ctx, config)
	if err != nil {
		return nil, err
	}
	return &Cache{
		cache: cache,
	}, nil
}

func formatRawCarObjectKey(c cid.Cid) string {
	return "rco-" + c.String()
}

func formatSlotToCidKey(slot uint64) string {
	return "s2c-" + strconv.FormatUint(slot, 10)
}

func formatOffsetAndSizeKey(c cid.Cid) string {
	return "o&s-" + c.String()
}

// PutRawCarObject stores the raw CAR object data.
func (r *Cache) PutRawCarObject(c cid.Cid, data []byte) error {
	return r.cache.Set(formatRawCarObjectKey(c), data)
}

// GetRawCarObject returns the raw CAR object data from the cache if it exists.
func (r *Cache) GetRawCarObject(c cid.Cid) (v []byte, err error, has bool) {
	if v, err := r.cache.Get(formatRawCarObjectKey(c)); err == nil {
		return v, nil, true
	} else {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return nil, nil, false
		}
		return nil, err, false
	}
}

// PutSlotToCid stores the CID for the given slot.
func (r *Cache) PutSlotToCid(slot uint64, c cid.Cid) error {
	return r.cache.Set(formatSlotToCidKey(slot), c.Bytes())
}

// GetSlotToCid returns the CID for the given slot if it exists in the cache.
func (r *Cache) GetSlotToCid(slot uint64) (cid.Cid, error, bool) {
	if v, err := r.cache.Get(formatSlotToCidKey(slot)); err == nil {
		_, parsed, err := cid.CidFromBytes(v)
		if err != nil {
			return cid.Undef, err, false
		}
		return parsed, nil, true
	} else {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return cid.Undef, nil, false
		}
		return cid.Undef, err, false
	}
}

func (r *Cache) PutCidToOffsetAndSize(c cid.Cid, oas *indexes.OffsetAndSize) error {
	if oas == nil {
		return errors.New("offset and size is nil")
	}
	if !oas.IsValid() {
		return errors.New("offset and size is invalid")
	}
	return r.cache.Set(formatOffsetAndSizeKey(c), oas.Bytes())
}

func (r *Cache) GetCidToOffsetAndSize(c cid.Cid) (*indexes.OffsetAndSize, error, bool) {
	if v, err := r.cache.Get(formatOffsetAndSizeKey(c)); err == nil {
		var oas indexes.OffsetAndSize
		if err := oas.FromBytes(v); err != nil {
			return nil, err, false
		}
		return &oas, nil, true
	} else {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return nil, nil, false
		}
		return nil, err, false
	}
}
