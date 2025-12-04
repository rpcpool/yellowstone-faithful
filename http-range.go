package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/metrics"
	"k8s.io/klog/v2"
)

type readCloserWrapperForStats struct {
	rac        carreader.ReaderAtCloser
	isRemote   bool
	isSplitCar bool
	name       string
	size       int64
}

func (r *readCloserWrapperForStats) Size() int64 {
	return r.size
}

// when reading print a dot
func (r *readCloserWrapperForStats) ReadAt(p []byte, off int64) (n int, err error) {
	startedAt := time.Now()
	defer func() {
		took := time.Since(startedAt)
		var isIndex, isCar bool

		// Metrics want to always report
		// if has suffix .index, then it's an index file
		if strings.HasSuffix(r.name, ".index") {
			isIndex = true
		}
		// if has suffix .car, then it's a car file
		if strings.HasSuffix(r.name, ".car") || r.isSplitCar {
			isCar = true
		}

		if isCar {
			carName := filepath.Base(r.name)
			metrics.CarLookupHistogram.WithLabelValues(
				carName,
				boolToString(r.isRemote),
				boolToString(r.isSplitCar),
			).Observe(float64(took.Seconds()))
		}
		if isIndex {
			// get the index name, which is the part before the .index suffix, after the last .
			indexName := strings.TrimSuffix(r.name, ".index")
			// split the index name by . and get the last part
			byDot := strings.Split(indexName, ".")
			if len(byDot) > 0 {
				indexName = byDot[len(byDot)-1]
			}
			metrics.IndexLookupHistogram.WithLabelValues(
				indexName,
				boolToString(r.isRemote),
				boolToString(r.isSplitCar),
			).Observe(float64(took.Seconds()))
		}

		if klog.V(5).Enabled() {
			// Very verbose logging:
			var icon string
			if r.isRemote {
				// add internet icon
				icon = "🌐 "
			} else {
				// add disk icon
				icon = "💾 "
			}

			prefix := icon + "[READ-UNKNOWN]"
			if isIndex {
				prefix = icon + azureBG("[READ-INDEX]")
			}
			// if has suffix .car, then it's a car file
			if isCar {
				if r.isSplitCar {
					prefix = icon + azureBG("[READ-SPLIT-CAR]")
				} else {
					prefix = icon + purpleBG("[READ-CAR]")
				}
			}

			klog.V(5).Infof(prefix+" %s:%d+%d (%s)\n", (r.name), off, len(p), took)
		}
	}()
	return r.rac.ReadAt(p, off)
}

func purpleBG(s string) string {
	// blue bg, black fg
	return "\033[48;5;4m\033[38;5;0m" + s + "\033[0m"
}

func azureBG(s string) string {
	// azure bg, black fg
	return "\033[48;5;6m\033[38;5;0m" + s + "\033[0m"
}

// when closing print a newline
func (r *readCloserWrapperForStats) Close() error {
	return r.rac.Close()
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
