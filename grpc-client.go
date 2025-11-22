package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// newCmd_TestGRPC returns a command for testing the gRPC server
func newCmd_TestGRPC() *cli.Command {
	return &cli.Command{
		Name:        "test-grpc",
		Usage:       "Test the gRPC server",
		Description: "Connect to a Yellowstone Faithful gRPC server and test various methods",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server",
				Aliases:  []string{"s"},
				Usage:    "The server address in the format host:port",
				Value:    "localhost:50051",
				Required: false,
			},
			&cli.Uint64Flag{
				Name:     "start-slot",
				Usage:    "Start slot for streaming",
				Value:    307152000,
				Required: false,
			},
			&cli.Uint64Flag{
				Name:     "end-slot",
				Usage:    "End slot for streaming (optional)",
				Value:    0,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "token",
				Usage:    "Authentication token (if required)",
				Value:    "",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "tls",
				Usage:    "Use TLS for connection",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "vote",
				Usage:    "Include vote transactions",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "failed",
				Usage:    "Include failed transactions",
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "allow-unfiltered-streams",
				Usage:    "Allow empty transaction filters (server must be started with --allow-unfiltered-streams)",
				Value:    false,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "account",
				Usage:    "Account to filter by (comma separated for multiple)",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "method",
				Usage:    "Method to test (version, block, transaction, stream-transactions)",
				Value:    "version",
				Required: false,
			},
			&cli.Uint64Flag{
				Name:     "slot",
				Usage:    "Slot for block or transaction methods",
				Value:    0,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "signature",
				Usage:    "Transaction signature (base58 encoded) for transaction method",
				Value:    "",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "verbose",
				Usage:    "Enable verbose output",
				Value:    false,
				Required: false,
			},
			&cli.IntFlag{
				Name:     "timeout",
				Usage:    "Timeout in seconds for the operation",
				Value:    300, // 5 minutes default
				Required: false,
			},
		},
		Action: func(cctx *cli.Context) error {
			ctx := cctx.Context

			serverAddr := cctx.String("server")
			startSlot := cctx.Uint64("start-slot")
			endSlot := cctx.Uint64("end-slot")
			authToken := cctx.String("token")
			useTLS := cctx.Bool("tls")
			includeVote := cctx.Bool("vote")
			includeFailed := cctx.Bool("failed")
			allowUnfiltered := cctx.Bool("allow-unfiltered-streams")
			accountsStr := cctx.String("account")
			method := cctx.String("method")
			slot := cctx.Uint64("slot")
			signatureStr := cctx.String("signature")
			verbose := cctx.Bool("verbose")
			timeoutSec := cctx.Int("timeout")

			// Set timeout for the operation
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()

			// Parse accounts
			var accounts []string
			if accountsStr != "" {
				accounts = strings.Split(accountsStr, ",")
			}

			// Create client
			client, err := newFaithfulClient(serverAddr, useTLS)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.close()

			// Run the requested method
			switch method {
			case "version":
				return testVersion(timeoutCtx, client, authToken, verbose)
			case "block":
				if slot == 0 {
					return fmt.Errorf("slot is required for block method")
				}
				return testBlock(timeoutCtx, client, authToken, slot, verbose)
			case "transaction":
				if signatureStr == "" {
					return fmt.Errorf("signature is required for transaction method")
				}
				return testTransaction(timeoutCtx, client, authToken, signatureStr, verbose)
			case "stream-transactions":
				return testStreamTransactions(timeoutCtx, client, authToken, startSlot, endSlot, includeVote, includeFailed, accounts, verbose, allowUnfiltered)
			default:
				return fmt.Errorf("unknown method: %s", method)
			}
		},
	}
}

// faithfulClient is a wrapper around the Faithful gRPC client
type faithfulClient struct {
	conn   *grpc.ClientConn
	client old_faithful_grpc.OldFaithfulClient
}

// Create a new Faithful client
func newFaithfulClient(serverAddr string, useTLS bool) (*faithfulClient, error) {
	var opts []grpc.DialOption

	// Configure TLS
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Set maximum message size (100MB)
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(100*MiB),
		grpc.MaxCallSendMsgSize(100*MiB),
	))

	// Add client-side keepalive parameters
	keepaliveParams := keepalive.ClientParameters{
		Time:                60 * time.Second, // Ping server if idle for 30 seconds
		Timeout:             30 * time.Second, // Wait 10 seconds for ping ack
		PermitWithoutStream: true,             // Allow pings even without active streams
	}
	opts = append(opts, grpc.WithKeepaliveParams(keepaliveParams))

	// Connect to the server with a timeout
	dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, serverAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", serverAddr, err)
	}

	return &faithfulClient{
		conn:   conn,
		client: old_faithful_grpc.NewOldFaithfulClient(conn),
	}, nil
}

// Close the client connection
func (c *faithfulClient) close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// addAuthToken adds authentication token to the context if provided
func addAuthToken(ctx context.Context, authToken string) context.Context {
	if authToken == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-token", authToken)
}

// Test the GetVersion method
func testVersion(ctx context.Context, client *faithfulClient, authToken string, verbose bool) error {
	ctx = addAuthToken(ctx, authToken)

	startTime := time.Now()
	resp, err := client.client.GetVersion(ctx, &old_faithful_grpc.VersionRequest{})
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	klog.Infof("Server version: %s (request took %v)", resp.GetVersion(), duration)
	return nil
}

// Test the GetBlock method
func testBlock(ctx context.Context, client *faithfulClient, authToken string, slot uint64, verbose bool) error {
	ctx = addAuthToken(ctx, authToken)

	startTime := time.Now()
	resp, err := client.client.GetBlock(ctx, &old_faithful_grpc.BlockRequest{Slot: slot})
	duration := time.Since(startTime)

	if err != nil {
		if s, ok := status.FromError(err); ok {
			return fmt.Errorf("failed to get block: %s (code: %d)", s.Message(), s.Code())
		}
		return fmt.Errorf("failed to get block: %w", err)
	}

	klog.Infof("Block at slot %d fetched in %v", resp.Slot, duration)
	klog.Infof("Block time: %d", resp.BlockTime)
	klog.Infof("Parent slot: %d", resp.ParentSlot)
	klog.Infof("Transactions count: %d", len(resp.Transactions))

	if verbose && len(resp.Transactions) > 0 {
		klog.Infof("First 5 transactions (or fewer if less available):")
		for i, tx := range resp.Transactions {
			if i >= 5 {
				break
			}
			klog.Infof("  Transaction %d:", i)
			if tx.Index != nil {
				klog.Infof("    Index: %d", *tx.Index)
			}
			// You can add more transaction details here if needed
		}
	}

	return nil
}

// Test the GetTransaction method
func testTransaction(ctx context.Context, client *faithfulClient, authToken string, signatureStr string, verbose bool) error {
	ctx = addAuthToken(ctx, authToken)

	// Decode the signature from base58
	signature, err := solana.SignatureFromBase58(signatureStr)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	startTime := time.Now()
	resp, err := client.client.GetTransaction(ctx, &old_faithful_grpc.TransactionRequest{Signature: signature[:]})
	duration := time.Since(startTime)

	if err != nil {
		if s, ok := status.FromError(err); ok {
			return fmt.Errorf("failed to get transaction: %s (code: %d)", s.Message(), s.Code())
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	klog.Infof("Transaction fetched in %v", duration)
	klog.Infof("Slot: %d", resp.Slot)
	klog.Infof("Block time: %d", resp.BlockTime)
	if resp.Index != nil {
		klog.Infof("Transaction index: %d", *resp.Index)
	}

	if verbose {
		klog.Infof("Transaction details:")
		klog.Infof("  Transaction data size: %d bytes", len(resp.Transaction.Transaction))
		klog.Infof("  Meta data size: %d bytes", len(resp.Transaction.Meta))
		// You can add more transaction parsing here if needed
	}

	return nil
}

// Test the StreamTransactions method
func testStreamTransactions(
	ctx context.Context,
	client *faithfulClient,
	authToken string,
	startSlot uint64,
	endSlot uint64,
	includeVote bool,
	includeFailed bool,
	accounts []string,
	verbose bool,
	allowUnfiltered bool,
) error {
	ctx = addAuthToken(ctx, authToken)

	// Prepare the request
	request := &old_faithful_grpc.StreamTransactionsRequest{StartSlot: startSlot}

	if !allowUnfiltered {
		request.Filter = &old_faithful_grpc.StreamTransactionsFilter{
			Vote:   &includeVote,
			Failed: &includeFailed,
		}
	}

	if endSlot > startSlot {
		request.EndSlot = &endSlot
	}

	// Add account filters if provided
	if len(accounts) > 0 && !allowUnfiltered {
		request.Filter.AccountInclude = accounts
		klog.Infof("Filtering for accounts: %v", accounts)
	}

	if allowUnfiltered {
		klog.Infof("Unfiltered streaming enabled: sending request without filters (requires server --allow-unfiltered-streams)")
	}

	// Start streaming
	klog.Infof("Starting transaction stream from slot %d", startSlot)
	if endSlot > startSlot {
		klog.Infof("  to slot %d", endSlot)
	} else {
		klog.Infof("  with no end slot specified (using server default)")
	}
	klog.Infof("Including vote transactions: %v", includeVote)
	klog.Infof("Including failed transactions: %v", includeFailed)

	startTime := time.Now()
	stream, err := client.client.StreamTransactions(ctx, request)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			return fmt.Errorf("failed to start streaming: %s (code: %d)", s.Message(), s.Code())
		}
		return fmt.Errorf("failed to start streaming: %w", err)
	}

	txCount := 0
	slotToTxCount := make(map[uint64]int)
	lastLogTime := time.Now()
	logInterval := 5 * time.Second

	for {
		tx, err := stream.Recv()
		if err == io.EOF {
			duration := time.Since(startTime)
			klog.Infof("Stream completed: received %d transactions in %v", txCount, duration)
			break
		}
		if err != nil {
			if s, ok := status.FromError(err); ok {
				return fmt.Errorf("error while receiving: %s (code: %d)", s.Message(), s.Code())
			}
			return fmt.Errorf("error while receiving: %w", err)
		}

		txCount++
		slotToTxCount[tx.Slot]++

		// Log progress at regular intervals
		if time.Since(lastLogTime) > logInterval {
			elapsed := time.Since(startTime)
			rate := float64(txCount) / elapsed.Seconds()
			klog.Infof("Received %d transactions so far (%.1f tx/sec)...", txCount, rate)
			lastLogTime = time.Now()
		}

		// Print the first few transactions in detail if verbose
		if verbose && txCount <= 5 {
			klog.Infof("Transaction #%d at slot %d", txCount, tx.Slot)
			if tx.Transaction.Index != nil {
				klog.Infof("  Index: %d", *tx.Transaction.Index)
			}
			klog.Infof("  Block time: %d", tx.BlockTime)
			klog.Infof("  Transaction data size: %d bytes", len(tx.Transaction.Transaction))
			klog.Infof("  Meta data size: %d bytes", len(tx.Transaction.Meta))
		}
	}

	duration := time.Since(startTime)
	klog.Infof("Stream ended successfully. Received %d transactions in %v", txCount, duration)
	klog.Infof("Average rate: %.1f transactions per second", float64(txCount)/duration.Seconds())

	if verbose {
		klog.Infof("Transactions by slot:")
		// Get sorted slot list for consistent output
		slots := make([]uint64, 0, len(slotToTxCount))
		for slot := range slotToTxCount {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })

		// Print transaction count by slot
		for _, slot := range slots {
			klog.Infof("  Slot %d: %d transactions", slot, slotToTxCount[slot])
		}
	}

	return nil
}
