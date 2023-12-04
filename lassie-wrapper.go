package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/filecoin-project/lassie/pkg/indexerlookup"
	"github.com/filecoin-project/lassie/pkg/lassie"
	"github.com/filecoin-project/lassie/pkg/net/host"
	"github.com/filecoin-project/lassie/pkg/retriever"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/storage"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	trustlessutils "github.com/ipld/go-trustless-utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

type lassieWrapper struct {
	lassie *lassie.Lassie
}

type RwStorage interface {
	storage.Storage
	storage.WritableStorage
	storage.ReadableStorage
}

func (l *lassieWrapper) GetNodeByCid(ctx context.Context, wantedCid cid.Cid) ([]byte, error) {
	store := NewWrappedMemStore()
	{
		_, err := l.Fetch(
			ctx,
			wantedCid,
			"",
			trustlessutils.DagScopeBlock,
			store,
		)
		if err != nil {
			return nil, err
		}
	}
	for key, node := range store.Bag {
		if cid.MustParse([]byte(key)).Equals(wantedCid) {
			return node, nil
		}
	}
	return nil, nil
}

func (l *lassieWrapper) GetSubgraph(ctx context.Context, wantedCid cid.Cid) (*WrappedMemStore, error) {
	store := NewWrappedMemStore()
	{
		_, err := l.Fetch(
			ctx,
			wantedCid,
			"",
			trustlessutils.DagScopeAll,
			store,
		)
		if err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (l *lassieWrapper) Fetch(
	ctx context.Context,
	rootCid cid.Cid,
	path string,
	dagScope trustlessutils.DagScope,
	store RwStorage,
) (*types.RetrievalStats, error) {
	request, err := types.NewRequestForPath(store, rootCid, path, trustlessutils.DagScope(dagScope), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.PreloadLinkSystem = cidlink.DefaultLinkSystem()
	request.PreloadLinkSystem.SetReadStorage(store)
	request.PreloadLinkSystem.SetWriteStorage(store)
	request.PreloadLinkSystem.TrustedStorage = true

	stats, err := l.lassie.Fetch(ctx, request)
	if err != nil {
		return stats, fmt.Errorf("failed to fetch: %w", err)
	}
	return stats, nil
}

func newLassieWrapper(
	cctx *cli.Context,
	fetchProviderAddrInfos []peer.AddrInfo,
) (*lassieWrapper, error) {
	ctx := cctx.Context

	providerTimeout := cctx.Duration("provider-timeout")
	globalTimeout := cctx.Duration("global-timeout")
	bitswapConcurrency := cctx.Int("bitswap-concurrency")

	providerTimeoutOpt := lassie.WithProviderTimeout(providerTimeout)

	host, err := host.InitHost(ctx, []libp2p.Option{})
	if err != nil {
		return nil, err
	}
	hostOpt := lassie.WithHost(host)
	lassieOpts := []lassie.LassieOption{providerTimeoutOpt, hostOpt}

	if len(fetchProviderAddrInfos) > 0 {
		finderOpt := lassie.WithFinder(retriever.NewDirectCandidateFinder(host, fetchProviderAddrInfos))
		if cctx.IsSet("ipni-endpoint") {
			klog.Warning("Ignoring ipni-endpoint flag since direct provider is specified")
		}
		lassieOpts = append(lassieOpts, finderOpt)
	} else if cctx.IsSet("ipni-endpoint") {
		endpoint := cctx.String("ipni-endpoint")
		endpointUrl, err := url.Parse(endpoint)
		if err != nil {
			klog.Error("Failed to parse IPNI endpoint as URL", "err", err)
			return nil, fmt.Errorf("cannot parse given IPNI endpoint %s as valid URL: %w", endpoint, err)
		}
		finder, err := indexerlookup.NewCandidateFinder(indexerlookup.WithHttpEndpoint(endpointUrl))
		if err != nil {
			klog.Error("Failed to instantiate IPNI candidate finder", "err", err)
			return nil, err
		}
		lassieOpts = append(lassieOpts, lassie.WithFinder(finder))
		klog.Info("Using explicit IPNI endpoint to find candidates", "endpoint", endpoint)
	}

	if len(providerBlockList) > 0 {
		lassieOpts = append(lassieOpts, lassie.WithProviderBlockList(providerBlockList))
	}

	if len(protocols) > 0 {
		lassieOpts = append(lassieOpts, lassie.WithProtocols(protocols))
	}

	if globalTimeout > 0 {
		lassieOpts = append(lassieOpts, lassie.WithGlobalTimeout(globalTimeout))
	}

	if bitswapConcurrency > 0 {
		lassieOpts = append(lassieOpts, lassie.WithBitswapConcurrency(bitswapConcurrency))
	}

	lassie, err := lassie.NewLassie(ctx, lassieOpts...)
	if err != nil {
		return nil, err
	}

	// if eventRecorderCfg.EndpointURL != "" {
	// 	setupLassieEventRecorder(ctx, eventRecorderCfg, lassie)
	// }

	return &lassieWrapper{
		lassie: lassie,
	}, nil
}

type WrappedMemStore struct {
	*memstore.Store
}

func NewWrappedMemStore() *WrappedMemStore {
	return &WrappedMemStore{Store: &memstore.Store{}}
}

func (w *WrappedMemStore) Each(ctx context.Context, cb func(cid.Cid, []byte) error) error {
	for key, value := range w.Store.Bag {
		if err := cb(cid.MustParse([]byte(key)), value); err != nil {
			if errors.Is(err, ErrStopIteration) {
				return nil
			}
			return err
		}
	}
	return nil
}
