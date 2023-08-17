package main

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
	"k8s.io/klog/v2"
)

func (ser *rpcServer) handleGetVersion(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) {
	// taken from solana mainnet version reponse
	var versionResponse GetVersionResponse
	versionResponse.FeatureSet = 1879391783
	versionResponse.SolanaCore = "1.14.23"
	var err = conn.Reply(
		ctx,
		req.ID,
		versionResponse,
		func(m map[string]any) map[string]any {
			r := map[string]any{}
			r["feature-set"] = m["featureSet"]
			r["solana-core"] = m["solanaCore"]
			return r
		},
	)
	if err != nil {
		klog.Errorf("failed to reply: %v", err)
	}
}
