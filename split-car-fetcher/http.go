package splitcarfetcher

import (
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

var (
	DefaultMaxIdleConnsPerHost = 100
	DefaultTimeout             = 1000 * time.Second
	DefaultKeepAlive           = 180 * time.Second
)

func NewHTTPTransport() *http.Transport {
	return &http.Transport{
		IdleConnTimeout:     time.Minute,
		MaxConnsPerHost:     DefaultMaxIdleConnsPerHost,
		MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
		MaxIdleConns:        DefaultMaxIdleConnsPerHost,
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
