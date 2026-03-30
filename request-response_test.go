package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
