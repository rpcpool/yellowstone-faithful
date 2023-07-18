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

# Check if wget is available
DOWNLOAD_COMMAND="wget"
READ_COMMAND="wget -qO-"
if ! [ -x "$(command -v wget)" ]; then
    # Fallback to curl
    if ! [ -x "$(command -v curl)" ]; then
        echo "curl nor wget not installed"
        exit 1
    else
        DOWNLOAD_COMMAND="curl -O"
        READ_COMMAND="curl -s"
    fi
    echo "wget is not installed"
    exit 1
fi

EPOCH_URL=https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car

set -x
$DOWNLOAD_COMMAND $EPOCH_URL