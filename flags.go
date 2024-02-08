package main

// CREDIT: from https://github.com/filecoin-project/lassie/blob/main/cmd/lassie/flags.go
import (
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/lassie/pkg/heyfil"
	"github.com/filecoin-project/lassie/pkg/lassie"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multicodec"
	"github.com/urfave/cli/v2"
)

// IsVerbose is a global var signaling if the CLI is running in
// verbose mode or not (default: false).
var IsVerbose bool

var (
	defaultTempDirectory     string   = os.TempDir() // use the system default temp dir
	verboseLoggingSubsystems []string = []string{    // verbose logging is enabled for these subsystems when using the verbose or very-verbose flags
		"lassie",
		"lassie/retriever",
		"lassie/httpserver",
		"lassie/indexerlookup",
		"lassie/bitswap",
	}
)

const (
	defaultProviderTimeout time.Duration = 20 * time.Second // 20 seconds
)

// FlagVerbose enables verbose mode, which shows info information about
// operations invoked in the CLI.
var FlagVerbose = &cli.BoolFlag{
	Name: "verbose",
	// Aliases: []string{"v"},
	Usage:  "enable verbose mode for logging",
	Action: setLogLevel("INFO"),
}

// FlagVeryVerbose enables very verbose mode, which shows debug information about
// operations invoked in the CLI.
var FlagVeryVerbose = &cli.BoolFlag{
	Name:    "very-verbose",
	Aliases: []string{"vv"},
	Usage:   "enable very verbose mode for debugging",
	Action:  setLogLevel("DEBUG"),
}

// setLogLevel returns a CLI Action function that sets the
// logging level for the given subsystems to the given level.
// It is used as an action for the verbose and very-verbose flags.
func setLogLevel(level string) func(*cli.Context, bool) error {
	return func(cctx *cli.Context, _ bool) error {
		// don't override logging if set in the environment.
		if os.Getenv("GOLOG_LOG_LEVEL") != "" {
			return nil
		}
		// set the logging level for the given subsystems
		for _, name := range verboseLoggingSubsystems {
			_ = log.SetLogLevel(name, level)
		}
		return nil
	}
}

// FlagEventRecorderAuth asks for and provides the authorization token for
// sending metrics to an event recorder API via a Basic auth Authorization
// HTTP header. Value will formatted as "Basic <value>" if provided.
var FlagEventRecorderAuth = &cli.StringFlag{
	Name:        "event-recorder-auth",
	Usage:       "the authorization token for an event recorder API",
	DefaultText: "no authorization token will be used",
	EnvVars:     []string{"LASSIE_EVENT_RECORDER_AUTH"},
}

// FlagEventRecorderUrl asks for and provides the URL for an event recorder API
// to send metrics to.
var FlagEventRecorderInstanceId = &cli.StringFlag{
	Name:        "event-recorder-instance-id",
	Usage:       "the instance ID to use for an event recorder API request",
	DefaultText: "a random v4 uuid",
	EnvVars:     []string{"LASSIE_EVENT_RECORDER_INSTANCE_ID"},
}

// FlagEventRecorderUrl asks for and provides the URL for an event recorder API
// to send metrics to.
var FlagEventRecorderUrl = &cli.StringFlag{
	Name:        "event-recorder-url",
	Usage:       "the url of an event recorder API",
	DefaultText: "no event recorder API will be used",
	EnvVars:     []string{"LASSIE_EVENT_RECORDER_URL"},
}

var (
	providerBlockList    map[peer.ID]bool
	FlagExcludeProviders = &cli.StringFlag{
		Name:        "exclude-providers",
		DefaultText: "All providers allowed",
		Usage:       "Provider peer IDs, separated by a comma. Example: 12D3KooWBSTEYMLSu5FnQjshEVah9LFGEZoQt26eacCEVYfedWA4",
		EnvVars:     []string{"LASSIE_EXCLUDE_PROVIDERS"},
		Action: func(cctx *cli.Context, v string) error {
			// Do nothing if given an empty string
			if v == "" {
				return nil
			}

			providerBlockList = make(map[peer.ID]bool)
			vs := strings.Split(v, ",")
			for _, v := range vs {
				peerID, err := peer.Decode(v)
				if err != nil {
					return err
				}
				providerBlockList[peerID] = true
			}
			return nil
		},
	}
)

var fetchProviderAddrInfos []peer.AddrInfo

var FlagAllowProviders = &cli.StringFlag{
	Name:        "providers",
	Aliases:     []string{"provider"},
	DefaultText: "Providers will be discovered automatically",
	Usage: "Comma-separated addresses of providers, to use instead of " +
		"automatic discovery. Accepts full multiaddrs including peer ID, " +
		"multiaddrs without peer ID and url-style addresses for HTTP and " +
		"Filecoin SP f0 actor addresses. Lassie will attempt to connect to the " +
		"peer(s). Example: " +
		"/ip4/1.2.3.4/tcp/1234/p2p/12D3KooWBSTEYMLSu5FnQjshEVah9LFGEZoQt26eacCEVYfedWA4,http://ipfs.io,f01234",
	EnvVars: []string{"LASSIE_ALLOW_PROVIDERS"},
	Action: func(cctx *cli.Context, v string) error {
		// Do nothing if given an empty string
		if v == "" {
			return nil
		}

		// in case we have been given filecoin actor addresses we can look them up
		// with heyfil and translate to full multiaddrs, otherwise this is a
		// pass-through
		trans, err := heyfil.Heyfil{TranslateFaddr: true}.TranslateAll(strings.Split(v, ","))
		if err != nil {
			return err
		}
		fetchProviderAddrInfos, err = types.ParseProviderStrings(strings.Join(trans, ","))
		return err
	},
}

var (
	protocols     []multicodec.Code
	FlagProtocols = &cli.StringFlag{
		Name:        "protocols",
		DefaultText: "bitswap,graphsync,http",
		Usage:       "List of retrieval protocols to use, separated by a comma",
		EnvVars:     []string{"LASSIE_SUPPORTED_PROTOCOLS"},
		Action: func(cctx *cli.Context, v string) error {
			// Do nothing if given an empty string
			if v == "" {
				return nil
			}

			var err error
			protocols, err = types.ParseProtocolsString(v)
			return err
		},
	}
)

var FlagTempDir = &cli.StringFlag{
	Name:        "tempdir",
	Aliases:     []string{"td"},
	Usage:       "directory to store temporary files while downloading",
	Value:       defaultTempDirectory,
	DefaultText: "os temp directory",
	EnvVars:     []string{"LASSIE_TEMP_DIRECTORY"},
}

var FlagBitswapConcurrency = &cli.IntFlag{
	Name:    "bitswap-concurrency",
	Usage:   "maximum number of concurrent bitswap requests",
	Value:   lassie.DefaultBitswapConcurrency,
	EnvVars: []string{"LASSIE_BITSWAP_CONCURRENCY"},
}

var FlagBitswapConcurrencyPerRetrieval = &cli.IntFlag{
	Name:    "bitswap-concurrency-per-retrieval",
	Usage:   "maximum number of concurrent bitswap requests per retrieval",
	Value:   lassie.DefaultBitswapConcurrencyPerRetrieval,
	EnvVars: []string{"LASSIE_BITSWAP_CONCURRENCY_PER_RETRIEVAL"},
}

var FlagGlobalTimeout = &cli.DurationFlag{
	Name:    "global-timeout",
	Aliases: []string{"gt"},
	Usage:   "consider it an error after not completing a retrieval after this amount of time",
	EnvVars: []string{"LASSIE_GLOBAL_TIMEOUT"},
}

var FlagProviderTimeout = &cli.DurationFlag{
	Name:    "provider-timeout",
	Aliases: []string{"pt"},
	Usage:   "consider it an error after not receiving a response from a storage provider after this amount of time",
	Value:   defaultProviderTimeout,
	EnvVars: []string{"LASSIE_PROVIDER_TIMEOUT"},
}

var FlagIPNIEndpoint = &cli.StringFlag{
	Name:        "ipni-endpoint",
	Aliases:     []string{"ipni"},
	DefaultText: "Defaults to https://cid.contact",
	Usage:       "HTTP endpoint of the IPNI instance used to discover providers.",
}

func ResetGlobalFlags() {
	// Reset global variables here so that they are not used
	// in subsequent calls to commands during testing.
	fetchProviderAddrInfos = make([]peer.AddrInfo, 0)
	protocols = make([]multicodec.Code, 0)
	providerBlockList = make(map[peer.ID]bool)
}
