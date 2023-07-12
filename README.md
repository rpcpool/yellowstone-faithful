# Project Yellowstone: Old Faithful

Project Yellowstone: Old Faithful is the project to make all of Solana's history accessible, content addressable and available via a variety of means. The goal of this project is to completely replace the Bigtable dependency for Solana history access with a self-hostable, decentralized history archive that is usable for infrastructure providers, individual Solana users, explorers, indexers, and anyone else in need of historical access.

This is currently in RFC stage, which means that it is not intended for production use and that there may be breaking changes to the format, the CLI utilities or any other details related to the project. 

## Usage

This repo provides the `faithful-cli` command line interface. This tool allows you to interact with the Old Faithful archive as stored on disk (if you have made a local copy) or directly from Filecoin. The CLI provides an RPC server that supports;

  - getBlock
  - getTransaction 
  - getSignaturesForAddress

### Local RPC server

If you have a local copy of a CAR archive and the indexes and run a RPC server servicing data from them. For example:

```
/usr/local/bin/faithful-cli rpc-server-car \
    --listen $PORT \
    epoch-455.car \
    epoch-455.car.*.cid-to-offset.index \
    epoch-455.car.*.slot-to-cid.index \
    epoch-455.car.*.sig-to-cid.index \
    epoch-455.car-*-gsfa-index
```

To get URLs for downloading the local files, please open a data cloning request as an issue on this repo with your contact details and we will help organise a cloning setup for you. 

### Filecoin RPC server

The filecoin RPC server allows provide getBlock, getTransaction and getSignaturesForAddress powered by Filecoin. This requires access to indexes. The indexes allow you to lookup transaction signatures, block numbers and addresses and map them to Filecoin CIDs.

You can run it in the following way:

```
faithful-cli rpc-server-filecoin -config 455.yml
```

The config file points faithful to the location of the required indexes (`455.yaml`):
```
indexes:
  slot_to_cid: './epoch-455.car.bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y.slot-to-cid.index'
  sig_to_cid: './epoch-455.car.bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y.sig-to-cid.index'
  gsfa: './epoch-455.car.gsfa.index'
```

Due to latency in fetching signatures, typically the getSignaturesForAddress index needs to be stored in a local directory, but the other indexes can be fetched via HTTP or via local file system access. If you provide a URL, you need to make sure that the url supports HTTP Range requests. S3 or similar APIs will support this. 

There is a mode in which you can use a remote gSFA index, which limits it to only return signatures and not additional transaction meta data. In this mode, you can use a remote gSFA index. To enable this mode run faithful-cli in the following way:

```
faithful-cli rpc-server-filecoin -config 455.yml -gsfa-only-signatures=true
```

### Filecoin fetch via CID

If you already know the CID of the data you are looking for you can fetch it via `faithful-cli fetch <cid>`. This requires no further indexes and can also be used to recursively fetch data for example for an epoch. 

### Limitations

Currently the CLI is only designed to service one epoch at a time. Support for multiple epochs is incoming. We will also soon support fetching indexes from Filecoin as well, currently those are available via S3 together with the raw car files. For full access, please contact help@triton.one. 

Filecoin retrievals without a CDN can also be slow. We are working on integration with Filecoin CDNs and other caching solutions. Fastest retrievals will happen if you service from local disk. 

## Technical overview

The core of the project is history archives in Content Addressable format ([overview](https://web3.storage/docs/how-tos/work-with-car-files/), [specs](https://ipld.io/specs/transport/car/carv1/)). These represent a verifiable, immutable view of the Solana history. The CAR files that this project generates follows a [schema](https://github.com/rpcpool/yellowstone-faithful/blob/main/ledger.ipldsch) specifically developed for Solana's historical archives. 

The content addressable nature means that each epoch, block, transaction and shredding is uniquely identified by a content hash. By knowing this content hash a user will be able to retreive a specific object of interest in a trustless manner, i.e. retrieve an object verifiably from a non-trusted source. Retrievals can be made via IPFS, the Filecoin network, or even by hosting the CAR files yourself on disk, a ceph cluster, S3, you name it.

### Indexes

Indexes will be needed to map Solana's block numbers, transaction signatures and addresses to their respective CIDs. These indexes will be developed as part of this project. There are four kinds of indexes that the Old Faithful index generation can provide:

 - slot-to-cid: Lookup a CID based on a slot number
 - tx-to-cid: Lookup a CID based on a transaction signature
 - gsfa: An index mapping Solana addresses to a list of singatures
 - cid-to-offset: Index for a specific CAR file, used by the local rpc server (see above) to find CIDs in a car file

### Archive access

The archive is currently under development. There will be two main ways to access the archive during its development phase:

1. Via Filecoin: Through collaboration with Protocol Labs and a Filplus allocation we are uploading all historical data to Filecoin. From here, any user can access the full histortical archive verifiably and freely from the Filecoin network. This is helpful as a way to test retreivals and fetch individual transactions.
2. Bulk transfer: If you want to start testing full epoch archives, we can provide you with access to a storage bucket from where you can pull the epoch archives.

The data that you will need to be able to run a local RPC server is:

  1) the Epoch car file containing all the data for that epoch
  2) the slot-to-cid index for that epoch
  3) the tx-to-cid index for that epoch
  4) the cid-to-offset index for that epoch car file
  5) Optionally (if you want to support getSignaturesForAddress): the gsfa index

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
radiance car create2 107 --db=46223992/rocksdb --out=/storage/car/epoch-107.car
```

This will produce a car file called epoch-107.car containing all the blocks and transactions for that epoch.

## Index generation

Once the radiance tooling has been used to prepare a car file (or if you have downloaded a car file externally) you can generate indexes from this car file by using the `faithful-cli`:

```
NAME:
   faithful index

USAGE:
   faithful index command [command options] [arguments...]

DESCRIPTION:
   Create various kinds of indexes for CAR files.

COMMANDS:
   cid-to-offset  
   slot-to-cid    
   sig-to-cid     
   all            
   gsfa           
   help, h        Shows a list of commands or help for one command

OPTIONS:
   --help, -h  show help
```

For example, to generate the three indexes cid-to-offset, slot-to-cid, sig-to-cid you would run:

```
faithful-cli index all epoch-107.car .
```

This would generate the indexes in the current dir for epoch-107.

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
