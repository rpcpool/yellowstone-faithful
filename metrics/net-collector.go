package metrics

import (
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	psnet "github.com/shirou/gopsutil/v3/net" // Renamed gopsutil/net to psnet to avoid conflict
)

// netCollector implements the prometheus.Collector interface.
type netCollector struct {
	// Mutex to protect lastStats during concurrent scrapes
	mutex sync.Mutex
	// lastStats stores the previous stats map[interfaceName]net_lastStat
	lastStats map[string]net_lastStat
	// interfaces contains the whitelist of interface names to monitor.
	// If empty, all interfaces are monitored.
	interfaces map[string]struct{}

	// Descriptors for the Prometheus metrics
	recvBytesTotalDesc *prometheus.Desc
	sentBytesTotalDesc *prometheus.Desc
	recvRateDesc       *prometheus.Desc
	sentRateDesc       *prometheus.Desc

	// errorDesc is used to report an error as a metric
	errorDesc *prometheus.Desc
}

// net_lastStat holds the values from the previous scrape for rate calculation.
type net_lastStat struct {
	recvBytes uint64
	sentBytes uint64
	time      time.Time
}

// NewNetCollector creates and returns a new netCollector.
// interfaces is a list of interface names to monitor (e.g., "eth0", "lo").
// If the list is empty or nil, all interfaces will be monitored.
func NewNetCollector(interfaces []string) *netCollector {
	// Convert slice to map for efficient lookups
	interfaceMap := make(map[string]struct{})
	if len(interfaces) > 0 {
		for _, iface := range interfaces {
			interfaceMap[iface] = struct{}{}
		}
	}

	// Define the Prometheus metric descriptors
	return &netCollector{
		lastStats:  make(map[string]net_lastStat),
		interfaces: interfaceMap, // Store the map
		recvBytesTotalDesc: prometheus.NewDesc("net_receive_bytes_total",
			"Total number of bytes received from this interface.",
			[]string{"interface"}, // Label: interface (e.g., "eth0", "lo")
			nil,
		),
		sentBytesTotalDesc: prometheus.NewDesc("net_send_bytes_total",
			"Total number of bytes sent from this interface.",
			[]string{"interface"},
			nil,
		),
		recvRateDesc: prometheus.NewDesc("net_receive_rate_bytes_per_second",
			"The current rate of bytes received per second from this interface.",
			[]string{"interface"},
			nil,
		),
		sentRateDesc: prometheus.NewDesc("net_send_rate_bytes_per_second",
			"The current rate of bytes sent per second from this interface.",
			[]string{"interface"},
			nil,
		),
		// New descriptor for reporting errors
		errorDesc: prometheus.NewDesc("net_collector_error",
			"Indicates an error occurred during net stats collection.",
			nil, // No labels
			nil,
		),
	}
}

// Describe implements the prometheus.Collector interface.
// It sends the descriptors of all metrics collected by this collector
// to the provided channel.
func (c *netCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.recvBytesTotalDesc
	ch <- c.sentBytesTotalDesc
	ch <- c.recvRateDesc
	ch <- c.sentRateDesc
	ch <- c.errorDesc // Add the error descriptor
}

// Collect implements the prometheus.Collector interface.
// It fetches the current net I/O stats and sends them as
// Prometheus metrics to the provided channel.
func (c *netCollector) Collect(ch chan<- prometheus.Metric) {
	// Lock to ensure thread-safety
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get stats per interface
	ioStats, err := psnet.IOCounters(true) // Use aliased package name 'psnet'
	if err != nil {
		log.Printf("Error getting net IO counters: %v", err)
		// Report the error as an InvalidMetric
		ch <- prometheus.NewInvalidMetric(c.errorDesc, err)
		return
	}

	// Add a log to check if the returned map is empty
	if len(ioStats) == 0 {
		log.Println("Warning: net.IOCounters() returned no data. Check permissions or interface filters.")
		return
	}

	now := time.Now()

	// Iterate over stats for each interface
	for _, stats := range ioStats {
		interfaceName := stats.Name

		// If an interface whitelist is provided, skip interfaces not in the list.
		if len(c.interfaces) > 0 {
			if _, ok := c.interfaces[interfaceName]; !ok {
				continue // Not in the whitelist, skip
			}
		}

		// Log for debugging
		// log.Printf("Processing metrics for interface: %s", interfaceName)

		// 1. Report Total Bytes (Counter)
		ch <- prometheus.MustNewConstMetric(
			c.recvBytesTotalDesc,
			prometheus.CounterValue,
			float64(stats.BytesRecv), // Total bytes received
			interfaceName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.sentBytesTotalDesc,
			prometheus.CounterValue,
			float64(stats.BytesSent), // Total bytes sent
			interfaceName,
		)

		// 2. Calculate and Report Rate (Gauge)
		// Check if we have previous stats to calculate a rate
		if last, ok := c.lastStats[interfaceName]; ok {
			// Calculate time delta in seconds
			duration := now.Sub(last.time).Seconds()

			// Ensure duration is positive to avoid division by zero
			if duration > 0 {
				// Calculate delta in bytes
				recvDelta := float64(stats.BytesRecv - last.recvBytes)
				sentDelta := float64(stats.BytesSent - last.sentBytes)

				// Calculate rate (bytes per second)
				recvRate := recvDelta / duration
				sentRate := sentDelta / duration

				// Handle counter resets/wraps (where delta < 0)
				if recvRate < 0 {
					recvRate = 0
				}
				if sentRate < 0 {
					sentRate = 0
				}

				// Report the calculated rates as Gauges
				ch <- prometheus.MustNewConstMetric(
					c.recvRateDesc,
					prometheus.GaugeValue,
					recvRate, // Bytes received per second
					interfaceName,
				)
				ch <- prometheus.MustNewConstMetric(
					c.sentRateDesc,
					prometheus.GaugeValue,
					sentRate, // Bytes sent per second
					interfaceName,
				)
			}
		}

		// Store the current stats for the next scrape
		c.lastStats[interfaceName] = net_lastStat{
			recvBytes: stats.BytesRecv,
			sentBytes: stats.BytesSent,
			time:      now,
		}
	}
}
