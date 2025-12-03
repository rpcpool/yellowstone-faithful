package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	jd "github.com/josephburnett/jd/v2"
)

// Config holds the command line arguments
type Config struct {
	RefRPC          string
	TargetRPC       string
	SlotsPerEpoch   int
	MaxTxsToCheck   int
	Verbose         bool
	SlotsInEpoch    int64 // Standard Solana slots per epoch
	StopOnDiff      bool
	FullSig         bool
	SkipEpochs      FlagUint64Slice
	RunGRPCLoadTest bool
	GRPCTarget      string
	GRPCToken       string
	GRPCProto       string
	GRPCConcurrency int
	RunTxLoadTest   bool
	TxConcurrency   int
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

type FlagUint64Slice []uint64

func (f *FlagUint64Slice) String() string {
	strs := []string{}
	for _, v := range *f {
		strs = append(strs, fmt.Sprintf("%d", v))
	}
	return strings.Join(strs, ",")
}

func (f *FlagUint64Slice) Set(value string) error {
	var parsed uint64
	_, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil {
		return err
	}
	*f = append(*f, parsed)
	return nil
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
	flag.BoolVar(&cfg.FullSig, "full-sig", false, "Print full transaction signatures in logs")
	flag.BoolVar(&cfg.RunGRPCLoadTest, "grpc-load-test", false, "Run gRPC load test step")
	flag.StringVar(&cfg.GRPCTarget, "grpc-target", "", "Target gRPC endpoint (required for --grpc-load-test)")
	flag.StringVar(&cfg.GRPCToken, "grpc-token", "<token>", "Auth token for gRPC")
	flag.StringVar(&cfg.GRPCProto, "grpc-proto", "old-faithful-proto/proto/old-faithful.proto", "Path to .proto file")
	flag.IntVar(&cfg.GRPCConcurrency, "grpc-concurrency", 100, "Number of concurrent gRPC streams")
	flag.BoolVar(&cfg.RunTxLoadTest, "tx-load-test", false, "Run getTransaction load test (JSON-RPC)")
	flag.IntVar(&cfg.TxConcurrency, "tx-concurrency", 100, "Concurrency for tx load test")
	flag.Var(&cfg.SkipEpochs, "skip-epoch", "Epoch number to skip (can be specified multiple times)")
	flag.Parse()

	// Configure logger to remove timestamps for cleaner output (we can add them back if strictly needed,
	// but for CLI tools, raw output often looks better).
	// However, keeping standard log format is safer for debugging.
	// Let's stick to standard log but clean the messages.
	log.SetFlags(log.Ltime) // Only show time, remove date to save space

	if cfg.RunGRPCLoadTest {
		if cfg.GRPCTarget == "" {
			log.Fatal("❌ --grpc-target is required when --grpc-load-test is enabled")
		}
		runGRPCLoadTest(cfg)
		return
	}

	if cfg.RunTxLoadTest {
		runTxLoadTest(cfg)
		return
	}

	log.Printf("🔹 Starting Verification")
	log.Printf("   Ref:    %s", cfg.RefRPC)
	log.Printf("   Target: %s", cfg.TargetRPC)

	// 1. Fetch Epochs
	epochs, err := fetchEpochs(cfg.TargetRPC)
	if err != nil {
		log.Fatalf("❌ Failed to fetch epochs: %v", err)
	}
	log.Printf("   Epochs: %d found %v", len(epochs), epochs)
	if len(epochs) == 0 {
		log.Fatal("❌ No epochs returned from target")
	}
	if len(cfg.SkipEpochs) > 0 {
		skipMap := make(map[uint64]bool)
		for _, e := range cfg.SkipEpochs {
			skipMap[e] = true
		}
		filtered := []uint64{}
		for _, e := range epochs {
			if !skipMap[e] {
				filtered = append(filtered, e)
			} else {
				log.Printf("   ⏭️  Skipping epoch %d as per configuration", e)
			}
		}
		epochs = filtered
		log.Printf("   Epochs after skip: %d remaining %v", len(epochs), epochs)
	}

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

	// 1. Fetch Block from Ref ONLY first (to get signatures and baseline)
	refBlock, refLat, refErr := callRPC(client, cfg.RefRPC, "getBlock", params)

	if refErr != nil {
		// If Ref failed, we can't get signatures to check Txs, but we should still try to check Target Block existence?
		// Or just fail the slot. Let's fail the slot for consistency with previous logic which required both.
		// However, to strictly check Target Block latency independently, we could fetch it, but we have no Ref to compare.
		// Let's abort if Ref fails.
		log.Printf("   ❌ Ref Fetch Failed: %v", refErr)
		return
	}

	// 2. Extract signatures from Ref Block
	var blockStruct struct {
		Transactions []struct {
			Transaction struct {
				Signatures []string `json:"signatures"`
			} `json:"transaction"`
		} `json:"transactions"`
	}

	if err := json.Unmarshal(refBlock, &blockStruct); err != nil {
		log.Printf("   ❌ Failed to parse ref block structure: %v", err)
		return
	}

	sigsToCheck := []string{}
	txCount := len(blockStruct.Transactions)

	if txCount > 0 {
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
	}

	// 3. Check Transactions (Target "cold" read - assuming block hasn't been fetched yet)
	for _, sig := range sigsToCheck {
		compareTransaction(client, cfg, sig)
	}

	// 4. Fetch Block from Target (now that Txs are checked)
	targetBlock, targetLat, targetErr := callRPC(client, cfg.TargetRPC, "getBlock", params)

	// Log Slot Header with Latency (now that we have both)
	logLatency(fmt.Sprintf("📦 SLOT %d", slot), refLat, targetLat)

	// Handle availability issues
	if targetErr != nil {
		log.Printf("   ❌ Target Fetch Failed: %v", targetErr)
		return
	}

	// 5. Compare Block Data
	var refData, targetData interface{}
	json.Unmarshal(refBlock, &refData)
	json.Unmarshal(targetBlock, &targetData)

	if !reflect.DeepEqual(refData, targetData) {
		compareJSON(refBlock, targetBlock, fmt.Sprintf("Block %d", slot), cfg.StopOnDiff)
	} else if cfg.Verbose {
		log.Printf("   ✅ Content Match")
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
	logLatency(fmt.Sprintf("   📄 %s", shortSig(signature, cfg.FullSig)), refLat, targetLat)

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

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, latency, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

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

func shortSig(sig string, full bool) string {
	if full {
		return sig
	}
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

func runGRPCLoadTest(cfg Config) {
	log.Printf("🔥 Starting gRPC Load Test")
	log.Printf("   Concurrency: %d", cfg.GRPCConcurrency)
	log.Printf("   Endpoint:    %s", cfg.GRPCTarget)
	log.Printf("   Proto:       %s", cfg.GRPCProto)
	log.Printf("   Target:      OldFaithful.OldFaithful/StreamTransactions")

	// Verify grpcurl exists
	if _, err := exec.LookPath("grpcurl"); err != nil {
		log.Fatal("❌ grpcurl not found in PATH. Please install it to run the load test.")
	}

	// Fetch epochs from target to determine a valid slot range
	epochs, err := fetchEpochs(cfg.TargetRPC)
	if err != nil {
		log.Fatalf("❌ Failed to fetch epochs for load test config: %v", err)
	}
	if len(epochs) == 0 {
		log.Fatal("❌ No epochs returned from target")
	}

	// Use the first epoch (usually the latest)
	targetEpoch := epochs[0]
	startSlot := targetEpoch * uint64(cfg.SlotsInEpoch)
	endSlot := startSlot + uint64(cfg.SlotsInEpoch) - 1

	log.Printf("   📅 Configured for Epoch %d (Slots %d-%d)", targetEpoch, startSlot, endSlot)

	// Payload construction
	payload := fmt.Sprintf(`{
        "start_slot": %d, 
        "end_slot": %d, 
        "filter": {
            "vote": false, 
            "failed": false, 
            "account_include":["TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"]
        }
    }`, startSlot, endSlot)

	// Context to handle cancellation (Ctrl+C)
	// This context will kill child processes when cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n🛑 Stopping load test...")
		cancel()
	}()

	var wg sync.WaitGroup
	var totalTx uint64
	startTime := time.Now()

	// TPS Monitor
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var lastCount uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := atomic.LoadUint64(&totalTx)
				diff := current - lastCount
				lastCount = current
				elapsed := time.Since(startTime).Seconds()
				avg := 0.0
				if elapsed > 0 {
					avg = float64(current) / elapsed
				}
				log.Printf("   ⚡ Status: %d total txs | Current TPS: %d | Avg TPS: %.1f", current, diff, avg)
			}
		}
	}()

	log.Printf("   🚀 Launching %d streams...", cfg.GRPCConcurrency)

	for i := 0; i < cfg.GRPCConcurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Construct arguments
			args := []string{
				"-proto", cfg.GRPCProto,
				"-H", fmt.Sprintf("x-token: %s", cfg.GRPCToken),
				"-plaintext",
				"-keepalive-time", "10",
				"-max-time", "0",
				"-d", payload,
				cfg.GRPCTarget,
				"OldFaithful.OldFaithful/StreamTransactions",
			}

			cmd := exec.CommandContext(ctx, "grpcurl", args...)

			// Capture stdout to count transactions
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Printf("   ❌ Stream %d failed to get stdout: %v", id, err)
				return
			}

			// Capture stderr to debug immediate failures
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			if err := cmd.Start(); err != nil {
				log.Printf("   ❌ Stream %d failed to start: %v", id, err)
				return
			}

			// Parse output for transactions
			scanner := bufio.NewScanner(stdout)
			// Increase buffer size if needed, but default is usually fine for JSON lines unless they are huge
			// scanner.Buffer(make([]byte, 64*1024), 1024*1024)

			go func() {
				for scanner.Scan() {
					line := scanner.Text()
					// Heuristic: Assume distinct transaction messages contain "slot"
					// This works if grpcurl pretty prints (multiple lines per tx) or prints single lines.
					// We just count occurrences of the key.
					if strings.Contains(line, "\"slot\":") {
						atomic.AddUint64(&totalTx, 1)
					}
				}
			}()

			// Wait for command to finish (or be killed by context)
			if err := cmd.Wait(); err != nil {
				// Only log if context wasn't cancelled (clean shutdown)
				if ctx.Err() == nil {
					// Use strings.TrimSpace to clean up the error log
					log.Printf("   ❌ Stream %d exited early: %v | Stderr: %s", id, err, strings.TrimSpace(stderr.String()))
				}
			}
		}(i)
	}

	log.Printf("   ✅ All streams launched. Waiting... (Press Ctrl+C to stop)")
	wg.Wait()
	log.Printf("   🏁 Load test finished.")
}

func runTxLoadTest(cfg Config) {
	log.Printf("🔥 Starting getTransaction Load Test (JSON-RPC)")
	log.Printf("   Concurrency: %d", cfg.TxConcurrency)
	log.Printf("   Target:      %s", cfg.TargetRPC)

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.TxConcurrency,
		MaxIdleConnsPerHost:   cfg.TxConcurrency,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// 1. Harvest Signatures
	log.Printf("   🌾 Harvesting signatures from reference (%s) using target epochs...", cfg.RefRPC)

	// Get Target Epochs to know valid ranges
	targetEpochs, err := fetchEpochs(cfg.TargetRPC)
	if err != nil || len(targetEpochs) == 0 {
		log.Fatalf("❌ Failed to fetch epochs from target: %v", err)
	}

	totalTargetSigs := 100_000
	// Calculate signatures needed per epoch to reach total ~50k
	// Ceiling division: (x + y - 1) / y
	sigsPerEpoch := (totalTargetSigs + len(targetEpochs) - 1) / len(targetEpochs)
	// Ensure we grab at least a few if calculation is weird, though math holds.
	if sigsPerEpoch == 0 {
		sigsPerEpoch = 100
	}

	log.Printf("   🎯 Plan: Harvest ~%d total signatures across %d epochs (~%d/epoch)", totalTargetSigs, len(targetEpochs), sigsPerEpoch)

	sigs := []string{}

	for i, epoch := range targetEpochs {
		epochSigs := []string{}
		startSlot := epoch * uint64(cfg.SlotsInEpoch)
		endSlot := startSlot + uint64(cfg.SlotsInEpoch) - 1
		rangeSz := new(big.Int).SetUint64(endSlot - startSlot)

		// Try up to 20 random slots per epoch to fill the quota
		// This prevents getting stuck on an epoch with missing blocks in ref
		for attempts := 0; attempts < 20 && len(epochSigs) < sigsPerEpoch; attempts++ {
			// Generate random slot in epoch
			offset, err := rand.Int(rand.Reader, rangeSz)
			if err != nil {
				continue
			}
			slot := startSlot + offset.Uint64()

			params := []interface{}{
				slot,
				map[string]interface{}{
					"encoding":                       "json",
					"transactionDetails":             "full",
					"rewards":                        false,
					"maxSupportedTransactionVersion": 0,
				},
			}

			// Fetch from Reference RPC
			block, _, err := callRPC(client, cfg.RefRPC, "getBlock", params)
			if err != nil {
				continue
			}

			// Extract signatures
			var blockStruct BlockShort
			if err := json.Unmarshal(block, &blockStruct); err == nil {
				for _, tx := range blockStruct.Transactions {
					if txMap, ok := tx.Transaction.(map[string]interface{}); ok {
						if sigsArr, ok := txMap["signatures"].([]interface{}); ok && len(sigsArr) > 0 {
							if s, ok := sigsArr[0].(string); ok {
								epochSigs = append(epochSigs, s)
							}
						}
					}
				}

				// Fallback for different structure
				if len(epochSigs) == 0 {
					var b struct {
						Transactions []struct {
							Transaction struct {
								Signatures []string `json:"signatures"`
							} `json:"transaction"`
						} `json:"transactions"`
					}
					if err := json.Unmarshal(block, &b); err == nil {
						for _, t := range b.Transactions {
							if len(t.Transaction.Signatures) > 0 {
								epochSigs = append(epochSigs, t.Transaction.Signatures[0])
							}
						}
					}
				}
			}
		}

		// Cap and append
		if len(epochSigs) > sigsPerEpoch {
			epochSigs = epochSigs[:sigsPerEpoch]
		}
		sigs = append(sigs, epochSigs...)
		fmt.Printf("\r   🌾 Harvested %d signatures (Epoch %d/%d)...", len(sigs), i+1, len(targetEpochs))
	}
	fmt.Println()

	if len(sigs) == 0 {
		log.Fatal("❌ Failed to harvest any signatures. Check reference RPC health and epoch alignment.")
	}
	log.Printf("   ✅ Ready with %d unique signatures.", len(sigs))

	// 2. Start Load Test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n🛑 Stopping load test...")
		cancel()
	}()

	var wg sync.WaitGroup
	var totalTx uint64
	var totalLatency int64 // microseconds
	var totalHttpErrors uint64
	var totalNetworkErrors uint64
	var totalRpcErrors uint64
	startTime := time.Now()

	// TPS Monitor
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var lastCount uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := atomic.LoadUint64(&totalTx)
				latencySum := atomic.LoadInt64(&totalLatency)

				nErr := atomic.LoadUint64(&totalNetworkErrors)
				hErr := atomic.LoadUint64(&totalHttpErrors)
				rErr := atomic.LoadUint64(&totalRpcErrors)

				diff := current - lastCount
				lastCount = current

				elapsed := time.Since(startTime).Seconds()
				avg := 0.0
				if elapsed > 0 {
					avg = float64(current) / elapsed
				}

				avgLat := 0.0
				if current > 0 {
					avgLat = float64(latencySum) / float64(current) / 1000.0 // ms
				}

				log.Printf("   ⚡ Status: %d total | TPS: %d (Avg: %.1f) | Avg Latency: %.1fms | NetErr: %d | HTTPErr: %d | RPCErr: %d",
					current, diff, avg, avgLat, nErr, hErr, rErr)
			}
		}
	}()

	log.Printf("   🚀 Launching %d workers...", cfg.TxConcurrency)

	for i := 0; i < cfg.TxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Pick random signature
					sig := sigs[randInt(len(sigs))] // Using insecure rand for speed here or crypto/rand wrapped

					// Reusing crypto/rand based helper or just math/rand?
					// main uses crypto/rand. Let's make a quick helper or just use math/rand seeded once if not strict.
					// Actually, for load test, math/rand is fine and faster.
					// But we haven't imported math/rand (only math/big).
					// Let's use crypto/rand helper below.

					params := []interface{}{
						sig,
						map[string]interface{}{
							"encoding":                       "json",
							"maxSupportedTransactionVersion": 0,
						},
					}

					_, dur, err := callRPC(client, cfg.TargetRPC, "getTransaction", params)
					if err == nil {
						atomic.AddUint64(&totalTx, 1)
						atomic.AddInt64(&totalLatency, int64(dur.Microseconds()))
					} else {
						errMsg := err.Error()
						if strings.Contains(errMsg, "RPC Error") {
							atomic.AddUint64(&totalRpcErrors, 1)
						} else if strings.Contains(errMsg, "unexpected status code") {
							atomic.AddUint64(&totalHttpErrors, 1)
						} else {
							atomic.AddUint64(&totalNetworkErrors, 1)
						}
					}
				}
			}
		}()
	}

	wg.Wait()
}

func randInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}
