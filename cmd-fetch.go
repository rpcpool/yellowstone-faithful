package main

import (
	"github.com/urfave/cli/v2"
)

// import (
// 	"context"
// 	"fmt"
// 	"io"
// 	"net/url"
// 	"os"
// 	"strings"

// 	"github.com/dustin/go-humanize"
// 	"github.com/filecoin-project/lassie/pkg/aggregateeventrecorder"
// 	"github.com/filecoin-project/lassie/pkg/events"
// 	"github.com/filecoin-project/lassie/pkg/indexerlookup"
// 	"github.com/filecoin-project/lassie/pkg/lassie"
// 	"github.com/filecoin-project/lassie/pkg/net/host"
// 	"github.com/filecoin-project/lassie/pkg/retriever"
// 	"github.com/filecoin-project/lassie/pkg/storage"
// 	"github.com/filecoin-project/lassie/pkg/types"
// 	"github.com/google/uuid"
// 	"github.com/ipfs/go-cid"
// 	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
// 	"github.com/libp2p/go-libp2p"
// 	"github.com/libp2p/go-libp2p/core/peer"
// 	"github.com/multiformats/go-multicodec"
// 	"github.com/urfave/cli/v2"
// 	"k8s.io/klog/v2"
// )

// var fetchProviderAddrInfos []peer.AddrInfo

// var (
// 	providerBlockList    map[peer.ID]bool
// 	FlagExcludeProviders = &cli.StringFlag{
// 		Name:        "exclude-providers",
// 		DefaultText: "All providers allowed",
// 		Usage:       "Provider peer IDs, seperated by a comma. Example: 12D3KooWBSTEYMLSu5FnQjshEVah9LFGEZoQt26eacCEVYfedWA4",
// 		EnvVars:     []string{"LASSIE_EXCLUDE_PROVIDERS"},
// 		Action: func(cctx *cli.Context, v string) error {
// 			// Do nothing if given an empty string
// 			if v == "" {
// 				return nil
// 			}

// 			providerBlockList = make(map[peer.ID]bool)
// 			vs := strings.Split(v, ",")
// 			for _, v := range vs {
// 				peerID, err := peer.Decode(v)
// 				if err != nil {
// 					return err
// 				}
// 				providerBlockList[peerID] = true
// 			}
// 			return nil
// 		},
// 	}
// )

// var (
// 	protocols     []multicodec.Code
// 	FlagProtocols = &cli.StringFlag{
// 		Name:        "protocols",
// 		DefaultText: "bitswap,graphsync,http",
// 		Usage:       "List of retrieval protocols to use, seperated by a comma",
// 		EnvVars:     []string{"LASSIE_SUPPORTED_PROTOCOLS"},
// 		Action: func(cctx *cli.Context, v string) error {
// 			// Do nothing if given an empty string
// 			if v == "" {
// 				return nil
// 			}

// 			var err error
// 			protocols, err = types.ParseProtocolsString(v)
// 			return err
// 		},
// 	}
// )

func newCmd_Fetch() *cli.Command {
	return &cli.Command{
		Name:        "fetch",
		Description: "Fetch Solana data from Filecoin/IPFS",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			// &cli.StringFlag{
			// 	Name:        "providers",
			// 	Aliases:     []string{"provider"},
			// 	DefaultText: "Providers will be discovered automatically",
			// 	Usage:       "Addresses of providers, including peer IDs, to use instead of automatic discovery, seperated by a comma. All protocols will be attempted when connecting to these providers. Example: /ip4/1.2.3.4/tcp/1234/p2p/12D3KooWBSTEYMLSu5FnQjshEVah9LFGEZoQt26eacCEVYfedWA4",
			// 	Action: func(cctx *cli.Context, v string) error {
			// 		// Do nothing if given an empty string
			// 		if v == "" {
			// 			return nil
			// 		}

			// 		var err error
			// 		fetchProviderAddrInfos, err = types.ParseProviderStrings(v)
			// 		return err
			// 	},
			// },
			// FlagExcludeProviders,
			// FlagProtocols,
		},
		Action: func(c *cli.Context) error {
			panic("not implemented")
		},
	}
}

// func Fetch(cctx *cli.Context) error {
// 	if cctx.Args().Len() != 1 {
// 		return fmt.Errorf("usage: lassie fetch [-o <CAR file>] [-t <timeout>] <CID>[/path/to/content]")
// 	}

// 	ctx := cctx.Context
// 	msgWriter := cctx.App.ErrWriter
// 	dataWriter := cctx.App.Writer

// 	progress := cctx.Bool("progress")
// 	providerTimeout := cctx.Duration("provider-timeout")
// 	globalTimeout := cctx.Duration("global-timeout")
// 	dagScope := cctx.String("dag-scope")
// 	tempDir := cctx.String("tempdir")
// 	bitswapConcurrency := cctx.Int("bitswap-concurrency")
// 	eventRecorderURL := cctx.String("event-recorder-url")
// 	authToken := cctx.String("event-recorder-auth")
// 	instanceID := cctx.String("event-recorder-instance-id")

// 	rootCid, path, err := parseCidPath(cctx.Args().Get(0))
// 	if err != nil {
// 		return err
// 	}

// 	providerTimeoutOpt := lassie.WithProviderTimeout(providerTimeout)

// 	host, err := host.InitHost(ctx, []libp2p.Option{})
// 	if err != nil {
// 		return err
// 	}
// 	hostOpt := lassie.WithHost(host)
// 	lassieOpts := []lassie.LassieOption{providerTimeoutOpt, hostOpt}

// 	if len(fetchProviderAddrInfos) > 0 {
// 		finderOpt := lassie.WithFinder(retriever.NewDirectCandidateFinder(host, fetchProviderAddrInfos))
// 		if cctx.IsSet("ipni-endpoint") {
// 			klog.Warning("Ignoring ipni-endpoint flag since direct provider is specified")
// 		}
// 		lassieOpts = append(lassieOpts, finderOpt)
// 	} else if cctx.IsSet("ipni-endpoint") {
// 		endpoint := cctx.String("ipni-endpoint")
// 		endpointUrl, err := url.Parse(endpoint)
// 		if err != nil {
// 			klog.Errorln("Failed to parse IPNI endpoint as URL", "err", err)
// 			return fmt.Errorf("cannot parse given IPNI endpoint %s as valid URL: %w", endpoint, err)
// 		}
// 		finder, err := indexerlookup.NewCandidateFinder(indexerlookup.WithHttpEndpoint(endpointUrl))
// 		if err != nil {
// 			klog.Errorln("Failed to instantiate IPNI candidate finder", "err", err)
// 			return err
// 		}
// 		lassieOpts = append(lassieOpts, lassie.WithFinder(finder))
// 		klog.Infoln("Using explicit IPNI endpoint to find candidates", "endpoint", endpoint)
// 	}

// 	if len(providerBlockList) > 0 {
// 		lassieOpts = append(lassieOpts, lassie.WithProviderBlockList(providerBlockList))
// 	}

// 	if len(protocols) > 0 {
// 		lassieOpts = append(lassieOpts, lassie.WithProtocols(protocols))
// 	}

// 	if globalTimeout > 0 {
// 		lassieOpts = append(lassieOpts, lassie.WithGlobalTimeout(globalTimeout))
// 	}

// 	if tempDir != "" {
// 		lassieOpts = append(lassieOpts, lassie.WithTempDir(tempDir))
// 	} else {
// 		tempDir = os.TempDir()
// 	}

// 	if bitswapConcurrency > 0 {
// 		lassieOpts = append(lassieOpts, lassie.WithBitswapConcurrency(bitswapConcurrency))
// 	}

// 	lassie, err := lassie.NewLassie(ctx, lassieOpts...)
// 	if err != nil {
// 		return err
// 	}

// 	// create and subscribe an event recorder API if configured
// 	setupLassieEventRecorder(ctx, eventRecorderURL, authToken, instanceID, lassie)

// 	if len(fetchProviderAddrInfos) == 0 {
// 		fmt.Fprintf(msgWriter, "Fetching %s", rootCid.String()+path)
// 	} else {
// 		fmt.Fprintf(msgWriter, "Fetching %s from %v", rootCid.String()+path, fetchProviderAddrInfos)
// 	}
// 	if progress {
// 		fmt.Fprintln(msgWriter)
// 		pp := &progressPrinter{writer: msgWriter}
// 		lassie.RegisterSubscriber(pp.subscriber)
// 	}

// 	outfile := fmt.Sprintf("%s.car", rootCid)
// 	if cctx.IsSet("output") {
// 		outfile = cctx.String("output")
// 	}

// 	var carWriter *storage.DeferredCarWriter
// 	if outfile == "-" { // stdout
// 		// we need the onlyWriter because stdout is presented as an os.File, and
// 		// therefore pretend to support seeks, so feature-checking in go-car
// 		// will make bad assumptions about capabilities unless we hide it
// 		carWriter = storage.NewDeferredCarWriterForStream(rootCid, &onlyWriter{dataWriter})
// 	} else {
// 		carWriter = storage.NewDeferredCarWriterForPath(rootCid, outfile)
// 	}
// 	carStore := storage.NewCachingTempStore(carWriter.BlockWriteOpener(), tempDir)
// 	defer carStore.Close()

// 	var blockCount int
// 	var byteLength uint64
// 	carWriter.OnPut(func(putBytes int) {
// 		blockCount++
// 		byteLength += uint64(putBytes)
// 		if !progress {
// 			fmt.Fprint(msgWriter, ".")
// 		} else {
// 			fmt.Fprintf(msgWriter, "\rReceived %d blocks / %s...", blockCount, humanize.IBytes(byteLength))
// 		}
// 	}, false)

// 	request, err := types.NewRequestForPath(carStore, rootCid, path, types.CarScope(dagScope))
// 	if err != nil {
// 		return err
// 	}
// 	// setup preload storage for bitswap, the temporary CAR store can set up a
// 	// separate preload space in its storage
// 	request.PreloadLinkSystem = cidlink.DefaultLinkSystem()
// 	preloadStore := carStore.PreloadStore()
// 	request.PreloadLinkSystem.SetReadStorage(preloadStore)
// 	request.PreloadLinkSystem.SetWriteStorage(preloadStore)
// 	request.PreloadLinkSystem.TrustedStorage = true

// 	stats, err := lassie.Fetch(ctx, request, func(types.RetrievalEvent) {})
// 	if err != nil {
// 		fmt.Fprintln(msgWriter)
// 		return err
// 	}
// 	fmt.Fprintf(msgWriter, "\nFetched [%s] from [%s]:\n"+
// 		"\tDuration: %s\n"+
// 		"\t  Blocks: %d\n"+
// 		"\t   Bytes: %s\n",
// 		rootCid,
// 		stats.StorageProviderId,
// 		stats.Duration,
// 		blockCount,
// 		humanize.IBytes(stats.Size),
// 	)

// 	return nil
// }

// func parseCidPath(cpath string) (cid.Cid, string, error) {
// 	cstr := strings.Split(cpath, "/")[0]
// 	path := strings.TrimPrefix(cpath, cstr)
// 	rootCid, err := cid.Parse(cstr)
// 	if err != nil {
// 		return cid.Undef, "", err
// 	}
// 	return rootCid, path, nil
// }

// // setupLassieEventRecorder creates and subscribes an EventRecorder if an event recorder URL is given
// func setupLassieEventRecorder(
// 	ctx context.Context,
// 	eventRecorderURL string,
// 	authToken string,
// 	instanceID string,
// 	lassie *lassie.Lassie,
// ) {
// 	if eventRecorderURL != "" {
// 		if instanceID == "" {
// 			uuid, err := uuid.NewRandom()
// 			if err != nil {
// 				klog.Warningln("failed to generate default event recorder instance ID UUID, no instance ID will be provided", "err", err)
// 			}
// 			instanceID = uuid.String() // returns "" if uuid is invalid
// 		}

// 		eventRecorder := aggregateeventrecorder.NewAggregateEventRecorder(ctx, aggregateeventrecorder.EventRecorderConfig{
// 			InstanceID:            instanceID,
// 			EndpointURL:           eventRecorderURL,
// 			EndpointAuthorization: authToken,
// 		})
// 		lassie.RegisterSubscriber(eventRecorder.RetrievalEventSubscriber())
// 		klog.Infoln("Reporting retrieval events to event recorder API", "url", eventRecorderURL, "instance_id", instanceID)
// 	}
// }

// type progressPrinter struct {
// 	candidatesFound int
// 	writer          io.Writer
// }

// func (pp *progressPrinter) subscriber(event types.RetrievalEvent) {
// 	switch ret := event.(type) {
// 	case events.RetrievalEventStarted:
// 		switch ret.Phase() {
// 		case types.IndexerPhase:
// 			fmt.Fprintf(pp.writer, "\rQuerying indexer for %s...\n", ret.PayloadCid())
// 		case types.QueryPhase:
// 			fmt.Fprintf(pp.writer, "\rQuerying [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 		case types.RetrievalPhase:
// 			fmt.Fprintf(pp.writer, "\rRetrieving from [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 		}
// 	case events.RetrievalEventConnected:
// 		switch ret.Phase() {
// 		case types.QueryPhase:
// 			fmt.Fprintf(pp.writer, "\rQuerying [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 		case types.RetrievalPhase:
// 			fmt.Fprintf(pp.writer, "\rRetrieving from [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 		}
// 	case events.RetrievalEventProposed:
// 		fmt.Fprintf(pp.writer, "\rRetrieving from [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 	case events.RetrievalEventAccepted:
// 		fmt.Fprintf(pp.writer, "\rRetrieving from [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 	case events.RetrievalEventFirstByte:
// 		fmt.Fprintf(pp.writer, "\rRetrieving from [%s] (%s)...\n", types.Identifier(ret), ret.Code())
// 	case events.RetrievalEventCandidatesFound:
// 		pp.candidatesFound = len(ret.Candidates())
// 	case events.RetrievalEventCandidatesFiltered:
// 		num := "all of them"
// 		if pp.candidatesFound != len(ret.Candidates()) {
// 			num = fmt.Sprintf("%d of them", len(ret.Candidates()))
// 		} else if pp.candidatesFound == 1 {
// 			num = "it"
// 		}
// 		if len(fetchProviderAddrInfos) > 0 {
// 			fmt.Fprintf(pp.writer, "Found %d storage providers candidates from the indexer, querying %s:\n", pp.candidatesFound, num)
// 		} else {
// 			fmt.Fprintf(pp.writer, "Using the explicitly specified storage provider(s), querying %s:\n", num)
// 		}
// 		for _, candidate := range ret.Candidates() {
// 			fmt.Fprintf(pp.writer, "\r\t%s, Protocols: %v\n", candidate.MinerPeer.ID, candidate.Metadata.Protocols())
// 		}
// 	case events.RetrievalEventFailed:
// 		if ret.Phase() == types.IndexerPhase {
// 			fmt.Fprintf(pp.writer, "\rRetrieval failure from indexer: %s\n", ret.ErrorMessage())
// 		} else {
// 			fmt.Fprintf(pp.writer, "\rRetrieval failure for [%s]: %s\n", types.Identifier(ret), ret.ErrorMessage())
// 		}
// 	case events.RetrievalEventSuccess:
// 		// noop, handled at return from Retrieve()
// 	}
// }

// type onlyWriter struct {
// 	w io.Writer
// }

// func (ow *onlyWriter) Write(p []byte) (n int, err error) {
// 	return ow.w.Write(p)
// }
