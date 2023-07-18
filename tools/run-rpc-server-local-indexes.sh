#!/bin/bash

set -o pipefail
set -e

# Check if there is an epoch number provided
if [ $# -eq 0 ]; then
    echo "No epoch number provided"
    exit 1
fi

EPOCH=$1
# Check if the epoch number is a number
re='^[0-9]+$'
if ! [[ $EPOCH =~ $re ]]; then
    echo "Epoch number is not a number"
    exit 1
fi

# Check if the epoch number is greater than or equal to 0
if [ $EPOCH -lt 0 ]; then
    echo "Epoch number is less than 0"
    exit 1
fi

INDEX_DIR=${2:-.}

EPOCH_URL=https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car

set -x
faithful-cli rpc-server-car --listen ":7999" \
     ${EPOCH_URL} \
     ${INDEX_DIR}/epoch-${EPOCH}.car.*.cid-to-offset.index \
     ${INDEX_DIR}/epoch-${EPOCH}.car.*.slot-to-cid.index \
     ${INDEX_DIR}/epoch-${EPOCH}.car.*.sig-to-cid.index