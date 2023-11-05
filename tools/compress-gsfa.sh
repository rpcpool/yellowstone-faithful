#!/bin/bash

set -o pipefail
set -e

# the provided folder must exist and end with -gsfa-index or -gsfa-index/
if [ ! -d "$1" ]; then
	echo "The provided index folder does not exist"
	exit 1
fi
# must have suffix -gsfa-index or -gsfa-index/
if [ "${1: -11}" != "-gsfa-index" ] && [ "${1: -12}" != "-gsfa-index/" ]; then
	echo "The provided index folder must end with -gsfa-index or -gsfa-index/"
	exit 1
fi
# declare index folder and destination folder, trim trailing slash if present
source_folder="${1%/}"
destination_folder="${2%/}"
destination_file="$destination_folder/$(basename "$source_folder").tar.zst"
# check if destination folder exists
if [ ! -d "$destination_folder" ]; then
	echo "The provided destination folder does not exist"
	exit 1
fi
# check if destination file already exists
if [ -f "$destination_file" ]; then
	echo "The destination file already exists: $destination_file"
	exit 1
fi
# Get the size of the index folder
index_size=$(du -sh "$source_folder" | cut -f1)
echo "Index folder size: $index_size"
# Get the available space in the destination folder
available_space=$(df -h "$destination_folder" | tail -1 | awk '{print $4}')
echo "Available space in destination folder: $available_space"
echo "Compressing $source_folder to $destination_file ..."
tar -I zstd -cf "$destination_file" -C "$(dirname "$source_folder")" "$(basename "$source_folder")"
echo "Done"
