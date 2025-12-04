package metrics

import (
	"fmt"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/disk"
)

// GetDeviceForDirectory finds the block device name (e.g., "sda1") for a given directory path.
// It works by finding the mount point that contains the directory.
func GetDeviceForDirectory(dir string) (string, error) {
	// Get absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
	}

	partitions, err := disk.Partitions(false) // Get all partitions
	if err != nil {
		return "", fmt.Errorf("failed to get partitions: %w", err)
	}

	bestMatch := ""
	var bestPartition disk.PartitionStat

	// Find the partition with the longest mount point that is a prefix of the dir
	for _, p := range partitions {
		if strings.HasPrefix(absDir, p.Mountpoint) {
			if len(p.Mountpoint) > len(bestMatch) {
				bestMatch = p.Mountpoint
				bestPartition = p
			}
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no mount point found for directory %s", absDir)
	}

	// disk.IOCounters uses the base device name (e.g., "sda1" not "/dev/sda1")
	deviceName := filepath.Base(bestPartition.Device)
	return deviceName, nil
}

// diskCollector implements the prometheus.Collector interface.
type diskCollector struct {
	// Mutex to protect lastStats and devices during concurrent access
	mutex sync.Mutex
	// lastStats stores the previous stats map[deviceName]lastStat
	lastStats map[string]lastStat
	// devices contains the whitelist of device names to monitor.
	// If empty, all devices are monitored.
	devices map[string]struct{}

	// Descriptors for the Prometheus metrics
	readBytesTotalDesc  *prometheus.Desc
	writeBytesTotalDesc *prometheus.Desc
	readRateDesc        *prometheus.Desc
	writeRateDesc       *prometheus.Desc

	// errorDesc is used to report an error as a metric
	errorDesc *prometheus.Desc
}

// lastStat holds the values from the previous scrape for rate calculation.
type lastStat struct {
	readBytes  uint64
	writeBytes uint64
	time       time.Time
}

// NewDiskCollector creates and returns a new diskCollector.
// devices is a list of device names to monitor (e.g., "sda", "nvme0n1").
// If the list is empty or nil, all devices will be monitored.
func NewDiskCollector(devices []string) *diskCollector {
	// Convert slice to map for efficient lookups
	deviceMap := make(map[string]struct{})
	if len(devices) > 0 {
		for _, device := range devices {
			deviceMap[device] = struct{}{}
		}
	}

	// Define the Prometheus metric descriptors
	return &diskCollector{
		lastStats: make(map[string]lastStat),
		devices:   deviceMap, // Store the map
		readBytesTotalDesc: prometheus.NewDesc("disk_read_bytes_total",
			"Total number of bytes read from this disk.",
			[]string{"device"}, // Label: device (e.g., "sda", "nvme0n1")
			nil,
		),
		writeBytesTotalDesc: prometheus.NewDesc("disk_write_bytes_total",
			"Total number of bytes written to this disk.",
			[]string{"device"},
			nil,
		),
		readRateDesc: prometheus.NewDesc("disk_read_rate_bytes_per_second",
			"The current rate of bytes read per second from this disk.",
			[]string{"device"},
			nil,
		),
		writeRateDesc: prometheus.NewDesc("disk_write_rate_bytes_per_second",
			"The current rate of bytes written per second to this disk.",
			[]string{"device"},
			nil,
		),
		// New descriptor for reporting errors
		errorDesc: prometheus.NewDesc("disk_collector_error",
			"Indicates an error occurred during disk stats collection.",
			nil, // No labels
			nil,
		),
	}
}

// AddDevice adds a new device to the monitor whitelist.
// If the whitelist was previously empty (monitoring all devices),
// calling this will switch the collector to monitor ONLY the whitelisted devices.
func (c *diskCollector) AddDevice(device string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.devices[device] = struct{}{}
}

// HasDevice checks if a specific device is currently in the whitelist.
func (c *diskCollector) HasDevice(device string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, exists := c.devices[device]
	return exists
}

// Describe implements the prometheus.Collector interface.
// It sends the descriptors of all metrics collected by this collector
// to the provided channel.
func (c *diskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.readBytesTotalDesc
	ch <- c.writeBytesTotalDesc
	ch <- c.readRateDesc
	ch <- c.writeRateDesc
	ch <- c.errorDesc // Add the error descriptor
}

// Collect implements the prometheus.Collector interface.
// It fetches the current disk I/O stats and sends them as
// Prometheus metrics to the provided channel.
func (c *diskCollector) Collect(ch chan<- prometheus.Metric) {
	// Lock to ensure thread-safety
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get stats for all physical disks (perdisk=true)
	ioStats, err := disk.IOCounters()
	if err != nil {
		log.Printf("Error getting disk IO counters: %v", err)
		// Report the error as an InvalidMetric
		ch <- prometheus.NewInvalidMetric(c.errorDesc, err)
		return
	}

	// Add a log to check if the returned map is empty
	if len(ioStats) == 0 {
		log.Println("Warning: disk.IOCounters() returned no data. Check permissions or disk filters.")
		return
	}

	now := time.Now()

	// Iterate over stats for each device
	for deviceName, stats := range ioStats {
		// If a device whitelist is provided, skip devices not in the list.
		if len(c.devices) > 0 {
			if _, ok := c.devices[deviceName]; !ok {
				continue // Not in the whitelist, skip
			}
		}

		// Add a log to see which device is being processed
		slog.Debug("Processing disk device", "device", deviceName)

		// 1. Report Total Bytes (Counter)
		// This value is a continuously increasing counter from the OS.
		ch <- prometheus.MustNewConstMetric(
			c.readBytesTotalDesc,
			prometheus.CounterValue,
			float64(stats.ReadBytes), // Total bytes read
			deviceName,
		)
		ch <- prometheus.MustNewConstMetric(
			c.writeBytesTotalDesc,
			prometheus.CounterValue,
			float64(stats.WriteBytes), // Total bytes written
			deviceName,
		)

		// 2. Calculate and Report Rate (Gauge)
		// Check if we have previous stats to calculate a rate
		if last, ok := c.lastStats[deviceName]; ok {
			// Calculate time delta in seconds
			duration := now.Sub(last.time).Seconds()

			// Ensure duration is positive to avoid division by zero
			if duration > 0 {
				// Calculate delta in bytes
				readDelta := float64(stats.ReadBytes - last.readBytes)
				writeDelta := float64(stats.WriteBytes - last.writeBytes)

				// Calculate rate (bytes per second)
				readRate := readDelta / duration
				writeRate := writeDelta / duration

				// Handle counter resets/wraps (where delta < 0)
				// If the new value is less than the old one, the counter
				// likely reset. Report 0 for this interval.
				if readRate < 0 {
					readRate = 0
				}
				if writeRate < 0 {
					writeRate = 0
				}

				// Report the calculated rates as Gauges
				ch <- prometheus.MustNewConstMetric(
					c.readRateDesc,
					prometheus.GaugeValue,
					readRate, // Bytes read per second
					deviceName,
				)
				ch <- prometheus.MustNewConstMetric(
					c.writeRateDesc,
					prometheus.GaugeValue,
					writeRate, // Bytes written per second
					deviceName,
				)
			}
		}

		// Store the current stats for the next scrape
		c.lastStats[deviceName] = lastStat{
			readBytes:  stats.ReadBytes,
			writeBytes: stats.WriteBytes,
			time:       now,
		}
	}
}
