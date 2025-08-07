# Old Faithful Orchestrator

The Old Faithful Orchestrator manages storage locations for Solana epoch CAR files and their indexes.

## Features

- **Multi-Storage Support**: Scan and manage CAR files across multiple storage backends (local filesystem, HTTP/S3)
- **CAR Report Integration**: Uses the official CAR report from GitHub as the source of truth for available epochs
- **Index Tracking**: Tracks indexes (SlotToCid, SigToCid, etc.) separately from CAR files
- **Smart Probing**: Only probes for epochs known to exist according to the CAR report
- **Concurrent Operations**: Uses concurrent HTTP requests for efficient scanning
- **Caching**: Caches CAR report data to reduce GitHub API calls

## Building

```bash
cargo build --release
```

## Configuration

Create a TOML configuration file (see `config.example.toml`):

```toml
[[storage]]
type = "local"
path = "/var/lib/faithful/storage"
epoch_range = { start = 0, end = 100 }  # Optional

[[storage]]
type = "http"
url = "https://files.old-faithful.net"
timeout = "30s"
epoch_range = { start = 0, end = 800 }  # Recommended for HTTP
```

## Running

```bash
# Run with configuration file
cargo run --release -- --config config.toml

# With verbose logging
cargo run --release -- --config config.toml -v

# With very verbose logging
cargo run --release -- --config config.toml -vv
```

## Architecture

The orchestrator consists of several key components:

1. **Storage Backends**: Implementations for different storage types (local, HTTP)
2. **CAR Report**: Fetches and parses the official CAR report from GitHub
3. **Epoch State**: Tracks which epochs are available where and their completeness
4. **Storage Manager**: Coordinates scanning across multiple storage backends

## CAR File Structure

CAR files are expected to follow this directory structure:
```
{storagePath}/{epochNumber}/epoch-{epochNumber}.car
```

For example:
- `/storage/0/epoch-0.car`
- `https://files.old-faithful.net/827/epoch-827.car`

## Index Files

Index files are tracked separately and can exist in different storage locations than their corresponding CAR files. Supported index types:
- `slot-to-cid.index`
- `sig-to-cid.index`
- `cid-to-offset-and-size.index`
- `sig-exists.index`
- `gsfa.indexdir`

## Future Work

- Generate Old Faithful configuration files based on epoch availability
- Web interface for monitoring and maintenance
- Periodic scanning and automatic configuration updates