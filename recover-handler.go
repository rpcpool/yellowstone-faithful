package main

import (
	"runtime/debug"

	"github.com/rpcpool/yellowstone-faithful/metrics"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

// recoverHandler turns a panic in h into a 500 response, so one bad request cannot stop the server.
func recoverHandler(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				metrics.RpcHandlerPanics.Inc()
				klog.Errorf("panic serving %s %q from %s: %v\n%s", ctx.Method(), ctx.Path(), ctx.RemoteAddr(), r, debug.Stack())
				ctx.Response.Reset()
				ctx.SetConnectionClose()
				ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
			}
		}()
		h(ctx)
	}
}
