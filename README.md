# :hotsprings: Project Yellowstone: Old Faithful

:hotsprings: Project Yellowstone: Old Faithful is the project to make all of Solana's history accessible, content addressable and available via a variety of means. The goal of this project is to completely replace the Bigtable dependency for Solana history access with a self-hostable, decentralized history archive that is usable for infrastructure providers, individual Solana users, explorers, indexers, and anyone else in need of historical access.

This is currently in RFC stage, which means that it is not intended for production use and that there may be breaking changes to the format, the CLI utilities or any other details related to the project.

For more documentation, please visit [https://docs.old-faithful.net](https://docs.old-faithful.net).

> ❗ **Request for comment**: We are currently looking for feedback and comments on the new archival format and the RPC server setup. We invite all interested parties to test the archival access and open issues on this repo with questions/comments/requests for improvements.

## Usage

This repo provides the `faithful-cli` command line interface. This tool allows you to interact with the Old Faithful archive as stored on disk (if you have made a local copy), from old-faithful.net or directly from Filecoin. The CLI provides an RPC server that supports:

  - getBlock
  - getTransaction
  - getSignaturesForAddress
  - getBlockTime
  - getGenesisHash (for epoch 0)
  - getFirstAvailableBlock
  - getSlot
  - getVersion

## RPC server

The RPC server is available via the `faithful-cli rpc` command. 

The command accepts a list of [epoch config files](#epoch-configuration-files) and dirs as arguments. Each config file is specific for an epoch and provides the location of the block/transaction data and the indexes for that epoch. The indexes are used to map Solana block numbers, transaction signatures and addresses to their respective CIDs. The indexes are generated from the CAR file and can be generated via the `faithful-cli index` command (see [Index generation](#index-generation)).

It supports the following flags:

- `--listen`: The address to listen on for JSON-RPC requests, e.g. `--listen=:8888`
- `--grpc-listen`: The address to listen on for gRPC requests, e.g. `--grpc-listen=:8889`
- `--include`: You can specify one or more (reuse the same flag multiple times) glob patterns to include files or dirs that match them, e.g. `--include=/path/epoch-*.yml`.
- `--exclude`: You can specify one or more (reuse the same flag multiple times) glob patterns to exclude files or dirs that match them, e.g. `--exclude=/something-*/epoch-*.yml`.
- `--debug`: Enable debug logging.
- `--proxy`: Proxy requests to a downstream RPC server if the data can't be found in the archive, e.g. `--proxy=/path/to/my-proxy-config.yml`. See [RPC server proxying](#rpc-server-proxying) for more details.
- `--gsfa-only-signatures`: When enabled, the RPC server will only return signatures for getSignaturesForAddress requests instead of the full transaction data.
- `--watch`: When specified, all the provided epoch files and dirs will be watched for changes and the RPC server will automatically reload the data when changes are detected. Usage: `--watch` (boolean flag). This is useful when you want to provide just a folder and then add new epochs to it without having to restart the server.
- `--epoch-load-concurrency=2`: How many epochs to load in parallel when starting the RPC server. Defaults to number of CPUs. This is useful when you have a lot of epochs and want to speed up the initial load time.
- `--epoch-search-concurrency=3`: How many epochs to search in parallel when looking for data. Defaults to 3. This is useful for performance tuning when serving requests across multiple epochs.
- `--tier1-epoch-limit=10`: Last N epochs to search first in tiered search. Defaults to 10. For 1000 epochs, this searches epochs 991-998 (last-3 to last-10).
- `--tier2-epoch-limit=50`: Last N epochs to search second in tiered search. Defaults to 50. For 1000 epochs, this searches epochs 951-990 (last-10 to last-50).
- `--max-cache=<megabytes>`: How much memory to use for caching. Defaults to 0 (no limit). This is useful when you want to limit the memory usage of the RPC server.
- `--filecoin-api-address`: Filecoin API endpoint address for direct Filecoin retrievals, e.g. `--filecoin-api-address=https://api.node.glif.io`

NOTES:

- By default, the RPC server doesn't support the `jsonParsed` format. You need to build the RPC server with the `make jsonParsed-linux` flag to enable this.

## Epoch configuration files

To run a Faithful RPC server you need to specify configuration files for the epoch(s) you want to host. An epoch config file looks like this:

```yml
epoch: 0 # epoch number (required)
version: 1 # version number (required)
data: # data section (required)
  car:
    # Source the data from a CAR file (car-mode).
    # The URI can be a local filepath or an HTTP url.
    # This makes the indexes.cid_to_offset_and_size required.
    # If you are running in filecoin-mode, you can omit the car section entirely.
    uri: /media/runner/solana/cars/epoch-0.car
    # Alternative: Source CAR data from Filecoin pieces (advanced)
    # from_pieces:
    #   metadata:
    #     uri: /path/to/metadata.json
    #   deals:
    #     uri: /path/to/deals.json
    #   piece_to_uri:
    #     bafk2bzaceabc123...: # piece CID
    #       uri: "https://example.com/piece"
  filecoin:
    # filecoin-mode section: source the data directly from filecoin.
    # If you are running in car-mode, you can omit this section.
    # if enable=true, then the data will be sourced from filecoin.
    # if enable=false, then the data will be sourced from a CAR file (see 'car' section above).
    enable: false
    # Optional: Specify root CID and providers for Filecoin retrievals
    # root_cid: bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq
    # providers:
    #   - "/ip4/1.2.3.4/tcp/1234/p2p/12D3KooWExample..."
    #   - "/ip4/5.6.7.8/tcp/5678/p2p/12D3KooWAnother..."
genesis: # genesis section (required for epoch 0 only)
  # Local filepath to the genesis tarball.
  # You can download the genesis tarball from
  # wget https://api.mainnet-beta.solana.com/genesis.tar.bz2
  uri: /media/runner/solana/genesis.tar.bz2
indexes: # indexes section (required)
  cid_to_offset_and_size:
    # Required when using a CAR file; you can provide either a local filepath or a HTTP url.
    # Not used when running in filecoin-mode.
    uri: '/media/runner/solana/indexes/epoch-0/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-mainnet-cid-to-offset-and-size.index'
  slot_to_cid:
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-0/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-mainnet-slot-to-cid.index'
  sig_to_cid:
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-0/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-mainnet-sig-to-cid.index'
  sig_exists:
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-0/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-mainnet-sig-exists.index'
  slot_to_blocktime:
    # required for getBlockTime support; you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-0/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-mainnet-slot-to-blocktime.index'
  gsfa: # getSignaturesForAddress index
    # optional; must be a local directory path.
    uri: '/media/runner/solana/indexes/epoch-0/gsfa/epoch-0-bafyreifljyxj55v6jycjf2y7tdibwwwqx75eqf5mn2thip2sswyc536zqq-gsfa.indexdir'
```

NOTES:

- The `uri` parameter supports both HTTP URIs as well as file based ones (where not specified otherwise).
- If you specify an HTTP URI, you need to make sure that the url supports HTTP Range requests. S3 or similar APIs will support this.

## RPC server proxying

The RPC server supports proxying requests to a downstream RPC server when data can't be found in the archive. This is configured via the `--proxy` flag which takes a path to a proxy configuration file.

### Proxy configuration file format

```yaml
target: "https://api.mainnet-beta.solana.com"
headers:
  "User-Agent": "Old-Faithful-RPC-Server"
  "Custom-Header": "custom-value"
proxyFailedRequests: true
```

- `target`: The downstream RPC server URL to proxy requests to
- `headers`: Optional HTTP headers to include in proxied requests
- `proxyFailedRequests`: Whether to proxy requests that fail (default: false)

## Index generation

To run the old-faithful RPC server you need to generate indexes for the CAR files. You can do this via the `faithful-cli index` command.

- `faithful-cli index all <car-file> <output-dir>`: Generate all **required** indexes for a CAR file.
- `faithful-cli index gsfa <car-file> <output-dir>`: Generate the gsfa index for a CAR file.

NOTES:

- You need to have the CAR file available locally.
- The `cid_to_offset_and_size` index has an older version, which you can specify with `cid_to_offset` instead of `cid_to_offset_and_size`.

Flags:

- `--tmp-dir=/path/to/tmp/dir`: Where to store temporary files. Defaults to the system temp dir. (optional)
- `--verify`: Verify the indexes after generation. (optional)
- `--network=<network>`: Which network to use for the gsfa index. Defaults to `mainnet` (other options: `testnet`, `devnet`). (optional)

### Filecoin fetch via CID

If you already know the CID of the data you are looking for you can fetch it via `faithful-cli fetch <cid>`. This requires no further indexes and can also be used to recursively fetch data for example for an epoch. To avoid fetching the full dataset for an epoch (100s of GB) you probably want to pass the parameter `--dag-scope=block` to fetch only the particular CID entity that you are interested in.

### Production RPC server

The production RPC server is accessible via `faithful-cli rpc`. More documentation on this can be found at [https://old-faithful.net](https://old-faithful.net).

### Limitations

The (deprecated) testing server (`rpc-server-car` and `rpc-server-filecoin`) only supports single Epoch access. The production server supports handling a full set of epochs.

Filecoin retrievals without a CDN can also be slow. We are working on integration with Filecoin CDNs and other caching solutions. Fastest retrievals will happen if you service from local disk.

## Technical overview

The core of the project is history archives in Content Addressable format ([overview](https://old.web3.storage/docs/how-tos/work-with-car-files/), [specs](https://ipld.io/specs/transport/car/carv1/)). These represent a verifiable, immutable view of the Solana history. The CAR files that this project generates follows a [schema](https://github.com/rpcpool/yellowstone-faithful/blob/main/ledger.ipldsch) specifically developed for Solana's historical archives.

The content addressable nature means that each epoch, block, transaction and shredding is uniquely identified by a content hash. By knowing this content hash a user will be able to retrieve a specific object of interest in a trustless manner, i.e. retrieve an object verifiably from a non-trusted source. Retrievals can be made via IPFS, the Filecoin network, or even by hosting the CAR files yourself on disk, a ceph cluster, S3, you name it.

### Indexes

Indexes will be needed to map Solana's block numbers, transaction signatures and addresses to their respective CIDs. These indexes will be developed as part of this project. There are several kinds of indexes that the Old Faithful index generation can provide:

 - slot-to-cid: Lookup a CID based on a slot number
 - tx-to-cid: Lookup a CID based on a transaction signature
 - gsfa: An index mapping Solana addresses to a list of singatures
 - cid-to-offset-and-size: Index for a specific CAR file, used by the local rpc server (see above) to find CIDs in a car file
 - sig-exists: An index to speed up lookups for signatures when using multiepoch support in the production server
 - slot-to-blocktime: Maps slot numbers to their block timestamps, required for getBlockTime RPC method support

### Archive access

The archive is currently under development. There will be two main ways to access the archive during its development phase:

1. Via Filecoin: Through collaboration with Protocol Labs and a Filplus allocation we are uploading all historical data to Filecoin. From here, any user can access the full histortical archive verifiably and freely from the Filecoin network. This is helpful as a way to test retreivals and fetch individual transactions.
2. Bulk transfer: If you want to start testing full epoch archives, we can provide you with access to a storage bucket from where you can pull the epoch archives.

The data that you will need to be able to run a local RPC server is:

  1) the Epoch car file containing all the data for that epoch
  2) the slot-to-cid index for that epoch
  3) the tx-to-cid index for that epoch
  4) the cid-to-offset-and-size index for that epoch car file
  5) the sig-exists index for that epoch (optional, but important to speed up multiepoch fetches)
  6) the slot-to-blocktime index for that epoch (required for getBlockTime support)
  7) Optionally (if you want to support getSignaturesForAddress): the gsfa index

The epoch car file can be generated from a rocksdb snapshot from a running validator or from one of the archives provided by the Solana foundation or third parties like Triton. You can also download a pre-generated Epoch car file either from Filecoin itself or via the download URLs provided by Triton.

If you have an epoch car file you can generate all the other indexes, see below notes about index generation. You can also download indexes from a third party source or (soon) retrieve them via Filecoin.

### Data tooling

The primary data preparation tooling used in this project is based in the `radiance` tool developed by Jump's Firedancer team. It is rapidely developing, and active development for this project is currently based out of this repository and branch: [Radiance Triton](https://github.com/gagliardetto/radiance-triton/).

The radiance tool utilises the rocksdb snapshots that have been generated by [Warehouse](https://github.com/solana-labs/solana-bigtable) nodes. From these snapshots a CAR file per epoch is generated. This CAR file then needs to be processed by Filecoin tools such as [split-and-commp](https://github.com/anjor/go-fil-dataprep/) which generates the details needed for making a Filecoin deal.

Currently, this tool is being tested from the following warehouse archives:
  - Solana Foundation (public)
    -  gs://mainnet-beta-ledger-us-ny5
    -  gs://mainnet-beta-ledger-europe-fr2
    -  gs://mainnet-beta-ledger-asia-sg1
  - Triton One (private)

If you have warehouse nodes generating rocksdb archive snapshots, please contact lk@triton.one (even if they can't be made publicly available). We would like to have you generate CAR files for verification purposes.

## Data preparation

Using the rocksdb archives, the Radiance tool can be used to generate one CAR file per epoch. This CAR file is then made available via storage providers such as Filecoin and private storage buckets.

CAR file generation produces a CAR containing a DAG. This DAG is reproducible and follows the structure of Epoch -> Block -> Transaction see [schema](https://github.com/rpcpool/yellowstone-faithful/blob/main/ledger.ipldsch). The CAR file generation is deterministic, so even if you use different rocksdb source snapshots you should end up with the same CAR output. This allows comparison between different providers.

The data generation flow is illustrated below:

![radiance datagen flow](https://github.com/rpcpool/yellowstone-faithful/assets/5172293/65f6dd75-189b-4253-968a-e81bfe6c130f)

## Generating an epoch car file

Once you have downloaded rocksdb ledger archives you can run the Radiance tool to generate a car file for an epoch. Make sure you have all the slots available in rocksdb ledger archive for the epoch. You may need to download multiple ledger snapshots in order to have a full set of slots available. Once you know you have a rocksdb that covers all the slots for the epoch run the radiance tool like follows:

```
radiance car create 107 --db=46223992/rocksdb --out=/storage/car/epoch-107.car
```

This will produce a car file called epoch-107.car containing all the blocks and transactions for that epoch.

## Index generation

Once the radiance tooling has been used to prepare a car file (or if you have downloaded a car file externally) you can generate indexes from this car file by using the `faithful-cli`:

```bash
NAME:
   faithful CLI index - Create various kinds of indexes for CAR files.

USAGE:
   faithful CLI index command [command options] [arguments...]

DESCRIPTION:
   Create various kinds of indexes for CAR files.

COMMANDS:
   cid-to-offset  
   slot-to-cid    
   sig-to-cid     
   all            Create all the necessary indexes for a Solana epoch.
   gsfa           
   sig-exists     
   help, h        Shows a list of commands or help for one command

OPTIONS:
   --help, -h  show help
```

For example, to generate the three indexes cid-to-offset-and-size, slot-to-cid, sig-to-cid, sig-exists you would run:

```bash
faithful-cli index all epoch-107.car /storage/indexes/epoch-107
```

This would generate the indexes in `/storage/indexes/epoch-107` for epoch-107.

## Contributing

We are currently requesting contributions from the community in testing this tool for retrievals and for generating data. We also request input on the IPLD Schema and data format. Proposals, bug reports, questions, help requests etc. can be reported via issues on this repo.

## Contact

This project is currently managed by [Triton One](https://triton.one/). If you want more information contact us via [Telegram](https://t.me/+K0ONdq7fE4s0Mjdl).

## Acknowledgements

The originator of this project was [Richard Patel](https://github.com/terorie) ([Twitter](https://twitter.com/fd_ripatel)).

[@immaterial.ink](https://github.com/gagliardetto) ([Twitter](https://twitter.com/immaterial_ink)) is currently the lead dev on this project at Triton One.

This work has been supported greatly by Protocol Labs (special shout out to [anjor](https://github.com/anjor) ([Twitter](https://twitter.com/__anjor)) for all the guidance in Filecoin land to us Solana locals).

The Solana Foundation is funding this effort through a project grant.

[Solana.fm](https://solana.fm/) was, alongside Richard and Triton, one of the initiators of this project.

Also thanks to all RPC providers and others who have (and are) providing input to and support for this process.
