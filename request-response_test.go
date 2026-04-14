package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_parseGetBlockRequest_rewards(t *testing.T) {
	tests := []struct {
		name        string
		params      string
		wantRewards *bool
		wantErr     bool
	}{
		{
			name:        "rewards true",
			params:      `[100, {"rewards": true}]`,
			wantRewards: boolPtr(true),
		},
		{
			name:        "rewards false",
			params:      `[100, {"rewards": false}]`,
			wantRewards: boolPtr(false),
		},
		{
			name:        "rewards null defaults to true",
			params:      `[100, {"rewards": null}]`,
			wantRewards: boolPtr(true),
		},
		{
			name:        "rewards absent defaults to true",
			params:      `[100, {}]`,
			wantRewards: boolPtr(true),
		},
		{
			name:    "rewards invalid type",
			params:  `[100, {"rewards": "yes"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(tt.params)
			got, err := parseGetBlockRequest(&raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantRewards == nil {
				assert.Nil(t, got.Options.Rewards)
			} else {
				require.NotNil(t, got.Options.Rewards)
				assert.Equal(t, *tt.wantRewards, *got.Options.Rewards)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func Test_handleGetBlockCar_parseErrorIncludesRawParams(t *testing.T) {
	req := &jsonrpc2.Request{
		Method: "getBlock",
		Params: rawMessagePtr(`[100, {"encoding": null}]`),
	}
	reqCtx := &fasthttp.RequestCtx{}
	reqCtx.Request.Header.SetUserAgent("solana-web3.js/1.95.0")

	_, err := (&MultiEpoch{}).handleGetBlock_car(
		context.Background(),
		&requestContext{ctx: reqCtx},
		req,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `encoding must be a string, got <nil>`)
	assert.Contains(t, err.Error(), `raw_params=[100,{"encoding":null}]`)
	assert.Contains(t, err.Error(), `user_agent="solana-web3.js/1.95.0"`)
}

func rawMessagePtr(s string) *json.RawMessage {
	raw := json.RawMessage(s)
	return &raw
}
