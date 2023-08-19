package main

import (
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

func (ser *MultiEpoch) tryEnrichGetVersion(body []byte) ([]byte, error) {
	var decodedRemote jsonrpc2.Response
	if err := json.Unmarshal(body, &decodedRemote); err != nil {
		return nil, err
	}
	if decodedRemote.Error != nil || decodedRemote.Result == nil {
		return nil, fmt.Errorf("response is not a success response")
	}
	// node decode the result:
	var decodedResult map[string]any
	if err := json.Unmarshal(*decodedRemote.Result, &decodedResult); err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}
	// enrich the result:
	faithfulVersion := ser.GetFaithfulVersionInfo()
	decodedResult["faithful"] = faithfulVersion

	// re-encode the result:
	encodedResult, err := json.Marshal(decodedResult)
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode result: %w", err)
	}
	// re-encode the response:
	decodedRemote.Result = (*json.RawMessage)(&encodedResult)
	encodedResponse, err := json.Marshal(decodedRemote)
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode response: %w", err)
	}
	// return the response:
	return encodedResponse, nil
}

func (ser *MultiEpoch) GetFaithfulVersionInfo() map[string]any {
	faithfulVersion := make(map[string]any)
	faithfulVersion["version"] = GitTag
	faithfulVersion["commit"] = GitCommit
	faithfulVersion["epochs"] = ser.GetEpochNumbers()
	return faithfulVersion
}
