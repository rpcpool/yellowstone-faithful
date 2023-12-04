package main

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

func replyJSON(ctx *fasthttp.RequestCtx, code int, v interface{}) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(code)

	if err := jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder(ctx).Encode(v); err != nil {
		klog.Errorf("failed to marshal response: %v", err)
	}
}
