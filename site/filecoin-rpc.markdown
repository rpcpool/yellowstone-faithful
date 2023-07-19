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

You can enter URLs from old-faithful.net in these config files.

There is a mode in which you can use a remote gSFA index, which limits it to only return signatures and not additional transaction meta data. In this mode, you can use a remote gSFA index. To enable this mode run faithful-cli in the following way:

```
faithful-cli rpc-server-filecoin -config 455.yml -gsfa-only-signatures=true
```

## Configuration files

Configuration files for filecoin accesses are still under development.