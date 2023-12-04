package main

import (
	"fmt"
)

type RpcServerOptions struct {
	ListenOn           string
	GsfaOnlySignatures bool
}

func getCidCacheKey(off int64, p []byte) string {
	return fmt.Sprintf("%d-%d", off, len(p))
}
