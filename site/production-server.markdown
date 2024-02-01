---
layout: page
title: Production server
permalink: /rpc-server/production/
parent: RPC server
nav_order: 2
---

# Production server

The production server comes with multiple features that the Quickstart server doesn't. It allows you to serve multiple (or all) epochs, with support for dynamically adding new epochs during runtime. You can also make multiple different filecoin settings, such as using a whitelisted provider or excluding certain Filecoin storage providers.

The production server is available via the `faithful-cli rpc`  command.

## Configuration files 

To run a Faithful RPC server you need to specify configuration files for the epoch(s) you want to host. For multi-epoch support you need to generate epoch config files for the epochs that you want to host. An epoch config file looks like this:

```yml
data:
  car:
    uri: /faithful/493/epoch-493.car
  filecoin:
    enable: false
epoch: 493
version: 1
indexes:
  cid_to_offset:
    uri: /faithful/493/epoch-493.car.bafyreidlbcsg46dn5mqppioijyqb5cn6j23rkcoazl7skif74kpa3lihxa.cid-to-offset.index
  sig_to_cid:
    uri: /faithful/493/epoch-493.car.bafyreidlbcsg46dn5mqppioijyqb5cn6j23rkcoazl7skif74kpa3lihxa.sig-to-cid.index
  slot_to_cid:
    uri: /faithful/493/epoch-493.car.bafyreidlbcsg46dn5mqppioijyqb5cn6j23rkcoazl7skif74kpa3lihxa.slot-to-cid.index
  sig_exists:
    uri: /faithful/493/epoch-493.car.bafyreidlbcsg46dn5mqppioijyqb5cn6j23rkcoazl7skif74kpa3lihxa.sig-exists.index
```

The `uri` parameter supports both HTTP URIs as well as file based ones.

If you want you can also run the RPC server using some (or all) epochs via Filecoin:

```yml
data:
  filecoin:
    enable: true
    root_cid: bafyreigq7w4bwspbsf7j4ykov34fcf6skrn663n4ywfalgxlhp7o5nes5a
epoch: 494
version: 1
indexes:
  cid_to_offset:
    uri: /faithful/494/epoch-494.car.bafyreigq7w4bwspbsf7j4ykov34fcf6skrn663n4ywfalgxlhp7o5nes5a.cid-to-offset.index
  sig_to_cid:
    uri: /faithful/494/epoch-494.car.bafyreigq7w4bwspbsf7j4ykov34fcf6skrn663n4ywfalgxlhp7o5nes5a.sig-to-cid.index
  slot_to_cid:
    uri: /faithful/494/epoch-494.car.bafyreigq7w4bwspbsf7j4ykov34fcf6skrn663n4ywfalgxlhp7o5nes5a.slot-to-cid.index
  sig_exists:
    uri: /faithful/494/epoch-494.car.bafyreigq7w4bwspbsf7j4ykov34fcf6skrn663n4ywfalgxlhp7o5nes5a.sig-exists.index
```

This requires specifying the root CID for the epoch. You can retrieve the root CID for any epoch from `https://files.old-faithful.net/<epoch>/epoch-<epoch>.cid` e.g. `https://files.old-faithful.net/489/epoch-489.cid`. You can mix and match epochs hosted in different ways, but each epoch can only be served from a single source. 

In this config file you can optionally add `gsfa` to point to the directory of the gsfa index for the epoch.

We host sample config files for each epoch under the path `https://files.old-faithful.net/<epoch>/<epoch>.yml` which will point at indexes and CAR files hosted remotely at https://files.old-faithful.net. You will typically want to at least clone the indexes locally if you want to use this for any production purposes. 

## Proxying support

The production RPC server provides a proxy mode which allows it to forward traffic it can't serve to a downstream RPC server. To configure this, simply provide the command line argument `--proxy` pointing it to a regular Solana RPC node.

## Split routing

The production RPC server can be deployed in split-routing mode via a Layer 7 proxy such as Nginx, Envoy or HAproxy. In this mode, you would forward requests for recent blocks or transactions to a regular Solana RPC server, and then forward requests for historical blocks or transactions to Old Faithful. 

Our recommendation is that you configure the Solana validator to keep at least 2 days of local history (approximately 1 bn shreds) but **without** access to a Bigtable history store. You can then first let requests be routed to the Solana validator and only if those fail to be found, forward the request to the Faithful RPC.

Another option can be to configure faithful to sit as the first line proxy and then use the built in proxy mode to forward RPC traffic it cannot service downstream to a vanilla Solana RPC node.


## Systemd unit

A sample systemd unit that looks for epoch config files in `/etc/faithful/epochs` is listed below. You can start this up and then add the epoch yaml configs into this folder to support new epochs.

```
[Unit]
Description=Faithful RPC server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/faithful-cli rpc --listen ":8899"  --watch -p -vv /etc/faithful/epochs                                                                                  
Restart=always
LimitNOFILE=1500000
LimitNPROC=2000000


[Install]
WantedBy=multi-user.target
```
