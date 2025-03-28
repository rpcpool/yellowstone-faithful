#!/bin/bash

if [ "$#" -ne 3 ]; then
    echo "Run $0 <epoch> <carfile> <output_folder>"
    exit 1
fi

# Create the epoch folder if it doesn't exist
mkdir -p "${3}/${1}"

# Create 30 gib pieces
/usr/local/bin/filecoin-data-prep -v >"${3}/${1}/dataprep.version.txt"
/usr/local/bin/filecoin-data-prep split-and-commp --size {{ warehouse_processor_car_size }} --output "${3}/${1}/sp-epoch-${1}" --metadata "${3}/${1}/metadata.csv" "${2}"

# @TODO use stream-commp to validate the hashes in the metadata.csv

mv "${2}" "${3}/${1}/"
