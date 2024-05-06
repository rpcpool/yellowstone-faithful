package main

import (
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// - RPC requests by method (counter)
// - Epochs available epoch_available{epoch="200"} = 1
// - status_code
// - miner ids
// - source type (ipfs/bitwarden/s3/etc)
// - response time histogram

func init() {
	prometheus.MustRegister(metrics_RpcRequestByMethod)
	prometheus.MustRegister(metrics_epochsAvailable)
	prometheus.MustRegister(metrics_statusCode)
	prometheus.MustRegister(metrics_methodToCode)
	prometheus.MustRegister(metrics_methodToSuccessOrFailure)
	prometheus.MustRegister(metrics_methodToNumProxied)
	prometheus.MustRegister(metrics_responseTimeHistogram)
}

var metrics_RpcRequestByMethod = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rpc_requests_by_method",
		Help: "RPC requests by method",
	},
	[]string{"method"},
)

var metrics_epochsAvailable = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "epoch_available",
		Help: "Epochs available",
	},
	[]string{"epoch"},
)

var metrics_statusCode = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "status_code",
		Help: "Status code",
	},
	[]string{"code"},
)

var metrics_methodToCode = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_code",
		Help: "Method to code",
	},
	[]string{"method", "code"},
)

var metrics_methodToSuccessOrFailure = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_success_or_failure",
		Help: "Method to success or failure",
	},
	[]string{"method", "status"},
)

var metrics_methodToNumProxied = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "method_to_num_proxied",
		Help: "Method to num proxied",
	},
	[]string{"method"},
)

var metrics_responseTimeHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "response_time_histogram",
		Help: "Response time histogram",
	},
	[]string{"method"},
)

// - Version information of this binary
var metrics_version = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "version",
		Help: "Version information of this binary",
	},
	[]string{"started_at", "tag", "commit", "compiler", "goarch", "goos", "goamd64", "vcs", "vcs_revision", "vcs_time", "vcs_modified"},
)

func init() {
	// Add an entry to the metric with the version information.
	labeledValues := map[string]string{
		"started_at":   StartedAt.Format(time.RFC3339),
		"tag":          GitTag,
		"commit":       GitCommit,
		"compiler":     "",
		"goarch":       "",
		"goos":         "",
		"goamd64":      "",
		"vcs":          "",
		"vcs_revision": "",
		"vcs_time":     "",
		"vcs_modified": "",
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if isAnyOf(setting.Key,
				"-compiler",
				"GOARCH",
				"GOOS",
				"GOAMD64",
				"vcs",
				"vcs.revision",
				"vcs.time",
				"vcs.modified",
			) {
				switch setting.Key {
				case "-compiler":
					labeledValues["compiler"] = setting.Value
				case "GOARCH":
					labeledValues["goarch"] = setting.Value
				case "GOOS":
					labeledValues["goos"] = setting.Value
				case "GOAMD64":
					labeledValues["goamd64"] = setting.Value
				case "vcs":
					labeledValues["vcs"] = setting.Value
				case "vcs.revision":
					labeledValues["vcs_revision"] = setting.Value
				case "vcs.time":
					labeledValues["vcs_time"] = setting.Value
				case "vcs.modified":
					labeledValues["vcs_modified"] = setting.Value
				}
			}
		}
	}
	metrics_version.With(labeledValues).Set(1)
}

var StartedAt = time.Now()

func GetUptime() time.Duration {
	return time.Since(StartedAt)
}
