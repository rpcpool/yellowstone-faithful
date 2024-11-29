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

get_indices() {
    local epoch=$1
    local cid_url="$host/$epoch/epoch-$epoch.cid"
    
    # First get the CID (BAFY...)
    if ! check_file_exists "$cid_url"; then
        echo "n/a"
        return
    fi
    
    local bafy=$(curl -s "$cid_url")
    if [[ -z "$bafy" ]]; then
        echo "n/a"
        return
    fi

    # Check all required index files exist
    local index_files=(
        "epoch-$epoch-$bafy-mainnet-cid-to-offset-and-size.index"
        "epoch-$epoch-$bafy-mainnet-sig-to-cid.index"
        "epoch-$epoch-$bafy-mainnet-sig-exists.index"
        "epoch-$epoch-$bafy-mainnet-slot-to-cid.index"
        "epoch-$epoch-gsfa.index.tar.zstd"
    )

    for file in "${index_files[@]}"; do
        if ! check_file_exists "$host/$epoch/$file"; then
            echo "n/a"
            return
        fi
    done

    # If we get here, all files exist
    echo "$host/$epoch/epoch-$epoch-indices"
}

get_deals() {
    local epoch=$1
    local deals_url="$deals_host/$epoch/deals.csv"
    if check_file_exists "$deals_url"; then
        local deals=$(curl -s "$deals_url")
        # right now it's just checking the deals.csv exists and is longer than 1 line
        # i.e. we sent a deal that was accepted by an SP
        # TODO: use `faithful check-deals` to determine if the full epoch can be loaded from filecoin 
        if [[ -n "$deals" ]] && [[ $(echo "$deals" | wc -l) -gt 1 ]]; then
            echo "$deals_url"
        else
            echo "n/a"
        fi
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
    echo "| $epoch | n/a | n/a | n/a | n/a  | n/a | n/a | n/a |"
}

print_row() {
    local epoch=$1
    local car=$2
    local sha=$3
    local sha_url=$4
    local size=$5
    local poh=$6
    local txmeta=$7
    local deals=$8
    local indices=$9

    # Only create links if the values aren't "n/a"
    local car_cell="✗"
    local sha_cell="✗" 
    local size_cell="✗"
    local poh_cell="✗"
    local txmeta_cell="✗"
    local deals_cell="✗"
    local indices_cell="✗"

    [[ "$car" != "n/a" ]] && car_cell="[epoch-$epoch.car]($car)"
    [[ "$sha" != "n/a" ]] && sha_cell="[${sha:0:5}]($sha_url)"
    [[ "$size" != "n/a" ]] && size_cell="$size GB"
    [[ "$poh" != "n/a" ]] && poh_cell="$poh"
    [[ "$txmeta" != "n/a" ]] && txmeta_cell="$txmeta"
    [[ "$indices" != "n/a" ]] && indices_cell="✓"
    [[ "$deals" != "n/a" ]] && deals_cell="[✓]($deals)"

    echo "| $epoch | $car_cell | $sha_cell | $size_cell | $txmeta_cell | $poh_cell | $indices_cell | $deals_cell |"
}

CURRENT_EPOCH=$(curl -s https://api.mainnet-beta.solana.com -s -X POST -H "Content-Type: application/json" -d '
  {"jsonrpc":"2.0","id":1, "method":"getEpochInfo"}
' -s | jq -r .result.epoch)

# descending order
EPOCH_LIST=$(seq $CURRENT_EPOCH -1 0)

# test 
# EPOCH_LIST=$(seq 687 -1 0)

# very fast test
EPOCH_LIST=$(seq 687 -1 670)

# base hostname
host="https://files.old-faithful.net"
deals_host="https://filecoin-car-storage-cdn.b-cdn.net/"

echo "| Epoch #  | CAR  | CAR SHA256  | CAR filesize | tx meta check | poh check | Indexes | Filecoin Deals |"
echo "|---|---|---|---|---|---|---|---|"

for EPOCH in $EPOCH_LIST; do
    CAR=$(get_car_url "$EPOCH")
    SHA_URL="$host/$EPOCH/epoch-$EPOCH.sha256"
    
    if check_file_exists "$CAR"; then
        print_row "$EPOCH" \
                 "$CAR" \
                 "$(get_sha "$EPOCH")" \
                 "$SHA_URL" \
                 "$(get_size "$EPOCH")" \
                 "$(get_poh "$EPOCH")" \
                 "$(get_txmeta "$EPOCH")" \
                 "$(get_deals "$EPOCH")" \
                 "$(get_indices "$EPOCH")" \

    else
        print_empty_row "$EPOCH"
    fi
done