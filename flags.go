package main

// CREDIT: from https://github.com/filecoin-project/lassie/blob/main/cmd/lassie/flags.go
import (
	"strings"

	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multicodec"
	"github.com/urfave/cli/v2"
)

// IsVerbose is a global var signaling if the CLI is running in
// verbose mode or not (default: false).
var IsVerbose bool

// FlagVerbose enables verbose mode, which shows verbose information about
// operations invoked in the CLI. It should be included as a flag on the
// top-level command (e.g. lassie -v).
var FlagVerbose = &cli.BoolFlag{
	Name:        "verbose",
	Aliases:     []string{"v"},
	Usage:       "enable verbose mode for logging",
	Destination: &IsVerbose,
}

// IsVeryVerbose is a global var signaling if the CLI is running in
// very verbose mode or not (default: false).
var IsVeryVerbose bool

// FlagVerbose enables verbose mode, which shows verbose information about
// operations invoked in the CLI. It should be included as a flag on the
// top-level command (e.g. lassie -v).
var FlagVeryVerbose = &cli.BoolFlag{
	Name:        "very-verbose",
	Aliases:     []string{"vv"},
	Usage:       "enable very verbose mode for debugging",
	Destination: &IsVeryVerbose,
}

// FlagEventRecorderAuth asks for and provides the authorization token for
// sending metrics to an event recorder API via a Basic auth Authorization
// HTTP header. Value will formatted as "Basic <value>" if provided.
var FlagEventRecorderAuth = &cli.StringFlag{
	Name:    "event-recorder-auth",
	Usage:   "the authorization token for an event recorder API",
	EnvVars: []string{"LASSIE_EVENT_RECORDER_AUTH"},
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
	Name:    "event-recorder-url",
	Usage:   "the url of an event recorder API",
	EnvVars: []string{"LASSIE_EVENT_RECORDER_URL"},
}

var providerBlockList map[peer.ID]bool

var FlagExcludeProviders = &cli.StringFlag{
	Name:        "exclude-providers",
	DefaultText: "All providers allowed",
	Usage:       "Provider peer IDs, seperated by a comma. Example: 12D3KooWBSTEYMLSu5FnQjshEVah9LFGEZoQt26eacCEVYfedWA4",
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

var (
	protocols     []multicodec.Code
	FlagProtocols = &cli.StringFlag{
		Name:        "protocols",
		DefaultText: "bitswap,graphsync,http",
		Usage:       "List of retrieval protocols to use, seperated by a comma",
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
	Value:       "",
	DefaultText: "os temp directory",
	EnvVars:     []string{"LASSIE_TEMP_DIRECTORY"},
}

var FlagBitswapConcurrency = &cli.IntFlag{
	Name:    "bitswap-concurrency",
	Usage:   "maximum number of concurrent bitswap requests per retrieval",
	Value:   6,
	EnvVars: []string{"LASSIE_BITSWAP_CONCURRENCY"},
}
