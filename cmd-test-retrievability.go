package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_TestRetrievability() *cli.Command {
	return &cli.Command{
		Name:  "test-retrievability",
		Usage: "Test retrievability of CIDs from Filecoin network",
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Usage:   "Input file containing CIDs (one per line), use '-' for stdin",
			},
			&cli.StringFlag{
				Name:  "deals-csv",
				Usage: "Deals CSV file to extract CIDs from (alternative to --input)",
			},
			&cli.StringFlag{
				Name:  "cid-type",
				Usage: "Which CIDs to test from deals.csv: 'piece', 'payload', or 'both' (default: both)",
				Value: "both",
			},
			&cli.StringFlag{
				Name:  "method",
				Usage: "Retrieval method: 'lassie', 'http', or 'both' (default: both)",
				Value: "both",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output file for results (CSV format), use '-' for stdout",
				Value:   "-",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Timeout per CID retrieval attempt",
				Value: 60 * time.Second,
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose output",
			},
		}, commonLassieFlags()...),
		Action: testRetrievabilityAction,
	}
}

func testRetrievabilityAction(cctx *cli.Context) error {
	ctx := cctx.Context
	inputFile := cctx.String("input")
	dealsCSV := cctx.String("deals-csv")
	cidType := cctx.String("cid-type")
	method := cctx.String("method")
	outputFile := cctx.String("output")
	timeout := cctx.Duration("timeout")
	verbose := cctx.Bool("verbose")

	// Validate input parameters
	if inputFile == "" && dealsCSV == "" {
		return fmt.Errorf("either --input or --deals-csv must be specified")
	}
	if inputFile != "" && dealsCSV != "" {
		return fmt.Errorf("cannot specify both --input and --deals-csv")
	}
	if dealsCSV != "" && cidType != "piece" && cidType != "payload" && cidType != "both" {
		return fmt.Errorf("invalid --cid-type: must be 'piece', 'payload', or 'both'")
	}
	if method != "lassie" && method != "http" && method != "both" {
		return fmt.Errorf("invalid --method: must be 'lassie', 'http', or 'both'")
	}

	// Read CIDs from input
	var cids []string
	var err error
	if dealsCSV != "" {
		cids, err = readCIDsFromDealsCSV(dealsCSV, cidType)
	} else {
		cids, err = readCIDsFromInput(inputFile)
	}
	if err != nil {
		return fmt.Errorf("failed to read CIDs: %w", err)
	}

	if len(cids) == 0 {
		return fmt.Errorf("no valid CIDs found in input")
	}

	klog.Infof("Testing retrievability for %d CIDs", len(cids))

	// Initialize Lassie wrapper if needed
	var lassieWrapper *lassieWrapper
	if method == "lassie" || method == "both" {
		lassieWrapper, err = newLassieWrapper(cctx, globalFetchProviderAddrInfos)
		if err != nil {
			return fmt.Errorf("failed to initialize lassie: %w", err)
		}
	}

	// Initialize deal registry if needed for HTTP method
	var dealRegistry *splitcarfetcher.DealRegistry
	if method == "http" || method == "both" {
		if dealsCSV == "" {
			return fmt.Errorf("--deals-csv is required when using HTTP retrieval method")
		}
		dealRegistry, err = splitcarfetcher.DealsFromCSV(dealsCSV)
		if err != nil {
			return fmt.Errorf("failed to load deals registry: %w", err)
		}
	}

	// Setup output writer
	outputWriter, err := setupOutputWriter(outputFile)
	if err != nil {
		return err
	}
	if outputWriter != os.Stdout {
		defer outputWriter.Close()
	}

	// Process CIDs and write results
	return processCIDs(ctx, lassieWrapper, dealRegistry, cids, outputWriter, timeout, verbose, method, cidType)
}

type RetrievabilityResult struct {
	CID         string
	Method      string
	Retrievable bool
	Duration    time.Duration
	Error       string
}

func setupOutputWriter(outputFile string) (*os.File, error) {
	if outputFile == "-" {
		return os.Stdout, nil
	}

	outputWriter, err := os.Create(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	return outputWriter, nil
}

func processCIDs(ctx context.Context, lassieWrapper *lassieWrapper, dealRegistry *splitcarfetcher.DealRegistry, cids []string, outputWriter *os.File, timeout time.Duration, verbose bool, method string, cidType string) error {
	// Write CSV header
	fmt.Fprintln(outputWriter, "CID,Method,Retrievable,Duration,Error")

	// Test each CID
	for i, cidStr := range cids {
		if verbose {
			klog.Infof("Testing CID %d/%d: %s", i+1, len(cids), cidStr)
		}

		results := testCIDRetrievabilityWithMethods(ctx, lassieWrapper, dealRegistry, cidStr, timeout, method)

		// Write results to CSV
		for _, result := range results {
			fmt.Fprintf(outputWriter, "%s,%s,%t,%s,%s\n",
				result.CID,
				result.Method,
				result.Retrievable,
				result.Duration.String(),
				escapeCSV(result.Error))
			logResult(result, verbose)
		}

	}

	if !verbose {
		fmt.Fprintln(os.Stderr) // New line after progress indicators
	}

	klog.Infof("Retrievability test completed for %d CIDs", len(cids))
	return nil
}

func logResult(result RetrievabilityResult, verbose bool) {
	if verbose {
		if result.Retrievable {
			klog.Infof("✓ %s [%s] - retrievable (%s)", result.CID, result.Method, result.Duration)
		} else {
			klog.Infof("✗ %s [%s] - not retrievable: %s", result.CID, result.Method, result.Error)
		}
	} else {
		// Show progress
		if result.Retrievable {
			fmt.Fprint(os.Stderr, "✓")
		} else {
			fmt.Fprint(os.Stderr, "✗")
		}
	}
}

func testCIDRetrievabilityWithMethods(ctx context.Context, lassie *lassieWrapper, dealRegistry *splitcarfetcher.DealRegistry, cidStr string, timeout time.Duration, method string) []RetrievabilityResult {
	var results []RetrievabilityResult

	switch method {
	case "lassie":
		results = append(results, testCIDRetrievability(ctx, lassie, cidStr, timeout, "lassie"))
	case "http":
		results = append(results, testCIDRetrievabilityHTTP(ctx, dealRegistry, cidStr, timeout))
	case "both":
		results = append(results, testCIDRetrievability(ctx, lassie, cidStr, timeout, "lassie"))
		results = append(results, testCIDRetrievabilityHTTP(ctx, dealRegistry, cidStr, timeout))
	}

	return results
}

func testCIDRetrievability(ctx context.Context, lassie *lassieWrapper, cidStr string, timeout time.Duration, method string) RetrievabilityResult {
	result := RetrievabilityResult{
		CID:    cidStr,
		Method: method,
	}

	// Parse CID
	parsedCID, err := cid.Parse(cidStr)
	if err != nil {
		result.Error = fmt.Sprintf("invalid CID: %v", err)
		return result
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Measure retrieval time
	start := time.Now()

	// Attempt to fetch just the block (not the entire DAG)
	_, err = lassie.GetNodeByCid(timeoutCtx, parsedCID)

	result.Duration = time.Since(start)

	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			result.Error = "timeout"
		} else {
			result.Error = err.Error()
		}
		result.Retrievable = false
	} else {
		result.Retrievable = true
	}

	return result
}

func testCIDRetrievabilityHTTP(ctx context.Context, dealRegistry *splitcarfetcher.DealRegistry, cidStr string, timeout time.Duration) RetrievabilityResult {
	result := RetrievabilityResult{
		CID:    cidStr,
		Method: "http",
	}

	// Parse CID
	parsedCID, err := cid.Parse(cidStr)
	if err != nil {
		result.Error = fmt.Sprintf("invalid CID: %v", err)
		return result
	}

	// Check if we have deal information for this CID
	if dealRegistry == nil {
		result.Error = "no deal registry available for HTTP retrieval"
		return result
	}

	deal, exists := dealRegistry.GetDeal(parsedCID)
	if !exists {
		result.Error = "CID not found in deals registry"
		return result
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Measure retrieval time
	start := time.Now()

	// Attempt to fetch data via HTTP
	err = testHTTPRetrievability(timeoutCtx, deal.URL)

	result.Duration = time.Since(start)

	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			result.Error = "timeout"
		} else {
			result.Error = err.Error()
		}
		result.Retrievable = false
	} else {
		result.Retrievable = true
	}

	return result
}

func testHTTPRetrievability(ctx context.Context, url string) error {
	// Create HTTP client
	client := splitcarfetcher.NewHTTPClient()
	defer client.CloseIdleConnections()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if the resource is accessible
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func readCIDsFromInput(inputFile string) ([]string, error) {
	var reader *bufio.Scanner

	if inputFile == "-" {
		reader = bufio.NewScanner(os.Stdin)
	} else {
		file, err := os.Open(inputFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = bufio.NewScanner(file)
	}

	var cids []string
	lineNum := 0

	for reader.Scan() {
		lineNum++
		line := strings.TrimSpace(reader.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate CID format
		if _, err := cid.Parse(line); err != nil {
			klog.Warningf("Skipping invalid CID on line %d: %s (%v)", lineNum, line, err)
			continue
		}

		cids = append(cids, line)
	}

	if err := reader.Err(); err != nil {
		return nil, err
	}

	return cids, nil
}

func readCIDsFromDealsCSV(dealsFile string, cidType string) ([]string, error) {
	// Load deals from CSV
	registry, err := splitcarfetcher.DealsFromCSV(dealsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load deals from CSV: %w", err)
	}

	cidMap := make(map[string]bool) // Use map to deduplicate CIDs
	var cids []string

	// Extract CIDs based on the specified type
	for pieceCID, deal := range registry.GetAllDeals() {
		switch cidType {
		case "piece":
			cidStr := pieceCID.String()
			if !cidMap[cidStr] {
				cids = append(cids, cidStr)
				cidMap[cidStr] = true
			}
		case "payload":
			if deal.PayloadCID != "" {
				// Validate that the payload CID is a valid CID
				if _, err := cid.Parse(deal.PayloadCID); err != nil {
					klog.Warningf("Skipping invalid payload CID: %s (%v)", deal.PayloadCID, err)
					continue
				}
				if !cidMap[deal.PayloadCID] {
					cids = append(cids, deal.PayloadCID)
					cidMap[deal.PayloadCID] = true
				}
			}
		case "both":
			// Add piece CID
			pieceCIDStr := pieceCID.String()
			if !cidMap[pieceCIDStr] {
				cids = append(cids, pieceCIDStr)
				cidMap[pieceCIDStr] = true
			}
			// Add payload CID if it exists and is valid
			if deal.PayloadCID != "" {
				if _, err := cid.Parse(deal.PayloadCID); err != nil {
					klog.Warningf("Skipping invalid payload CID: %s (%v)", deal.PayloadCID, err)
				} else if !cidMap[deal.PayloadCID] {
					cids = append(cids, deal.PayloadCID)
					cidMap[deal.PayloadCID] = true
				}
			}
		}
	}

	klog.Infof("Extracted %d unique CIDs from deals.csv (type: %s)", len(cids), cidType)
	return cids, nil
}

func escapeCSV(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + s + "\""
	}
	return s
}

func commonLassieFlags() []cli.Flag {
	return []cli.Flag{
		FlagIPNIEndpoint,
		FlagProtocols,
		FlagAllowProviders,
		FlagExcludeProviders,
		FlagBitswapConcurrency,
		FlagGlobalTimeout,
		FlagProviderTimeout,
	}
}
