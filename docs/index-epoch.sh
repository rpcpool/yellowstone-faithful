#!/bin/bash

if [ "$#" -ne 4 ]; then
    echo "Run $0 <index type> <epoch> <carfile> <output_folder>"
    exit 4
fi

mkdir -p "${4}/tmp"

/usr/local/bin/faithful-cli version >"${4}/${2}/faithful.version.txt"

if [[ "${1}" != "gsfa" ]]; then
    # Generate the indexes
    /usr/local/bin/faithful-cli index "${1}" \
        --network=mainnet \
        --epoch=${EPOCH} \
        --tmp-dir "${4}/tmp" \
        "${3}" \
        "${4}/${2}/"
fi

if [[ "${1}" == "gsfa" ]] || [[ "${1}" == "all" ]]; then
    # Create gsfa index
    /usr/local/bin/faithful-cli index gsfa "${3}" "${4}/${2}/"
fi

# create a tar file from the gsfa index file
cd "${4}/${2}" && \
 tar -cf "${4}/${2}/epoch-${2}-gsfa.index.tar.zstd" -I "zstd -T8" ${4}/${2}/epoch-${2}*gsfa.indexdir && \
 cd - # dont put this string in quotes as we need the shell globbing

# Split CAR into 30gb split files, that's the max supported by Filecoin
/usr/local/bin/filecoin-data-prep fil-data-prep \
    --size 30000000000 \
    --output "${4}/${2}/sp-index-epoch-${2}" \
    --metadata "${4}/${2}/index.csv" \
    ${4}/${2}/*.index ${4}/${2}/*.index.tar.zstd # need to include shell globbing
