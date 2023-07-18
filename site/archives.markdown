---
layout: page
title: Archives
permalink: /archives/
has_children: true
nav_order: 4
---

# Archives

The core of the project is history archives in Content Addressable format ([overview](https://web3.storage/docs/how-tos/work-with-car-files/), [specs](https://ipld.io/specs/transport/car/carv1/)). These represent a verifiable, immutable view of the Solana history. The CAR files that this project generates follows a [schema](https://github.com/rpcpool/yellowstone-faithful/blob/main/ledger.ipldsch) specifically developed for Solana's historical archives.

The content addressable nature means that each epoch, block, transaction and shredding is uniquely identified by a content hash. By knowing this content hash a user will be able to retreive a specific object of interest in a trustless manner, i.e. retrieve an object verifiably from a non-trusted source. Retrievals can be made via IPFS, the Filecoin network, or even by hosting the CAR files yourself on disk, a ceph cluster, S3, you name it.
