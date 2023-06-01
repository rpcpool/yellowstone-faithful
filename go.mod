module github.com/rpcpool/yellowstone-faithful

go 1.20

replace go.firedancer.io/radiance => ../radiance

require (
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/gagliardetto/binary v0.7.8
	github.com/gagliardetto/solana-go v1.8.3-0.20230302093440-c6043ec381e3
	github.com/gin-gonic/gin v1.9.0
	github.com/ipfs/go-cid v0.4.1
	github.com/ipfs/go-ipld-cbor v0.0.6
	github.com/ipfs/go-libipfs v0.6.2
	github.com/ipld/go-car v0.6.0
	github.com/ipld/go-car/v2 v2.10.0
	github.com/ipld/go-ipld-prime v0.20.1-0.20230329011551-5056175565b0
	github.com/klauspost/compress v1.16.4
	github.com/sourcegraph/jsonrpc2 v0.2.0
	github.com/stretchr/testify v1.8.2
	github.com/urfave/cli/v2 v2.25.3
	github.com/vbauerster/mpb/v8 v8.2.1
	go.firedancer.io/radiance v0.0.0-00010101000000-000000000000
	golang.org/x/sync v0.1.0
	k8s.io/klog/v2 v2.90.1
)

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.14 // indirect
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/bytedance/sonic v1.8.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/dfuse-io/logging v0.0.0-20210109005628-b97a57253f70 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/gagliardetto/treeout v0.1.4 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.11.2 // indirect
	github.com/goccy/go-json v0.10.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-block-format v0.1.2 // indirect
	github.com/ipfs/go-blockservice v0.5.0 // indirect
	github.com/ipfs/go-datastore v0.6.0 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.3.0 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.0 // indirect
	github.com/ipfs/go-ipfs-exchange-interface v0.2.0 // indirect
	github.com/ipfs/go-ipfs-util v0.0.2 // indirect
	github.com/ipfs/go-ipld-format v0.4.0 // indirect
	github.com/ipfs/go-ipld-legacy v0.1.1 // indirect
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-log/v2 v2.5.1 // indirect
	github.com/ipfs/go-merkledag v0.10.0 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/ipfs/go-unixfsnode v1.7.0 // indirect
	github.com/ipfs/go-verifcid v0.0.2 // indirect
	github.com/ipld/go-codec-dagpb v1.6.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/libp2p/go-libp2p v0.27.1 // indirect
	github.com/linxGnu/grocksdb v1.7.16 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/zstdpool-freelist v0.0.0-20201229113212-927304c0c3b1 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.9.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.0 // indirect
	github.com/multiformats/go-multihash v0.2.1 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/novifinancial/serde-reflection/serde-generate/runtime/golang v0.0.0-20220519162058-e5cd3c3b3f3a // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/smartystreets/assertions v1.13.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/streamingfast/logging v0.0.0-20221209193439-bff11742bf4c // indirect
	github.com/teris-io/shortid v0.0.0-20220617161101-71ec9f2aa569 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.9 // indirect
	github.com/whyrusleeping/cbor v0.0.0-20171005072247-63513f603b11 // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20230126041949-52956bd4c9aa // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.mongodb.org/mongo-driver v1.11.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/term v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)
