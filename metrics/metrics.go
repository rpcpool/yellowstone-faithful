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

var IndexLookupHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "index_lookup_latency_histogram",
		Help:    "Index lookup latency",
		Buckets: prometheus.ExponentialBuckets(0.000001, 10, 10),
	},
	[]string{"index_type", "is_remote", "is_split_car"},
)

var CarLookupHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "car_lookup_latency_histogram",
		Help:    "Car lookup latency",
		Buckets: prometheus.ExponentialBuckets(0.000001, 10, 10),
	},
	[]string{"car", "is_remote"},
)

var RpcResponseLatencyHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "rpc_response_latency_histogram",
		Help:    "RPC response latency histogram",
		Buckets: prometheus.ExponentialBuckets(0.000001, 10, 10),
	},
	[]string{"rpc_method"},
)
