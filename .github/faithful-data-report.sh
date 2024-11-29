#!/usr/bin/env bash

# exit in case of error
set -e

get_sha() {
    local epoch=$1
    local sha_url="$host/$epoch/epoch-$epoch.sha256"
    if check_file_exists "$sha_url"; then
        local sha=$(curl -s "$sha_url")
        [[ -n "$sha" ]] && echo "$sha" || echo "n/a"
    else
        echo "n/a"
    fi
}

get_poh() {
    local epoch=$1
    local poh_url="$host/$epoch/poh-check.log"
    if check_file_exists "$poh_url"; then
        local poh=$(curl -s "$poh_url")
        [[ -n "$poh" ]] && echo "$poh" || echo "n/a"
    else
        echo "n/a"
    fi
}

get_txmeta() {
    local epoch=$1
    local txmeta_url="$host/$epoch/tx-metadata-check.log"
    if check_file_exists "$txmeta_url"; then
        local txmeta=$(curl -s "$poh_url")
        [[ -n "$txmeta" ]] && echo "$txmeta" || echo "n/a"
    else
        echo "n/a"
    fi
}

get_size() {
    local epoch=$1
    local size_url="$host/$epoch/epoch-$epoch.car"
    if check_file_exists "$size_url"; then
        local size=$(curl -s --head "$size_url" 2>/dev/null | grep -i content-length | awk '{print $2}' | tr -d '\r' | awk '{printf "%.0f", $1/1024/1024/1024}')
        [[ -n "$size" ]] && echo "$size" || echo "n/a"
    else
        echo "n/a"
    fi
}

get_car_url() {
    local epoch=$1
    local car_url="$host/$epoch/epoch-$epoch.car"
    if check_file_exists "$car_url"; then
        echo "$car_url"
    else
        echo "n/a"
    fi
}


check_file_exists() {
    local url=$1
    curl --output /dev/null --silent --head --fail "$url"
    return $?
}

print_empty_row() {
    local epoch=$1
    echo "| $epoch | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |"
}

print_row() {
    local epoch=$1
    local car=$2
    local sha=$3
    local sha_url=$4
    local size=$5
    local poh=$6
    local txmeta=$7

    # Only create links if the values aren't "n/a"
    local car_cell="n/a"
    local sha_cell="n/a"
    local size_cell="n/a"
    local poh_cell="n/a"
    local txmeta_cell="n/a"

    [[ "$car" != "n/a" ]] && car_cell="[epoch-$epoch.car]($car)"
    [[ "$sha" != "n/a" ]] && sha_cell="[$sha]($sha_url)"
    [[ "$size" != "n/a" ]] && size_cell="[$size]($car)"
    [[ "$poh" != "n/a" ]] && poh_cell="$poh"
    [[ "$txmeta" != "n/a" ]] && txmeta_cell="$txmeta"

    echo "| $epoch | $car_cell | $sha_cell | $size_cell |  | ✓ | $(date '+%Y-%m-%d %H:%M:%S') | ✓ | ✓ |"
}

CURRENT_EPOCH=$(curl -s https://api.mainnet-beta.solana.com -s -X POST -H "Content-Type: application/json" -d '
  {"jsonrpc":"2.0","id":1, "method":"getEpochInfo"}
' -s | jq -r .result.epoch)

# descending order
EPOCH_LIST=$(seq $CURRENT_EPOCH -1 0)
# test
EPOCH_LIST=$(seq 687 -1 0)
# fast test
EPOCH_LIST=$(seq 687 -1 670)

# base hostname
host="https://files.old-faithful.net"

echo "| Epoch #  | CAR  | CAR SHA256  | CAR filesize GB | tx meta check | poh check | CAR data created  | Indexes | Filecoin Deals |"
echo "|---|---|---|---|---|---|---|---|---|"

for EPOCH in $EPOCH_LIST; do
    CAR=$(get_car_url "$EPOCH")
    SHA_URL="$host/$EPOCH/epoch-$EPOCH.sha256"
    
    if check_file_exists "$CAR"; then
        print_row "$EPOCH" \
                 "$CAR" \
                 "$(get_sha "$EPOCH")" \
                 "$SHA_URL" \
                 "$(get_size "$EPOCH")"
    else
        print_empty_row "$EPOCH"
    fi
done