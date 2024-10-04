package main

import (
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/rpcpool/yellowstone-faithful/metrics"
	"k8s.io/klog/v2"
)

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
}

type readCloserWrapper struct {
	rac        ReaderAtCloser
	isRemote   bool
	isSplitCar bool
	name       string
	size       int64
}

func (r *readCloserWrapper) Size() int64 {
	return r.size
}

// when reading print a dot
func (r *readCloserWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	startedAt := time.Now()
	defer func() {
		took := time.Since(startedAt)
		var isIndex, isCar bool

		// Metrics want to always report
		// if has suffix .index, then it's an index file
		if strings.HasSuffix(r.name, ".index") {
			isIndex = true
			// get the index name, which is the part before the .index suffix, after the last .
			indexName := strings.TrimSuffix(r.name, ".index")
			// split the index name by . and get the last part
			byDot := strings.Split(indexName, ".")
			if len(byDot) > 0 {
				indexName = byDot[len(byDot)-1]
			}
			// TODO: distinguish between remote and local index reads
			metrics.IndexLookupHistogram.WithLabelValues(indexName).Observe(float64(took.Seconds()))
		}
		// if has suffix .car, then it's a car file
		if strings.HasSuffix(r.name, ".car") || r.isSplitCar {
			isCar = true
			carName := filepath.Base(r.name)
			// TODO: distinguish between remote and local index reads
			metrics.CarLookupHistogram.WithLabelValues(carName).Observe(float64(took.Seconds()))
		}

		if klog.V(5).Enabled() {
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
				// get the index name, which is the part before the .index suffix, after the last .
				indexName := strings.TrimSuffix(r.name, ".index")
				// split the index name by . and get the last part
				byDot := strings.Split(indexName, ".")
				if len(byDot) > 0 {
					indexName = byDot[len(byDot)-1]
				}
				// TODO: distinguish between remote and local index reads
				metrics.IndexLookupHistogram.WithLabelValues(indexName).Observe(float64(took.Seconds()))
			}
			// if has suffix .car, then it's a car file
			if isCar {
				if r.isSplitCar {
					prefix = icon + azureBG("[READ-SPLIT-CAR]")
				} else {
					prefix = icon + purpleBG("[READ-CAR]")
				}
				carName := filepath.Base(r.name)
				// TODO: distinguish between remote and local index reads
				metrics.CarLookupHistogram.WithLabelValues(carName).Observe(float64(took.Seconds()))
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
func (r *readCloserWrapper) Close() error {
	return r.rac.Close()
}
