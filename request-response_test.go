package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
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

func Test_parseGetBlockRequest_footer(t *testing.T) {
	tests := []struct {
		name       string
		params     string
		wantFooter bool
		wantErr    bool
	}{
		{name: "footer false", params: `[100, {"footer": false}]`, wantFooter: false},
		{name: "footer true", params: `[100, {"footer": true}]`, wantFooter: true},
		{name: "footer null defaults to true", params: `[100, {"footer": null}]`, wantFooter: true},
		{name: "footer absent defaults to true", params: `[100, {}]`, wantFooter: true},
		{name: "no options defaults to true", params: `[100]`, wantFooter: true},
		{name: "footer invalid type", params: `[100, {"footer": "yes"}]`, wantErr: true},
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
			require.NotNil(t, got.Options.Footer)
			assert.Equal(t, tt.wantFooter, *got.Options.Footer)
		})
	}
}

func Test_blockFooterUi(t *testing.T) {
	withMarkers := func(markers ...[]byte) *ipldbindcode.Block {
		list := ipldbindcode.List__Bytes(markers)
		ptr := &list
		block := &ipldbindcode.Block{}
		block.Meta.Block_markers = &ptr
		return block
	}
	// u16 version | u8 variant | u16 len | payload (footer_version | bank_hash | u64 time | u8 len + user agent | certs)
	marker := func(variant byte, payload []byte) []byte {
		b := append([]byte{1, 0, variant}, byte(len(payload)), byte(len(payload)>>8))
		return append(b, payload...)
	}
	userAgent := "agave/4.4.0-beta.0 (src:98535e50; feat:6e955f07, client:Agave)"
	footer := append([]byte{1}, make([]byte, 32)...)
	footer = binary.LittleEndian.AppendUint64(footer, 1791392218636594070)
	footer = append(append(append(footer, byte(len(userAgent))), userAgent...), 0, 0, 0)
	header := append([]byte{1}, make([]byte, 40)...)

	t.Run("alpenglow block", func(t *testing.T) {
		got, err := blockFooterUi(withMarkers(marker(1, header), marker(0, footer)))
		require.NoError(t, err)
		require.NotNil(t, got)
		raw, err := got.MarshalJSON()
		require.NoError(t, err)
		assert.JSONEq(t, `{"blockProducerTimeNanos":1791392218636594070,"blockUserAgent":"`+userAgent+`"}`, string(raw))
		assert.Contains(t, string(raw), `"blockProducerTimeNanos":1791392218636594070`, "u64 must keep every digit")
	})
	t.Run("block without markers", func(t *testing.T) {
		got, err := blockFooterUi(&ipldbindcode.Block{})
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("malformed footer", func(t *testing.T) {
		_, err := blockFooterUi(withMarkers(marker(0, []byte{1, 2, 3})))
		require.Error(t, err)
	})
}

func Test_parseGetBlockRequest_encoding(t *testing.T) {
	tests := []struct {
		name         string
		params       string
		wantEncoding solana.EncodingType
		wantErr      bool
	}{
		{
			name:         "encoding null defaults to json",
			params:       `[100, {"encoding": null}]`,
			wantEncoding: solana.EncodingJSON,
		},
		{
			name:         "encoding absent defaults to json",
			params:       `[100, {}]`,
			wantEncoding: solana.EncodingJSON,
		},
		{
			name:         "encoding explicit string",
			params:       `[100, {"encoding": "base64"}]`,
			wantEncoding: solana.EncodingBase64,
		},
		{
			name:    "encoding invalid type",
			params:  `[100, {"encoding": 123}]`,
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
			require.NotNil(t, got.Options.Encoding)
			assert.Equal(t, tt.wantEncoding, *got.Options.Encoding)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func Test_handleGetBlockCar_parseErrorIncludesRawParams(t *testing.T) {
	req := &jsonrpc2.Request{
		Method: "getBlock",
		Params: rawMessagePtr(`[100, {"encoding": 123}]`),
	}
	reqCtx := &fasthttp.RequestCtx{}
	reqCtx.Request.Header.SetUserAgent("solana-web3.js/1.95.0")

	_, err := (&MultiEpoch{}).handleGetBlock_car(
		context.Background(),
		&requestContext{ctx: reqCtx},
		req,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `encoding must be a string, got float64`)
	assert.Contains(t, err.Error(), `raw_params=[100,{"encoding":123}]`)
	assert.Contains(t, err.Error(), `user_agent="solana-web3.js/1.95.0"`)
}

func rawMessagePtr(s string) *json.RawMessage {
	raw := json.RawMessage(s)
	return &raw
}
