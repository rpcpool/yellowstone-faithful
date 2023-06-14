package main

import (
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

var (
	defaultMaxIdleConnsPerHost = 20
	defaultTimeout             = 20 * time.Second
	defaultKeepAlive           = 180 * time.Second
)

func newHTTPTransport() *http.Transport {
	return &http.Transport{
		IdleConnTimeout:     time.Minute,
		MaxConnsPerHost:     defaultMaxIdleConnsPerHost,
		MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
		Proxy:               http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultKeepAlive,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2: true,
		// MaxIdleConns:          100,
		TLSHandshakeTimeout: 10 * time.Second,
		// ExpectContinueTimeout: 1 * time.Second,
	}
}

// newHTTPClient returns a new Client from the provided config.
// Client is safe for concurrent use by multiple goroutines.
func newHTTPClient() *http.Client {
	tr := newHTTPTransport()

	return &http.Client{
		Timeout:   defaultTimeout,
		Transport: gzhttp.Transport(tr),
	}
}
