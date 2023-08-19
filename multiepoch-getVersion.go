package main

import (
  "context"
  "fmt"

  "github.com/sourcegraph/jsonrpc2"
)

// @TODO make these values make sense
func (ser *MultiEpoch) handleGetVersion(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
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
          return nil, fmt.Errorf("failed to reply: %w", err)
  }
  return nil, nil
}
