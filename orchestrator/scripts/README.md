# Scripts

This directory contains utility scripts for the Old Faithful project.

## download_indexdir.sh (Recommended)

Downloads indexdir files for Solana epochs from the Old Faithful network using aria2 for fast parallel downloads.

### Usage

```bash
./scripts/download_indexdir.sh [start_epoch] [end_epoch] [output_dir] [base_url]
```

### Requirements

- `aria2c` (install with: `brew install aria2` / `apt-get install aria2`)
- `curl`

## download_indexdir.ts (Alternative)

TypeScript version that downloads indexdir files using Node.js fetch.

### Usage

```bash
tsx scripts/download_indexdir.ts [startEpoch] [endEpoch] [outputDir] [baseUrl]
```

### Parameters

- `startEpoch` (optional): Starting epoch number (default: 0)
- `endEpoch` (optional): Ending epoch number (default: same as startEpoch)
- `outputDir` (optional): Output directory for downloaded files (default: "./data/indexdirs")
- `baseUrl` (optional): Base URL for the Old Faithful files (default: "https://files.old-faithful.net")

### Examples

Download indexdir for epoch 0 only:
```bash
tsx scripts/download_indexdir.ts 0
```

Download indexdirs for epochs 0 through 5:
```bash
tsx scripts/download_indexdir.ts 0 5
```

Download to a custom directory:
```bash
tsx scripts/download_indexdir.ts 0 2 ./my-data/indexes
```

Download from a custom base URL:
```bash
tsx scripts/download_indexdir.ts 0 1 ./data https://my-custom-host.com
```

### What it does

1. For each epoch, fetches the CID from `{baseUrl}/{epoch}/epoch-{epoch}.cid`
2. Uses the CID to construct the indexdir URL: `{baseUrl}/{epoch}/epoch-{epoch}-{cid}-mainnet-gsfa.indexdir/`
3. Downloads these files from the indexdir:
   - `manifest`
   - `linked-log`
   - `pubkey-to-offset-and-size.index`
4. Saves files to: `{outputDir}/epoch-{epoch}/epoch-{epoch}-{cid}-mainnet-gsfa.indexdir/`

### Output Structure

```
data/indexdirs/
├── epoch-0/
│   └── epoch-0-{cid}-mainnet-gsfa.indexdir/
│       ├── manifest
│       ├── linked-log
│       └── pubkey-to-offset-and-size.index
├── epoch-1/
│   └── epoch-1-{cid}-mainnet-gsfa.indexdir/
│       ├── manifest
│       ├── linked-log
│       └── pubkey-to-offset-and-size.index
└── ...
```

### Error Handling

- If a CID cannot be fetched for an epoch, that epoch is skipped
- If individual files fail to download, the script continues with remaining files
- Progress and errors are logged to the console 