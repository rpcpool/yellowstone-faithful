#!/bin/bash

# Script to upload an epoch folder to  b2
if [ "$#" -ne 2 ]; then
    echo "Run $0 <epoch> <folder>"
    exit 1
fi

if [[ ! -d "${2}/${1}" ]]; then
  echo "Folder ${2}/${1} doesn't exist"
  exit 1
fi

# unless doing full cargen or at least splitting, this file won't be generated
# if [[ ! -f "${2}/${1}/metadata.csv" ]]; then
#   echo "metadata.yaml must exist in the target folder ${2}/${1}/metadata.csv"
#   exit 1
# fi

# fix a bug in the CID naming
mv "${2}/${1}/${1}.cid" "${2}/${1}/epoch-${1}.cid" || true

echo "uploading to b2"
set -x
b2 sync \
    --includeRegex 'epoch-.*\.index' \
    --includeRegex 'epoch-.*.index.tar.zstd' \
    --includeRegex 'metadata.yaml' \
    --includeRegex 'index.csv' \
    --includeRegex 'metadata.csv' \
    --includeRegex 'sp-*.car' \
    --includeRegex 'epoch-.*\.car' \
    --includeRegex 'epoch-.*\.cid' \
    --includeRegex '.*\.slots\.txt' \
    --includeRegex '.*\.recap\.txt' \
    --includeRegex '.*\.recap\.yaml' \
    --includeRegex '.*\.version\.txt' \
    --excludeRegex '.*' "${2}/${1}/" \
    "b2://{{ warehouse_processor_bucket }}/${1}/"

b2 ls "{{ warehouse_processor_bucket }}" ${1}
