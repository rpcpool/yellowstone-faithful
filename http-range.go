package main

import (
	"io"
	"strings"
	"time"

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
			// if has suffix .index, then it's an index file
			if strings.HasSuffix(r.name, ".index") {
				prefix = icon + azureBG("[READ-INDEX]")
			}
			// if has suffix .car, then it's a car file
			if strings.HasSuffix(r.name, ".car") || r.isSplitCar {
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
func (r *readCloserWrapper) Close() error {
	return r.rac.Close()
}
