package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_TestRetrievability() *cli.Command {
	return &cli.Command{
		Name:  "test-retrievability",
		Usage: "Test retrievability of CIDs from Filecoin network",
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "Input file containing CIDs (one per line), use '-' for stdin",
				Required: true,
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
	outputFile := cctx.String("output")
	timeout := cctx.Duration("timeout")
	verbose := cctx.Bool("verbose")

	// Read CIDs from input
	cids, err := readCIDsFromInput(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read CIDs: %w", err)
	}

	if len(cids) == 0 {
		return fmt.Errorf("no valid CIDs found in input")
	}

	klog.Infof("Testing retrievability for %d CIDs", len(cids))

	// Initialize Lassie wrapper
	lassieWrapper, err := newLassieWrapper(cctx, globalFetchProviderAddrInfos)
	if err != nil {
		return fmt.Errorf("failed to initialize lassie: %w", err)
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
	return processCIDs(ctx, lassieWrapper, cids, outputWriter, timeout, verbose)
}

type RetrievabilityResult struct {
	CID         string
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

func processCIDs(ctx context.Context, lassieWrapper *lassieWrapper, cids []string, outputWriter *os.File, timeout time.Duration, verbose bool) error {
	// Write CSV header
	fmt.Fprintln(outputWriter, "CID,Retrievable,Duration,Error")

	// Test each CID
	for i, cidStr := range cids {
		if verbose {
			klog.Infof("Testing CID %d/%d: %s", i+1, len(cids), cidStr)
		}

		result := testCIDRetrievability(ctx, lassieWrapper, cidStr, timeout)
		
		// Write result to CSV
		fmt.Fprintf(outputWriter, "%s,%t,%s,%s\n", 
			result.CID, 
			result.Retrievable, 
			result.Duration.String(),
			escapeCSV(result.Error))

		logResult(result, verbose)
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
			klog.Infof("✓ %s - retrievable (%s)", result.CID, result.Duration)
		} else {
			klog.Infof("✗ %s - not retrievable: %s", result.CID, result.Error)
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

func testCIDRetrievability(ctx context.Context, lassie *lassieWrapper, cidStr string, timeout time.Duration) RetrievabilityResult {
	result := RetrievabilityResult{
		CID: cidStr,
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