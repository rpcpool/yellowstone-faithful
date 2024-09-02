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

CID_URL=https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.cid
EPOCH_CID=$($READ_COMMAND $CID_URL)

CONFIG_FILE_NAME="epoch-${EPOCH}.yml"

# Check if this is epoch 0 and in that case download the genesis file
if [ $EPOCH -eq 0 ]; then
# Check if genesis.tar.bz2 is already downloaded
    if [ ! -f "genesis.tar.bz2" ]; then
        GENESIS_URL=https://api.mainnet-beta.solana.com/genesis.tar.bz2
        $DOWNLOAD_COMMAND $GENESIS_URL
    fi
    GENESIS_CONFIG=$(cat <<EOF
genesis:
  uri: ${PWD}/genesis.tar.bz2
EOF
)
else
    GENESIS_CONFIG=""
fi

CONFIG_CONTENT=$(cat <<EOF
version: 1
epoch: ${EPOCH}
data:
  car:
    uri: https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car
  filecoin:
    enable: false
${GENESIS_CONFIG}
indexes:
  cid_to_offset:
    uri: https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car.${EPOCH_CID}.cid-to-offset.index
  sig_to_cid:
    uri: https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car.${EPOCH_CID}.sig-to-cid.index
  slot_to_cid:
    uri: https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car.${EPOCH_CID}.slot-to-cid.index
  sig_exists:
    uri: https://files.old-faithful.net/${EPOCH}/epoch-${EPOCH}.car.${EPOCH_CID}.sig-exists.index
EOF
)

# Write the content of the multiline variable to the configuration file
echo "$CONFIG_CONTENT" > $CONFIG_FILE_NAME

echo "Configuration file '$CONFIG_FILE_NAME' has been generated."
