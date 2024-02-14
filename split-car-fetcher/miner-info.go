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
		ttlcache.WithDisableTouchOnHit[string, *MinerInfo]())

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

	minerInfo, err := (&MinerInfoFetcher{Client: d.lotusClient}).GetProviderInfo(ctx, provider.String())
	if err != nil {
		return nil, err
	}
	d.minerInfoCache.Set(provider.String(), minerInfo, ttlcache.DefaultTTL)
	return minerInfo, nil
}

type MinerInfoFetcher struct {
	Client jsonrpc.RPCClient
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
