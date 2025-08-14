#!/bin/bash

# Script to check all epochs for incorrect indexes
# by comparing YAML configuration with actual index file sizes

echo "Epoch,YAML_Path,Index_Size,Status,Alt_Path,Alt_Size"

# Loop through all YAML files in /tank/webnx/
for yaml_file in /tank/webnx/*.yml; do
    # Extract epoch number from filename
    epoch=$(basename "$yaml_file" .yml)
    
    # Skip if not a number
    if ! [[ "$epoch" =~ ^[0-9]+$ ]]; then
        continue
    fi
    
    # Extract gsfa URI from YAML
    gsfa_path=$(grep -A1 "gsfa:" "$yaml_file" | grep "uri:" | awk '{print $2}')
    
    if [ -z "$gsfa_path" ]; then
        echo "$epoch,NO_GSFA_CONFIG,N/A,ERROR,N/A,N/A"
        continue
    fi
    
    # Check if the index file exists
    index_file="${gsfa_path}/pubkey-to-offset-and-size.index"
    
    if [ -f "$index_file" ]; then
        # Get file size in human readable format
        size=$(ls -lh "$index_file" | awk '{print $5}')
        
        # Check if size is less than 10K (likely broken)
        size_bytes=$(stat -c%s "$index_file" 2>/dev/null || stat -f%z "$index_file" 2>/dev/null)
        
        # Check for alternative path in gsfa-redone
        alt_path="/tank/gsfa-redone/${epoch}/$(basename "$gsfa_path")"
        alt_index="${alt_path}/pubkey-to-offset-and-size.index"
        alt_size="N/A"
        
        if [ -f "$alt_index" ]; then
            alt_size=$(ls -lh "$alt_index" | awk '{print $5}')
        fi
        
        # Determine status based on size
        if [ "$size_bytes" -lt 10240 ]; then  # Less than 10KB
            status="BROKEN"
        else
            status="OK"
        fi
        
        echo "$epoch,$gsfa_path,$size,$status,$alt_path,$alt_size"
    else
        # Check for alternative path
        alt_path="/tank/gsfa-redone/${epoch}/$(basename "$gsfa_path")"
        alt_index="${alt_path}/pubkey-to-offset-and-size.index"
        alt_size="N/A"
        
        if [ -f "$alt_index" ]; then
            alt_size=$(ls -lh "$alt_index" | awk '{print $5}')
        fi
        
        echo "$epoch,$gsfa_path,MISSING,ERROR,$alt_path,$alt_size"
    fi
done | sort -n -t, -k1