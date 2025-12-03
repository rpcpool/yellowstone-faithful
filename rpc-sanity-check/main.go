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
	"strings"
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

	// Configure logger to remove timestamps for cleaner output (we can add them back if strictly needed,
	// but for CLI tools, raw output often looks better).
	// However, keeping standard log format is safer for debugging.
	// Let's stick to standard log but clean the messages.
	log.SetFlags(log.Ltime) // Only show time, remove date to save space

	log.Printf("🔹 Starting Verification")
	log.Printf("   Ref:    %s", cfg.RefRPC)
	log.Printf("   Target: %s", cfg.TargetRPC)

	// 1. Fetch Epochs
	epochs, err := fetchEpochs(cfg.TargetRPC)
	if err != nil {
		log.Fatalf("❌ Failed to fetch epochs: %v", err)
	}
	log.Printf("   Epochs: %d found %v", len(epochs), epochs)

	client := &http.Client{Timeout: 30 * time.Second}

	// 2. Iterate Epochs
	for _, epoch := range epochs {
		fmt.Println() // Visual separator
		log.Printf("⏩ EPOCH %d", epoch)

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
			log.Printf("⚠️  Failed to generate secure random number: %v", err)
			slots[i] = min
			continue
		}
		slots[i] = min + offset.Uint64()
	}
	return slots
}

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
	// Params for getBlock
	params := []interface{}{
		slot,
		map[string]interface{}{
			"encoding":                       "json",
			"transactionDetails":             "full",
			"rewards":                        false,
			"maxSupportedTransactionVersion": 0,
		},
	}

	// 3. Fetch Block from both
	refBlock, refLat, refErr := callRPC(client, cfg.RefRPC, "getBlock", params)
	targetBlock, targetLat, targetErr := callRPC(client, cfg.TargetRPC, "getBlock", params)

	// Log Slot Header with Latency
	logLatency(fmt.Sprintf("📦 SLOT %d", slot), refLat, targetLat)

	// Handle availability issues
	if refErr != nil || targetErr != nil {
		if refErr != nil && targetErr != nil {
			if cfg.Verbose {
				log.Printf("   ⚠️  Skipped (both missing)")
			}
			return
		}
		log.Printf("   ❌ FETCH ERROR | Ref: %v | Target: %v", errorStr(refErr), errorStr(targetErr))
		return
	}

	// 4. Compare Block Data
	var refData, targetData interface{}
	json.Unmarshal(refBlock, &refData)
	json.Unmarshal(targetBlock, &targetData)

	if !reflect.DeepEqual(refData, targetData) {
		compareJSON(refBlock, targetBlock, fmt.Sprintf("Block %d", slot), cfg.StopOnDiff)
	} else if cfg.Verbose {
		log.Printf("   ✅ Content Match")
	}

	// Extract signatures
	var blockStruct struct {
		Transactions []struct {
			Transaction struct {
				Signatures []string `json:"signatures"`
			} `json:"transaction"`
		} `json:"transactions"`
	}

	if err := json.Unmarshal(targetBlock, &blockStruct); err != nil {
		log.Printf("   ❌ Failed to parse block structure: %v", err)
		return
	}

	sigsToCheck := []string{}
	txCount := len(blockStruct.Transactions)

	if txCount == 0 {
		return
	}

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

	// Indented log for transactions
	logLatency(fmt.Sprintf("   📄 %s", shortSig(signature)), refLat, targetLat)

	if refErr != nil && targetErr != nil {
		return
	}
	if refErr != nil || targetErr != nil {
		log.Printf("      ❌ TX FETCH ERROR | Ref: %v | Target: %v", errorStr(refErr), errorStr(targetErr))
		return
	}

	var refData, targetData interface{}
	json.Unmarshal(refTx, &refData)
	json.Unmarshal(targetTx, &targetData)

	if !reflect.DeepEqual(refData, targetData) {
		compareJSON(refTx, targetTx, signature, cfg.StopOnDiff)
	} else if cfg.Verbose {
		log.Printf("      ✅ Match")
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

	if string(rpcResp.Result) == "null" {
		return nil, latency, fmt.Errorf("result is null")
	}

	return rpcResp.Result, latency, nil
}

// logLatency prints a fixed-width, aligned latency comparison
func logLatency(label string, ref, target time.Duration) {
	refMs := float64(ref) / float64(time.Millisecond)
	targetMs := float64(target) / float64(time.Millisecond)
	diffMs := targetMs - refMs

	factor := 0.0
	if refMs > 0 {
		factor = targetMs / refMs
	}

	// Symbols for quick visual scanning of performance
	// If Target is significantly slower (>1.5x), warn
	perfStatus := ""
	if factor > 1.5 && diffMs > 50 { // Only warn if also absolute diff is meaningful
		perfStatus = "⚠️ "
	} else if factor < 0.8 {
		perfStatus = "🚀"
	}

	// Layout:
	// Label .................... | Ref= 123.4ms | Target= 123.4ms | Diff= +10.0ms (x1.00) ⚠️

	// Pad label to align columns (max label length assumption ~45 chars for indented txs)
	// Truncate label if too long to prevent wrap
	paddedLabel := label
	if len(label) < 50 {
		paddedLabel = label + strings.Repeat(" ", 50-len(label))
	}

	log.Printf("%s | Ref=%6.1fms | Target=%6.1fms | Diff=%+6.1fms (x%.2f) %s",
		paddedLabel, refMs, targetMs, diffMs, factor, perfStatus)
}

func compareJSON(ref []byte, target []byte, label string, stopOnDiff bool) {
	// Pre-process to scrub fields
	var refObj, targetObj interface{}
	if err := json.Unmarshal(ref, &refObj); err != nil {
		log.Printf("      ❌ JSON Parse Error (Ref): %v", err)
		return
	}
	if err := json.Unmarshal(target, &targetObj); err != nil {
		log.Printf("      ❌ JSON Parse Error (Target): %v", err)
		return
	}

	scrubKey(refObj, "stackHeight")
	scrubKey(targetObj, "stackHeight")
	scrubCompatibleLogs(refObj, targetObj)
	scrubKey(targetObj, "position")
	scrubKey(refObj, "rewards")
	scrubKey(targetObj, "rewards")

	normalizeRPC(refObj)
	normalizeRPC(targetObj)

	refScrubbed, _ := json.Marshal(refObj)
	targetScrubbed, _ := json.Marshal(targetObj)

	a, err := jd.ReadJsonString(string(refScrubbed))
	if err != nil {
		return
	}
	b, err := jd.ReadJsonString(string(targetScrubbed))
	if err != nil {
		return
	}

	ignoreOpts := `[
		{"@":["blockTime"],"^":["DIFF_OFF"]},
		{"@":["version"],"^":["DIFF_OFF"]}
	]`
	opts, _ := jd.ReadOptionsString(ignoreOpts)

	diff := a.Diff(b, opts...)

	if diffStr := diff.Render(); diffStr != "" {
		log.Printf("      ❌ DISCREPANCY: %s", label)
		// Indent the diff output for cleaner look
		lines := strings.Split(diffStr, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("          %s\n", line)
			}
		}

		if stopOnDiff {
			log.Printf("      🛑 Stopping on diff")
			os.Exit(1)
		}
	}
}

// Helpers

func errorStr(err error) string {
	if err == nil {
		return "OK"
	}
	return err.Error()
}

func shortSig(sig string) string {
	if len(sig) > 16 {
		return sig[:8] + "..." + sig[len(sig)-8:]
	}
	return sig
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
		if rLogs, rHas := refMap["logMessages"]; rHas {
			if tLogs, tHas := targetMap["logMessages"]; tHas {
				if isLogSubset(rLogs, tLogs) {
					delete(refMap, "logMessages")
					delete(targetMap, "logMessages")
				}
			}
		}
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
		return false
	}

	for i, rItem := range rList {
		rStr, rStrOk := rItem.(string)
		if !rStrOk {
			return false
		}
		if rStr == "Log truncated" {
			return true
		}
		if i >= len(tList) {
			return false
		}
		tStr, tStrOk := tList[i].(string)
		if !tStrOk || rStr != tStr {
			return false
		}
	}
	return true
}

func normalizeRPC(v interface{}) {
	switch tv := v.(type) {
	case map[string]interface{}:
		if val, ok := tv["blockHeight"]; ok {
			if f, ok := val.(float64); ok && f == 0 {
				tv["blockHeight"] = nil
			}
		}
		targetFields := []string{"innerInstructions", "postTokenBalances", "preTokenBalances"}
		for _, key := range targetFields {
			if val, ok := tv[key]; ok {
				if slice, ok := val.([]interface{}); ok && len(slice) == 0 {
					tv[key] = nil
				}
			}
		}
		for _, val := range tv {
			normalizeRPC(val)
		}
	case []interface{}:
		for _, val := range tv {
			normalizeRPC(val)
		}
	}
}
