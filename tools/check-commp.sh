#!/bin/bash

# Verify Piece CIDs for CAR files
# This script compares the piece CIDs in a metadata file with those calculated by stream-commp

# Check if stream-commp is installed
if ! command -v stream-commp &> /dev/null; then
    echo "Error: stream-commp is not installed or not in PATH. See https://github.com/filecoin-project/go-fil-commp-hashhash/tree/master/cmd/stream-commp"
    exit 1
fi

# Set the metadata file
METADATA_FILE="metadata.csv"
if [ ! -f "$METADATA_FILE" ]; then
    echo "Error: Metadata file '$METADATA_FILE' not found"
    echo "Please create the file or specify the correct path"
    exit 1
fi

# Skip the header line and process each CAR file
echo "Verifying piece CIDs for CAR files..."
echo "----------------------------------------"
echo "File | Expected Piece CID | Actual Piece CID | Status"
echo "----------------------------------------"

# Skip the header line and read each data line
tail -n +2 "$METADATA_FILE" | while IFS=, read -r car_file expected_piece_cid payload_cid padded_size file_size; do
    # Check if the CAR file exists
    if [ ! -f "$car_file" ]; then
        echo "$car_file | $expected_piece_cid | FILE NOT FOUND | ❌"
        continue
    fi

    # Get the actual piece CID using stream-commp
    commp_output=$(cat "$car_file" | stream-commp)
    actual_piece_cid=$(echo "$commp_output" | grep "CommPCid:" | awk '{print $2}')

    # Compare the piece CIDs
    if [ "$expected_piece_cid" = "$actual_piece_cid" ]; then
        echo "$car_file | $expected_piece_cid | $actual_piece_cid | ✅"
    else
        echo "$car_file | $expected_piece_cid | $actual_piece_cid | ❌"
    fi
done

echo "----------------------------------------"
echo "Verification complete."
