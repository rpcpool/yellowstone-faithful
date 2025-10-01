package splitcarfetcher

import (
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

var (
	DefaultMaxConnsPerHost     = 30_000
	DefaultMaxIdleConnsPerHost = 10_000
	DefaultTimeout             = 1000 * time.Second
	DefaultKeepAlive           = 180 * time.Second
)

func NewHTTPTransport() *http.Transport {
	return &http.Transport{
		IdleConnTimeout:     time.Minute,
		MaxConnsPerHost:     DefaultMaxConnsPerHost,
		MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
		MaxIdleConns:        0,
		Proxy:               http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   DefaultTimeout,
			KeepAlive: DefaultKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2: true,
		// MaxIdleConns:          100,
		TLSHandshakeTimeout: 10 * time.Second,
		// ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewHTTPClient returns a new Client from the provided config.
// Client is safe for concurrent use by multiple goroutines.
func NewHTTPClient() *http.Client {
	tr := NewHTTPTransport()

	return &http.Client{
		Timeout:   DefaultTimeout,
		Transport: gzhttp.Transport(tr),
	}
}
