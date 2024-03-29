---
layout: page
title: old-faithful.net
permalink: /rpc-server/old-faithful-net/
parent: RPC server
nav_order: 1
---

# Local CAR files and old-faithful.net

old-faithful.net provides archives that have been generated by Triton One as part of the Old Faithful project. You can use this data either by cloning it locally or by partially/fully serving it from old-faithful.net. A simple mode of operation can be to clone the index files locally but source the data from old-faithful.net. A similar mode of operation can be used with [Filecoin](/filecoin/).

For running a local server, it is usually easiest to have access to the [tools folder](https://github.com/rpcpool/yellowstone-faithful/tree/main/tools) from the repository. It provides some useful scripts to interface with the archives and indexes hosted on old-faithful.net as well as running the faithful-cli rpc server.

## Fully remote

You can use the RPC server with absolutely nothing downloaded locally. A helper script is available from the repository:

```
$ ./tools/run-rpc-server-remote.sh 0
```

## RPC server with local indexes

For ongoing testing, we strongly recommend that you download at least the indexes for best performance. If you have local indexes downloaded you can use the following helper script:

```
$ ./tools/run-rpc-server-local-indexes.sh 0 ./epoch0
```

There is a utility script in the `tools` folder that will download the indexes hosted on old-faithful.net. The indexes will also be available on Filecoin.

```
$ mkdir epoch0
$ cd epoch0
$ ../tools/download-indexes.sh 0 ./epoch0
$ ../tools/download-gsfa.sh 0 ./epoch0
```

## RPC server running fully locally

If you have a local copy of a CAR archive and the indexes and run a RPC server servicing data from them. For example:

```
/usr/local/bin/faithful-cli rpc \
    --listen $PORT \
    epoch-455.yaml
```

You can download the CAR files either via Filecoin or via the bucket provided by Triton. There are helper scripts in the `tools` folder. To download the full epoch data:

```
$ mkdir epoch0
$ cd epoch0
$ ../tools/download-epoch.sh 0
$ ../tools/download-indexes.sh 0
$ ../tools/download-gsfa.sh 0
```

Once files are downloaded there are also utility scripts to run the server:
```
$ ./tools/run-rpc-server-local.sh 0 ./epoch0
```

This will host epoch 0 from the data available in the folder epoch0.
