---
layout: page
title: Filecoin RPC
permalink: /filecoin/rpc/
parent: Filecoin
---

# Filecoin RPC

The filecoin RPC server allows provide getBlock, getTransaction and getSignaturesForAddress powered by Filecoin. This requires access to indexes. The indexes allow you to lookup transaction signatures, block numbers and addresses and map them to Filecoin CIDs.

You can run it in the following way:

```
faithful-cli rpc 455.yml
```

The config file specifies to use data from filecoin and indexes from local files. The config file looks like this:

```yml
epoch: 455 # epoch number (required)
data: # data section (required)
  filecoin:
    # filecoin-mode section: source the data directly from filecoin.
    enable: false
indexes: # indexes section (required)
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-455/epoch-455-bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y-mainnet-slot-to-cid.index'
  sig_to_cid:
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-455/epoch-455-bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y-mainnet-sig-to-cid.index'
  sig_exists:
    # required (always); you can provide either a local filepath or a HTTP url:
    uri: '/media/runner/solana/indexes/epoch-455/epoch-455-bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y-mainnet-sig-exists.index'
  gsfa: # getSignaturesForAddress index
    # optional; must be a local directory path.
    uri: '/media/runner/solana/indexes/epoch-455/gsfa/epoch-455-bafyreibkequ55hyrhyk6f24ctsofzri6bjykh76jxl3zju4oazu3u3ru7y-gsfa.indexdir'
```

Due to latency in fetching signatures, typically the getSignaturesForAddress index needs to be stored in a local directory, but the other indexes can be fetched via HTTP or via local file system access. If you provide a URL, you need to make sure that the url supports HTTP Range requests. S3 or similar APIs will support this.

You can enter URLs from old-faithful.net in these config files.

There is a mode in which you can use a remote gSFA index, which limits it to only return signatures and not additional transaction meta data. In this mode, you can use a remote gSFA index. To enable this mode run faithful-cli in the following way:

```bash
faithful-cli rpc -gsfa-only-signatures=true 455.yml 
```

## Configuration files

Configuration files for filecoin accesses are still under development.
