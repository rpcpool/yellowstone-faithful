package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

func newRPCHandler_fast(handler *deprecatedRPCServer) func(ctx *fasthttp.RequestCtx) {
	return func(c *fasthttp.RequestCtx) {
		startedAt := time.Now()
		defer func() {
			klog.Infof("request took %s", time.Since(startedAt))
		}()
		{
			// make sure the method is POST
			if !c.IsPost() {
				replyJSON(c, http.StatusMethodNotAllowed, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeMethodNotFound,
						Message: "Method not allowed",
					},
				})
				return
			}

			// limit request body size
			if c.Request.Header.ContentLength() > 1024 {
				replyJSON(c, http.StatusRequestEntityTooLarge, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInvalidRequest,
						Message: "Request entity too large",
					},
				})
				return
			}
		}
		// read request body
		body := c.Request.Body()

		// parse request
		var rpcRequest jsonrpc2.Request
		if err := json.Unmarshal(body, &rpcRequest); err != nil {
			klog.Errorf("failed to unmarshal request: %v", err)
			replyJSON(c, http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}

		klog.Infof("Received request: %q", strings.TrimSpace(string(body)))

		rqCtx := &requestContext{ctx: c}

		handler.Handle(c, rqCtx, &rpcRequest)
	}
}

func replyJSON(ctx *fasthttp.RequestCtx, code int, v interface{}) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(code)

	if err := jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder(ctx).Encode(v); err != nil {
		klog.Errorf("failed to marshal response: %v", err)
	}
}
