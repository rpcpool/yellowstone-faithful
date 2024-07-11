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

var ResponseTimeHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "response_time_histogram",
		Help: "Response time histogram",
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

var IndexLookups = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "index_lookups",
		Help:    "Index lookups",
		Buckets: prometheus.ExponentialBuckets(0.000001, 10, 10),
	},
	[]string{"index_type"},
)

var CarLookups = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "car_lookups",
		Help:    "Car lookups",
		Buckets: prometheus.ExponentialBuckets(0.000001, 10, 10),
	},
	[]string{"car"},
)
