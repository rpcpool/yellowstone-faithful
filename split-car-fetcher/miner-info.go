package splitcarfetcher

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/jellydator/ttlcache/v3"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/ybbus/jsonrpc/v3"
)

type MinerInfoCache struct {
	lotusClient    jsonrpc.RPCClient
	requestTimeout time.Duration
	minerInfoCache *ttlcache.Cache[string, *MinerInfo]
}
type MinerInfo struct {
	PeerIDEncoded           string `json:"PeerID"`
	PeerID                  peer.ID
	MultiaddrsBase64Encoded []string `json:"Multiaddrs"`
	Multiaddrs              []multiaddr.Multiaddr
}

func NewMinerInfo(
	lotusClient jsonrpc.RPCClient,
	cacheTTL time.Duration,
	requestTimeout time.Duration,
) *MinerInfoCache {
	minerInfoCache := ttlcache.New[string, *MinerInfo](
		ttlcache.WithTTL[string, *MinerInfo](cacheTTL),
		ttlcache.WithDisableTouchOnHit[string, *MinerInfo](),
	)

	return &MinerInfoCache{
		lotusClient:    lotusClient,
		requestTimeout: requestTimeout,
		minerInfoCache: minerInfoCache,
	}
}

func (d *MinerInfoCache) GetProviderInfo(ctx context.Context, provider address.Address) (*MinerInfo, error) {
	file := d.minerInfoCache.Get(provider.String())
	if file != nil && !file.IsExpired() {
		return file.Value(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, d.requestTimeout)
	defer cancel()
	minerInfo, err := retryExponentialBackoff(ctx,
		func() (*MinerInfo, error) {
			return (&MinerInfoFetcher{Client: d.lotusClient}).GetProviderInfo(ctx, provider.String())
		},
		time.Second*2,
		5,
	)
	if err != nil {
		return nil, err
	}
	d.minerInfoCache.Set(provider.String(), minerInfo, ttlcache.DefaultTTL)
	return minerInfo, nil
}

type MinerInfoFetcher struct {
	Client jsonrpc.RPCClient
}

func retryExponentialBackoff[T any](
	ctx context.Context,
	fn func() (T, error),
	startingBackoff time.Duration,
	maxRetries int,
) (T, error) {
	var err error
	var out T
	for i := 0; i < maxRetries; i++ {
		out, err = fn()
		if err == nil {
			return out, nil
		}
		select {
		case <-ctx.Done():
			return out, fmt.Errorf("context done: %w; last error: %s", ctx.Err(), err)
		case <-time.After(startingBackoff):
			startingBackoff *= 2
		}
	}
	return out, err
}

func (m *MinerInfoFetcher) GetProviderInfo(ctx context.Context, provider string) (*MinerInfo, error) {
	minerInfo := new(MinerInfo)
	err := m.Client.CallFor(ctx, minerInfo, "Filecoin.StateMinerInfo", provider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get miner info for %s: %w", provider, err)
	}

	minerInfo.Multiaddrs = make([]multiaddr.Multiaddr, len(minerInfo.MultiaddrsBase64Encoded))
	for i, addr := range minerInfo.MultiaddrsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode multiaddr %s: %w", addr, err)
		}
		minerInfo.Multiaddrs[i], err = multiaddr.NewMultiaddrBytes(decoded)
		if err != nil {
			return nil, fmt.Errorf("failed to parse multiaddr %s: %w", addr, err)
		}
	}
	minerInfo.PeerID, err = peer.Decode(minerInfo.PeerIDEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode peer id %s: %w", minerInfo.PeerIDEncoded, err)
	}

	return minerInfo, nil
}
