---
layout: page
title: Filecoin
permalink: /filecoin/
nav_order: 5
has_children: true
---

# Filecoin

As part of the Old Faithful project we are placing all of Solana's archive data on Filecoin. This will allow secure, decentralized access to Solana's history data.

## CIDs

To retrieve data from Filecoin you will need knowledge about the [content identifiers](/archives/cid/) for the epochs, blocks or transactions you are interested in. In theory, you can clone an entire epoch via Filecoin, but you can also retrieve an individual block or transaction. When fetching blocs or transaction, you can fetch only the metdata or the full data of the transaction/block.

## Fetching CIDs

If you already know the CID of the data you are looking for you can fetch it via `faithful-cli fetch <cid>`. This requires no further indexes and can also be used to recursively fetch data for example for an epoch. To avoid fetching the full dataset for an epoch (100s of GB) you probably want to pass the parameter `--dag-scope=block` to fetch only the particular CID entity that you are interested in.

## Retrieving Epoch CIDs

You can fetch the latest Epoch CIDs uploaded from old-faithful.net:

```
curl https://files.old-faithful.net/100/epoch-100.cid
```

