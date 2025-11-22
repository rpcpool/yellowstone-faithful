# Old Faithful gRPC Client Examples

This directory contains examples demonstrating all supported gRPC operations in the Old Faithful client SDK.

## Prerequisites

- Rust 1.75.0 or later
- Access to an Old Faithful gRPC server
- Optional: Authentication token (x-token) if required by your server

## Running Examples

All examples follow a consistent command-line interface:

```bash
cargo run --example <example_name> -- --endpoint <url> [--x-token <token>] [additional-args]
```

**Note:** The `x-token` header is only required if you have enabled it in the configuration or you are accessing a hosted Old Faithful version that requires it.

## Unary Examples

These examples demonstrate single request-response RPC calls.

### 1. Get Version (GetVersion)

Get the Old Faithful server version.

**Example Command:**
```bash
cargo run --example get_version -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token>
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)

---

### 2. Get Block Time (GetBlockTime)

Get the Unix timestamp for a specific slot.

**Example Command:**
```bash
cargo run --example get_block_time -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --slot 307152000
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--slot` or `-s`: Slot number (required)

---

### 3. Get Block (GetBlock)

Fetch a complete block by slot number.

**Example Command:**
```bash
cargo run --example get_block -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --slot 307152000
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--slot` or `-s`: Slot number (required)

---

### 4. Get Transaction (GetTransaction)

Fetch a transaction by its signature.

**Example Command:**
```bash
cargo run --example get_transaction -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --signature GbXoI+D7hhgeiUwovUhtaxog6zsxFcd5PKfhQM85GR6+NqmiFmQDf9cCCVj8BRj+DR1RvgR/E2E/ckbSGuQKCg==
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--signature` or `-s`: Transaction signature in base58 format (required)

---

## Streaming Examples

These examples demonstrate streaming data from the server. Press Ctrl+C to stop streaming.

### 5. Stream Blocks (StreamBlocks)

Stream blocks within a slot range.

**Example Command (No Filter):**
```bash
cargo run --example stream_blocks -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --start-slot 307152000 \
  --end-slot 307152010 \
  --limit 50
```

**Example Command (With Account Filter):**
```bash
cargo run --example stream_blocks -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --start-slot 307152000 \
  --end-slot 307152010 \
  --account-include Vote111111111111111111111111111111111111111 \
  --limit 50
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--start-slot`: Starting slot (inclusive, required)
- `--end-slot`: Ending slot (inclusive, optional)
- `--limit` or `-l`: Maximum blocks to receive (default: 100)
- `--account-include`: Filter by account (can be specified multiple times)

Blocks are returned when any transaction (including loaded accounts) mentions one of the provided accounts.

---

### 6. Stream Transactions (StreamTransactions)

Stream transactions within a slot range with advanced filtering.

**Example Command (No Filters):**
```bash
cargo run --example stream_transactions -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --start-slot 307152000 \
  --end-slot 307152010 \
  --limit 50
```

**Example Command (With Filters):**
```bash
cargo run --example stream_transactions -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --start-slot 307152000 \
  --end-slot 307152010 \
  --no-vote \
  --exclude-failed \
  --limit 50
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--start-slot`: Starting slot (inclusive, required)
- `--end-slot`: Ending slot (inclusive, optional)
- `--limit` or `-l`: Maximum transactions to receive (default: 100)
- `--no-vote`: Exclude vote transactions
- `--exclude-failed`: Skip failed transactions
- `--account-include`: Include transactions with these accounts (repeatable)
- `--account-exclude`: Exclude transactions with these accounts (repeatable)
- `--account-required`: Require these accounts in transactions (repeatable)

Filters are combined: transactions must satisfy every provided filter (non-vote if `--no-vote`, not failed if `--exclude-failed`, matches any `--account-include`, avoids any `--account-exclude`, and includes all `--account-required`).

---

### 7. Batch Get (Get - Bidirectional Streaming)

Efficiently fetch multiple blocks in a single batched request.

**Example Command:**
```bash
cargo run --example batch_get -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --x-token <redacted-token> \
  --slots 307152000,307152001,307152002
```

**Arguments:**
- `--endpoint` or `-e`: gRPC server URL (required)
- `--x-token` or `-t`: Authentication token (optional)
- `--slots`: Comma-separated list of slot numbers (required, e.g., `307152000,307152001,307152002`)

---

## Logging and Debugging

All examples support the `RUST_LOG` environment variable for detailed logging:

```bash
# Info level (default)
RUST_LOG=info cargo run --example get_block -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --slot 307152000

# Debug level (detailed)
RUST_LOG=debug cargo run --example stream_blocks -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443 \
  --start-slot 307152000

# Trace level (very verbose)
RUST_LOG=trace cargo run --example get_version -- \
  --endpoint https://customer-endpoint-2608.mainnet.rpcpool.com:443
```

## Notes

- **Protocol File:** All gRPC calls use the proto file located at `../old-faithful-proto/proto/old-faithful.proto`
- **Authentication:** The `x-token` header is only required if enabled in your server configuration
- **Endpoints:** Replace `customer-endpoint-2608.mainnet.rpcpool.com:443` with your actual Old Faithful server endpoint
- **Local Development:** For local testing, use `http://localhost:8889` as the endpoint
- **No Feature Flags Required:** As of the latest version, `--features grpc` is no longer needed; gRPC support is built-in

## Additional Resources

- [Old Faithful Documentation](https://docs.old-faithful.net)
- [gRPC Proto Definition](../old-faithful-proto/proto/old-faithful.proto)
- [SDK Source Code](../src)
