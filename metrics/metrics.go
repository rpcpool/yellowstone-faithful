package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RpcRequestByMethod = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rpc_requests_by_method",
		Help: "RPC requests by method",
	},
	[]string{"method"},
)

var EpochsAvailable = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "epoch_available",
		Help: "Epochs available",
	},
	[]string{"epoch"},
)

var StatusCode = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "status_code",
		Help: "Status code",
	},
	[]string{"code"},
)

var MethodToCode = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_code",
		Help: "Method to code",
	},
	[]string{"method", "code"},
)

var MethodToSuccessOrFailure = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_success_or_failure",
		Help: "Method to success or failure",
	},
	[]string{"method", "status"},
)

var MethodToNumProxied = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_num_proxied",
		Help: "Method to num proxied",
	},
	[]string{"method"},
)

// - Version information of this binary
var Version = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "version",
		Help: "Version information of this binary",
	},
	[]string{"started_at", "tag", "commit", "compiler", "goarch", "goos", "goamd64", "vcs", "vcs_revision", "vcs_time", "vcs_modified"},
)

var RpcResponseLatencyHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "rpc_response_latency_histogram",
		Help:    "RPC response latency histogram",
		Buckets: latencyBuckets,
	},
	[]string{"rpc_method"},
)

var RpcResponseLatencySummary = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name:       "rpc_response_latency_summary",
		Help:       "RPC response latency summary with percentiles",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.01, 0.99: 0.001},
		MaxAge:     time.Minute * 10,
	},
	[]string{"rpc_method"},
)

var IndexLookupHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "index_lookup_latency_histogram",
		Help:    "Index lookup latency",
		Buckets: latencyBuckets,
	},
	[]string{"index_type", "is_remote", "is_split_car"},
)

var CarLookupHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "car_lookup_latency_histogram",
		Help:    "Car lookup latency",
		Buckets: latencyBuckets,
	},
	[]string{"car", "is_remote", "is_split_car"},
)

// Cache metrics
var CacheHitMissTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cache_hit_miss_total",
		Help: "Total cache hits and misses",
	},
	[]string{"operation", "result"},
)

// Request count metrics
var TransactionCountPerRequest = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "transaction_count_per_request",
		Help:    "Number of transactions per request",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 500, 1000, 5000, 10000},
	},
	[]string{"method"},
)

var SignatureCountPerRequest = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "signature_count_per_request",
		Help:    "Number of signatures per request",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 500, 1000, 5000, 10000},
	},
	[]string{"method"},
)

// Index lookup metrics
var IndexLookupTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "index_lookup_total",
		Help: "Total index lookups",
	},
	[]string{"index_type", "result"},
)

var latencyBuckets = []float64{
	// fractional seconds from 0 to 1, with increments of 0.05 (= 50 ms)
	0, 0.025,
	0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45,
	0.5, 0.55, 0.6, 0.65, 0.7, 0.75, 0.8, 0.85, 0.9, 0.95,
	1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 10,
}

var ErrBlockNotFound = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "err_block_not_found",
		Help: "A block was not found because either skipped or not available in current set of epochs",
	},
)

var diskCollectorInstance *diskCollector

func init() {
	diskCollectorInstance = NewDiskCollector(nil)
	prometheus.MustRegister(diskCollectorInstance)
}

func MaybeAddDiskDevice(device string) {
	if diskCollectorInstance != nil {
		diskCollectorInstance.AddDevice(device)
	}
}

var RemoteFileHttpRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "splitcarfetcher_http_requests_total",
		Help: "Total number of HTTP requests made by splitcarfetcher.",
	},
	[]string{"method", "code"},
)
