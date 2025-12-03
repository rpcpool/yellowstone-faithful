package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"time"

	jd "github.com/josephburnett/jd/v2"
)

// Config holds the command line arguments
type Config struct {
	RefRPC        string
	TargetRPC     string
	SlotsPerEpoch int
	MaxTxsToCheck int
	Verbose       bool
	SlotsInEpoch  int64 // Standard Solana slots per epoch
	StopOnDiff    bool
}

// EpochResponse matches the structure {"epochs":[...]}
type EpochResponse struct {
	Epochs []uint64 `json:"epochs"`
}

// JSONRPCRequest is a standard JSON-RPC 2.0 request wrapper
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// JSONRPCResponse is a generic response wrapper
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// BlockShort structure to extract signatures easily while keeping the rest raw for comparison
type BlockShort struct {
	Transactions []struct {
		Transaction interface{} `json:"transaction"` // Could be []string or object depending on encoding
	} `json:"transactions"`
	Signatures []string `json:"signatures"` // Sometimes present depending on encoding
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.RefRPC, "ref-rpc", "https://api.mainnet-beta.solana.com", "Reference Solana RPC endpoint")
	flag.StringVar(&cfg.TargetRPC, "target-rpc", "http://faithful-staging1:8888", "Target Old-Faithful RPC endpoint to test")
	flag.IntVar(&cfg.SlotsPerEpoch, "slots-per-epoch", 5, "Number of random slots to sample per epoch")
	flag.IntVar(&cfg.MaxTxsToCheck, "max-txs", 5, "Max transactions to verify per block (randomly selected) to avoid rate limits")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose logging")
	flag.Int64Var(&cfg.SlotsInEpoch, "epoch-len", 432000, "Length of an epoch in slots (default 432000)")
	flag.BoolVar(&cfg.StopOnDiff, "stop-on-diff", false, "Exit immediately when a discrepancy is found")
	flag.Parse()

	// crypto/rand does not require seeding

	log.Printf("Starting verification...")
	log.Printf("Target: %s", cfg.TargetRPC)
	log.Printf("Reference: %s", cfg.RefRPC)

	// 1. Fetch Epochs
	epochs, err := fetchEpochs(cfg.TargetRPC)
	if err != nil {
		log.Fatalf("Failed to fetch epochs from target: %v", err)
	}
	log.Printf("Found %d epochs to check: %v", len(epochs), epochs)

	client := &http.Client{Timeout: 30 * time.Second}

	// 2. Iterate Epochs
	for _, epoch := range epochs {
		log.Printf("--- Processing Epoch %d ---", epoch)

		// Calculate slot range for this epoch
		startSlot := epoch * uint64(cfg.SlotsInEpoch)
		endSlot := startSlot + uint64(cfg.SlotsInEpoch) - 1

		// Pick random slots
		slots := generateRandomSlots(startSlot, endSlot, cfg.SlotsPerEpoch)

		for _, slot := range slots {
			processSlot(client, cfg, slot)
		}
	}
}

func fetchEpochs(baseURL string) ([]uint64, error) {
	url := fmt.Sprintf("%s/api/v1/epochs", baseURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var epochResp EpochResponse
	if err := json.NewDecoder(resp.Body).Decode(&epochResp); err != nil {
		return nil, err
	}
	return epochResp.Epochs, nil
}

func generateRandomSlots(min, max uint64, count int) []uint64 {
	slots := make([]uint64, count)
	rangeSz := new(big.Int).SetUint64(max - min)

	if rangeSz.Sign() <= 0 {
		return []uint64{min}
	}

	for i := 0; i < count; i++ {
		offset, err := rand.Int(rand.Reader, rangeSz)
		if err != nil {
			// Fallback or panic in case of crypto/rand failure, though unlikely
			log.Printf("Failed to generate secure random number: %v", err)
			slots[i] = min
			continue
		}
		slots[i] = min + offset.Uint64()
	}
	return slots
}

// securePerm generates a random permutation of integers from 0 to n-1
func securePerm(n int) []int {
	m := make([]int, n)
	for i := 0; i < n; i++ {
		m[i] = i
	}
	for i := n - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(jBig.Int64())
		m[i], m[j] = m[j], m[i]
	}
	return m
}

func processSlot(client *http.Client, cfg Config, slot uint64) {
	log.Printf("Checking Slot %d...", slot)

	// Params for getBlock: encoding json, maxSupportedTransactionVersion 0 (to handle versioned txs)
	params := []interface{}{
		slot,
		map[string]interface{}{
			"encoding":                       "json",
			"transactionDetails":             "full",
			"rewards":                        false, // disabled to reduce noise/payload size
			"maxSupportedTransactionVersion": 0,
		},
	}

	// 3. Fetch Block from both
	refBlock, refLat, refErr := callRPC(client, cfg.RefRPC, "getBlock", params)
	targetBlock, targetLat, targetErr := callRPC(client, cfg.TargetRPC, "getBlock", params)

	logLatency("Slot "+fmt.Sprint(slot), refLat, targetLat)

	// Handle availability issues
	if refErr != nil || targetErr != nil {
		// If both failed, likely skipped slot
		if refErr != nil && targetErr != nil {
			if cfg.Verbose {
				log.Printf("Slot %d: skipped/missing on both.", slot)
			}
			return
		}
		log.Printf("Slot %d: FETCH ERROR mismatch. RefErr: %q, TargetErr: %q", slot, refErr, targetErr)
		return
	}

	// 4. Compare Block Data
	var refData, targetData interface{}
	json.Unmarshal(refBlock, &refData)
	json.Unmarshal(targetBlock, &targetData)

	if !reflect.DeepEqual(refData, targetData) {
		compareJSON(refBlock, targetBlock, fmt.Sprintf("Block %d", slot), cfg.StopOnDiff)
	} else {
		if cfg.Verbose {
			log.Printf("Slot %d: Blocks match.", slot)
		}
	}

	// Extract signatures for transaction checking
	// We need to decode specifically to get signatures
	var blockStruct struct {
		Transactions []struct {
			Transaction struct {
				Signatures []string `json:"signatures"`
			} `json:"transaction"`
		} `json:"transactions"`
	}

	// We use the Target block as the source of truth for which signatures to check
	if err := json.Unmarshal(targetBlock, &blockStruct); err != nil {
		log.Printf("Failed to parse block structure for slot %d: %v", slot, err)
		return
	}

	sigsToCheck := []string{}
	txCount := len(blockStruct.Transactions)

	if txCount == 0 {
		return
	}

	// Random sampling of transactions using secure permutation
	perm := securePerm(txCount)
	limit := cfg.MaxTxsToCheck
	if limit > txCount {
		limit = txCount
	}

	for i := 0; i < limit; i++ {
		tx := blockStruct.Transactions[perm[i]]
		if len(tx.Transaction.Signatures) > 0 {
			sigsToCheck = append(sigsToCheck, tx.Transaction.Signatures[0])
		}
	}

	// 5. Check Transactions
	for _, sig := range sigsToCheck {
		compareTransaction(client, cfg, sig)
	}
}

func compareTransaction(client *http.Client, cfg Config, signature string) {
	params := []interface{}{
		signature,
		map[string]interface{}{
			"encoding":                       "json",
			"maxSupportedTransactionVersion": 0,
		},
	}

	refTx, refLat, refErr := callRPC(client, cfg.RefRPC, "getTransaction", params)
	targetTx, targetLat, targetErr := callRPC(client, cfg.TargetRPC, "getTransaction", params)

	logLatency("Tx "+signature, refLat, targetLat)

	if refErr != nil && targetErr != nil {
		return // Both failed
	}
	if refErr != nil || targetErr != nil {
		log.Printf(" [!] TX FETCH mismatch for %s. RefErr: %q, TargetErr: %q", signature, refErr, targetErr)
		return
	}

	var refData, targetData interface{}
	json.Unmarshal(refTx, &refData)
	json.Unmarshal(targetTx, &targetData)

	if !reflect.DeepEqual(refData, targetData) {
		compareJSON(refTx, targetTx, signature, cfg.StopOnDiff)
	} else {
		if cfg.Verbose {
			log.Printf("Tx %s matches.", signature)
		}
	}
}

func callRPC(client *http.Client, url, method string, params []interface{}) (json.RawMessage, time.Duration, error) {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payload))
	latency := time.Since(start)

	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, latency, err
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, latency, err
	}

	if rpcResp.Error != nil {
		return nil, latency, fmt.Errorf("RPC Error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Check if result is null (e.g. block not found)
	if string(rpcResp.Result) == "null" {
		return nil, latency, fmt.Errorf("result is null")
	}

	return rpcResp.Result, latency, nil
}

func logLatency(label string, ref, target time.Duration) {
	refMs := float64(ref) / float64(time.Millisecond)
	targetMs := float64(target) / float64(time.Millisecond)
	diffMs := targetMs - refMs
	factor := 0.0
	if refMs > 0 {
		factor = targetMs / refMs
	}

	// Format: Label | Ref=X | Target=Y | Diff=+/-Z (xF.FF)
	// Using fixed width for numbers to align visually in logs
	log.Printf("%s Latency: Ref=%6.1fms | Target=%6.1fms | Diff=%+6.1fms (x%.2f)",
		label, refMs, targetMs, diffMs, factor)
}

// compareJSON uses jd to print structural diffs
func compareJSON(ref []byte, target []byte, label string, stopOnDiff bool) {
	// Pre-process to scrub fields that are hard to target with path options (wildcards)
	var refObj, targetObj interface{}
	if err := json.Unmarshal(ref, &refObj); err != nil {
		panic(fmt.Errorf("label %s : failed to unmarshal ref JSON for scrubbing: %w", label, err))
	}
	if err := json.Unmarshal(target, &targetObj); err != nil {
		panic(fmt.Errorf("label %s : failed to unmarshal target JSON for scrubbing: %w", label, err))
	}

	scrubKey(refObj, "stackHeight")
	scrubKey(targetObj, "stackHeight")
	scrubCompatibleLogs(refObj, targetObj)
	scrubKey(targetObj, "position")
	// rewards can be noisy
	scrubKey(refObj, "rewards")
	scrubKey(targetObj, "rewards")

	// Normalize RPC values (0 vs null, [] vs null)
	normalizeRPC(refObj)
	normalizeRPC(targetObj)

	refScrubbed, _ := json.Marshal(refObj)
	targetScrubbed, _ := json.Marshal(targetObj)

	a, err := jd.ReadJsonString(string(refScrubbed))
	if err != nil {
		panic(fmt.Errorf("label %s : failed to parse ref JSON: %w", label, err))
	}
	b, err := jd.ReadJsonString(string(targetScrubbed))
	if err != nil {
		panic(fmt.Errorf("label %s : failed to parse target JSON: %w", label, err))
	}

	// Ignore common noisy fields
	ignoreOpts := `[
		{"@":["blockTime"],"^":["DIFF_OFF"]},
		{"@":["version"],"^":["DIFF_OFF"]}
	]`
	opts, _ := jd.ReadOptionsString(ignoreOpts)

	diff := a.Diff(b, opts...)

	if diffStr := diff.Render(); diffStr != "" {
		fmt.Printf(" [!] DISCREPANCY found for %s\n", label)
		fmt.Print(diffStr)
		if stopOnDiff {
			fmt.Printf("Stopping on diff for %s\n", label)
			os.Exit(1)
		}
	}
}

func scrubKey(v interface{}, key string) {
	switch tv := v.(type) {
	case map[string]interface{}:
		delete(tv, key)
		for _, val := range tv {
			scrubKey(val, key)
		}
	case []interface{}:
		for _, val := range tv {
			scrubKey(val, key)
		}
	}
}

func scrubCompatibleLogs(ref, target interface{}) {
	refMap, rOk := ref.(map[string]interface{})
	targetMap, tOk := target.(map[string]interface{})

	if rOk && tOk {
		// Check for logMessages at this level
		if rLogs, rHas := refMap["logMessages"]; rHas {
			if tLogs, tHas := targetMap["logMessages"]; tHas {
				if isLogSubset(rLogs, tLogs) {
					delete(refMap, "logMessages")
					delete(targetMap, "logMessages")
				}
			}
		}

		// Recurse into common keys
		for k, v := range refMap {
			if tv, exists := targetMap[k]; exists {
				scrubCompatibleLogs(v, tv)
			}
		}
		return
	}

	refSlice, rOk := ref.([]interface{})
	targetSlice, tOk := target.([]interface{})
	if rOk && tOk {
		// Recurse into array elements (e.g. transactions array)
		limit := len(refSlice)
		if len(targetSlice) < limit {
			limit = len(targetSlice)
		}
		for i := 0; i < limit; i++ {
			scrubCompatibleLogs(refSlice[i], targetSlice[i])
		}
	}
}

func isLogSubset(refVal, targetVal interface{}) bool {
	rList, rOk := refVal.([]interface{})
	tList, tOk := targetVal.([]interface{})
	if !rOk || !tOk {
		return false // Malformed or not arrays
	}

	for i, rItem := range rList {
		rStr, rStrOk := rItem.(string)
		if !rStrOk {
			return false // Logs should be strings
		}

		// If Ref says "Log truncated", we accept it as a match (prefix matched so far)
		if rStr == "Log truncated" {
			return true
		}

		// If Ref has more lines than Target (and wasn't truncated), mismatch
		if i >= len(tList) {
			return false
		}

		tStr, tStrOk := tList[i].(string)
		if !tStrOk || rStr != tStr {
			return false // Content mismatch
		}
	}

	// Ref is a prefix of Target (or exact match)
	return true
}

func normalizeRPC(v interface{}) {
	switch tv := v.(type) {
	case map[string]interface{}:
		// Handle blockHeight: 0 -> null
		if val, ok := tv["blockHeight"]; ok {
			if f, ok := val.(float64); ok && f == 0 {
				tv["blockHeight"] = nil
			}
		}

		// Handle array fields: [] -> null
		// Common fields in transaction meta that might be empty or null
		targetFields := []string{"innerInstructions", "postTokenBalances", "preTokenBalances"}
		for _, key := range targetFields {
			if val, ok := tv[key]; ok {
				if slice, ok := val.([]interface{}); ok && len(slice) == 0 {
					tv[key] = nil
				}
			}
		}

		// Recurse
		for _, val := range tv {
			normalizeRPC(val)
		}
	case []interface{}:
		for _, val := range tv {
			normalizeRPC(val)
		}
	}
}
