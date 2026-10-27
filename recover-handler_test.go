package main

import (
	"net"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func serveInMemory(t *testing.T, h fasthttp.RequestHandler) *fasthttp.Client {
	t.Helper()
	ln := fasthttputil.NewInmemoryListener()
	s := &fasthttp.Server{Handler: h}
	go s.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { s.Shutdown() })
	return &fasthttp.Client{Dial: func(string) (net.Conn, error) { return ln.Dial() }}
}

func doRequest(t *testing.T, c *fasthttp.Client, method, path, encoding string, body []byte) int {
	t.Helper()
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://faithful" + path)
	req.Header.SetMethod(method)
	if encoding != "" {
		req.Header.Set(fasthttp.HeaderContentEncoding, encoding)
	}
	req.SetBody(body)
	if err := c.Do(req, resp); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp.StatusCode()
}

func TestRecoverHandler(t *testing.T) {
	c := serveInMemory(t, recoverHandler(func(ctx *fasthttp.RequestCtx) {
		if string(ctx.Path()) == "/panic" {
			var b []byte
			_ = b[1]
		}
		ctx.SetStatusCode(fasthttp.StatusOK)
	}))
	if got := doRequest(t, c, fasthttp.MethodGet, "/panic", "", nil); got != fasthttp.StatusInternalServerError {
		t.Fatalf("panicking request: got status %d, want 500", got)
	}
	if got := doRequest(t, c, fasthttp.MethodGet, "/ok", "", nil); got != fasthttp.StatusOK {
		t.Fatalf("request after panic: got status %d, want 200", got)
	}
}

func TestRPCServerSurvivesMalformedBrotli(t *testing.T) {
	c := serveInMemory(t, newRPCServerHandler(&MultiEpoch{}, nil))
	if got := doRequest(t, c, fasthttp.MethodPost, "/", "br", []byte{0xff, 0xff, 0xff, 0xff, 0x00, 0x01, 0x02, 0x03}); got != fasthttp.StatusInternalServerError {
		t.Fatalf("malformed brotli body: got status %d, want 500", got)
	}
	if got := doRequest(t, c, fasthttp.MethodGet, "/health", "", nil); got != fasthttp.StatusOK {
		t.Fatalf("health after malformed brotli: got status %d, want 200", got)
	}
}
