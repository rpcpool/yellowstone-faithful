---
layout: page
title: Indexes
permalink: /archives/indexes/
parent: Archives
nav_order: 3
---

# Indexes

Indexes will be needed to map Solana's block numbers, transaction signatures and addresses to their respective Content Identifiers. These indexes will be developed as part of this project. There are three content identifier indexes kinds that the Old Faithful index generation can provide:

 - slot-to-cid: Lookup a CID based on a slot number
 - tx-to-cid: Lookup a CID based on a transaction signature
 - cid-to-offset: Index for a specific CAR file, used by the local rpc server (see above) to find CIDs in a car file

In addition to these Old Faithful supports an index called `gsfa` that maps Solana addresses to a list of transaction signatures.

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