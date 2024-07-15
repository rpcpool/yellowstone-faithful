#!/bin/bash

set -o pipefail
set -e

filename=$1

response=$(cat $filename | stream-commp 2>&1)

# Example response:
# cat epoch-0-1.car| stream-commp

# CommPCid: baga6ea4seaqlenpkttfecrwa74hkwfp3pnmkaatmiiwpvuclwtxcl4uvkajemea
# Payload:          1004138576 bytes
# Unpadded piece:   1065353216 bytes
# Padded piece:     1073741824 bytes

# CARv1 detected in stream

commp=$(echo $response | grep -o "CommPCid.*" | cut -f 2 -d ":" | xargs)
paddedSize=$(echo $response | grep -o "Padded piece:.*" | cut -f 2 -d ":" | sed "s/bytes//" |  xargs)

echo $filename, $commp, $paddedSize
