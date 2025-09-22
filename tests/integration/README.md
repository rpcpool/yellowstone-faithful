# Integration Tests for yellowstone-faithful

This directory contains comprehensive integration tests for the yellowstone-faithful project, covering all major functionality including gRPC endpoints, HTTP JSON-RPC endpoints, CAR file operations, and index generation/validation.

## Quick Start

### Prerequisites

1. **Build the binary:**
   ```bash
   make compile
   ```

2. **Install test dependencies (optional but recommended):**
   ```bash
   # For gRPC testing
   go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
   
   # For JSON formatting (optional)
   sudo apt-get install jq  # Ubuntu/Debian
   brew install jq          # macOS
   ```

3. **Run all tests:**
   ```bash
   ./run-all-tests.sh
   ```

## Test Structure

### Individual Test Scripts

| Script | Purpose | Key Features |
|--------|---------|-------------|
| `test-grpc.sh` | gRPC endpoint testing | Server startup, service discovery, method calls |
| `test-http.sh` | HTTP JSON-RPC testing | Standard Solana RPC methods, error handling |
| `test-car-ops.sh` | CAR file operations | dump-car, split, merge, validation |
| `test-index-ops.sh` | Index generation/validation | All index types, batch operations |
| `run-all-tests.sh` | Master test runner | Coordinated test execution, reporting |

### Test Categories

#### 1. gRPC Endpoint Tests (`test-grpc.sh`)
- **Server startup and shutdown**
- **Service discovery** (listing available services)
- **Method testing:**
  - GetVersion
  - GetBlock
  - GetTransaction
  - StreamBlocks
  - StreamTransactions
- **Authentication testing** (token-based)
- **Error handling and edge cases**

#### 2. HTTP JSON-RPC Tests (`test-http.sh`)
- **Server startup and basic connectivity**
- **Standard Solana JSON-RPC methods:**
  - getVersion
  - getHealth
  - getSlot
  - getBlock
  - getBlockHeight
  - getTransaction
  - getSignaturesForAddress
- **Error handling** (invalid methods, malformed requests)
- **Response validation**

#### 3. CAR File Operations (`test-car-ops.sh`)
- **CAR file analysis** (structure, format validation)
- **dump-car operations** (content extraction, JSON output)
- **CAR file splitting** (size-based partitioning)
- **CAR file merging** (combining multiple files)
- **Validation and integrity checks**

#### 4. Index Generation/Validation (`test-index-ops.sh`)
- **Index types tested:**
  - CID-to-offset mapping
  - Signature-to-CID mapping
  - Slot-to-CID mapping
  - Slot-to-blocktime mapping
  - Signature existence bitmap
  - GetSignaturesForAddress (GSFA) index
- **Batch operations** (generate/verify all indexes)
- **Index integrity verification**
- **Performance and size analysis**

## Usage Examples

### Run All Tests (Default)
```bash
# Sequential execution (recommended for debugging)
./run-all-tests.sh

# Parallel execution (faster)
./run-all-tests.sh --parallel

# With extended timeout
./run-all-tests.sh --timeout 600
```

### Run Individual Test Categories
```bash
# Test gRPC endpoints only
./test-grpc.sh

# Test HTTP JSON-RPC with custom port
./test-http.sh --port 8080

# Test CAR operations with custom fixtures
./test-car-ops.sh --fixtures /path/to/car/files

# Test index operations with custom log directory
./test-index-ops.sh --log-dir /tmp/index-tests
```

### GitHub Actions Integration
The tests are automatically run in CI/CD via the workflow at `.github/workflows/tests-integration.yml`.

## Test Data Requirements

### For Basic Testing (No Additional Data Needed)
- ✅ Command help and binary functionality
- ✅ Server startup/shutdown testing
- ✅ Basic connectivity and error handling
- ✅ Command structure validation

### For Comprehensive Testing (Requires Real Data)
To unlock full testing capabilities, provide:

1. **CAR Files** (`./fixtures/*.car`)
   - Real Solana blockchain data in CAR format
   - Various epochs and slot ranges
   - Both mainnet and testnet data recommended

2. **Configuration Files**
   - Epoch-specific configurations
   - Network settings
   - Index file paths

3. **Test Environment**
   - Sufficient disk space (indexes can be large)
   - Network connectivity for remote data fetching
   - Appropriate permissions for file operations

## Configuration Options

### Environment Variables
```bash
# Test execution
export RUN_PARALLEL=true          # Enable parallel test execution
export TIMEOUT_PER_TEST=300       # Timeout per test (seconds)
export CLEANUP_LOGS=true          # Cleanup old log files

# Service endpoints
export HTTP_PORT=7999             # HTTP JSON-RPC port
export GRPC_PORT=9999             # gRPC port

# Paths
export FAITHFUL_BINARY=/path/to/faithful-cli
```

### Command Line Options
```bash
# Master test runner options
./run-all-tests.sh --help

# Individual test options
./test-grpc.sh --help
./test-http.sh --help
./test-car-ops.sh --help
./test-index-ops.sh --help
```

## Output and Logging

### Log Files Location
All test logs are stored in `./test-logs/` with timestamped filenames:
- `integration-tests-master-YYYYMMDD_HHMMSS.log` - Master test log
- `grpc-test-YYYYMMDD_HHMMSS.log` - gRPC test details
- `http-test-YYYYMMDD_HHMMSS.log` - HTTP test details
- `car-ops-test-YYYYMMDD_HHMMSS.log` - CAR operations test details
- `index-ops-test-YYYYMMDD_HHMMSS.log` - Index operations test details

### Summary Report
After test completion, check:
- `./test-logs/integration-test-summary.txt` - Comprehensive summary
- Console output - Real-time test progress and results

### Log Levels
- 🔵 **INFO**: General information and progress
- 🟢 **SUCCESS**: Test passed successfully
- 🟡 **WARN**: Non-critical issues or expected failures
- 🔴 **ERROR**: Test failures or critical issues

## Understanding Test Results

### Expected Behaviors

#### ✅ Should Always Pass
- Binary help commands
- Server startup/shutdown
- Basic connectivity tests
- Command structure validation

#### ⚠️ May Fail Without Real Data
- Actual data retrieval (getBlock, getTransaction)
- Index generation from empty CAR files
- GSFA operations without transaction data
- Complex CAR operations

#### 🔴 Should Investigate
- Binary crashes or hangs
- Server startup failures
- Memory leaks or resource issues
- Command parsing errors

### Interpreting Results
```
DETAILED RESULTS:
==================
  Basic Binary     : PASSED   (2s)
  CAR Operations   : PASSED   (15s)
  Index Operations : FAILED   (45s)  # Expected without real CAR data
  HTTP JSON-RPC    : PASSED   (8s)
  gRPC Endpoints   : PASSED   (12s)
```

## Troubleshooting

### Common Issues

#### "Binary not found"
```bash
make compile
```

#### "Permission denied" 
```bash
chmod +x tests/integration/*.sh
```

#### "grpcurl not found"
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

#### Tests timeout
```bash
./run-all-tests.sh --timeout 600  # Increase to 10 minutes
```

#### Server startup fails
- Check port availability: `netstat -an | grep :7999`
- Review server logs in `./test-logs/`
- Verify binary functionality: `./bin/faithful-cli --help`

### Debug Mode
For detailed debugging, run tests with verbose output:
```bash
# Enable bash debug mode
bash -x ./test-grpc.sh

# Check specific test logs
tail -f ./test-logs/grpc-test-*.log
```

## Contributing

### Adding New Tests
1. Create test script following naming convention: `test-{category}.sh`
2. Use the logging functions for consistent output
3. Include help text and command-line options
4. Add timeout handling and cleanup
5. Update this README with test description

### Test Script Template
```bash
#!/bin/bash
set -euo pipefail

# Standard setup
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BINARY_PATH="${PROJECT_ROOT}/bin/faithful-cli"
LOG_DIR="${PROJECT_ROOT}/test-logs"

# Your test implementation here
```

### Best Practices
- Always use proper error handling (`set -euo pipefail`)
- Implement cleanup on script exit
- Use timeouts for long-running operations
- Provide helpful error messages and debugging information
- Test both success and failure scenarios

## CI/CD Integration

The integration tests are automatically executed in GitHub Actions on:
- Pull requests to `main` branch
- Pushes to `main` branch
- Manual workflow dispatch

See `.github/workflows/tests-integration.yml` for full CI/CD configuration.

## Performance Benchmarks

### Typical Test Durations (Without Real Data)
- Basic Binary: ~2-5 seconds
- CAR Operations: ~10-30 seconds
- Index Operations: ~30-60 seconds
- HTTP JSON-RPC: ~5-15 seconds
- gRPC Endpoints: ~10-20 seconds

### With Real Blockchain Data
- Index generation: ~5-30 minutes (depending on data size)
- CAR operations: ~1-10 minutes
- Server tests: ~30-120 seconds

## Support

For issues with integration tests:
1. Check the troubleshooting section above
2. Review test logs in `./test-logs/`
3. Run individual tests for targeted debugging
4. Open an issue with test logs and system information