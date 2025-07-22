package metrics

import (
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

var latencyBuckets = []float64{
	// fractional seconds from 0 to 1, with increments of 0.05 (= 50 ms)
	0, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45,
	0.5, 0.55, 0.6, 0.65, 0.7, 0.75, 0.8, 0.85, 0.9, 0.95,
	1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5,
	// then 5-10 with increments of 1
	6, 7, 8, 9, 10,
	// then 10-60 with increments of 5
	15, 20, 25, 30, 35, 40, 45, 50, 55, 60,
}

var ErrBlockNotFound = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "err_block_not_found",
		Help: "A block was not found because either skipped or not available in current set of epochs",
	},
)
