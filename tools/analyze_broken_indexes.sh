#!/bin/bash

# Script to analyze broken indexes and find those with good alternatives

echo "=== BROKEN INDEXES WITH GOOD ALTERNATIVES (>100MB) ==="
echo "Epoch,Current_Size,Alternative_Size,Alternative_Path"

# Process the CSV to find broken indexes with large alternatives
while IFS=',' read -r epoch yaml_path index_size status alt_path alt_size; do
    if [[ "$status" == "BROKEN" ]] && [[ "$alt_size" != "N/A" ]]; then
        # Convert size to bytes for comparison
        alt_size_unit=$(echo "$alt_size" | grep -o '[KMG]')
        alt_size_num=$(echo "$alt_size" | grep -o '^[0-9.]*')
        
        case "$alt_size_unit" in
            G) alt_bytes=$(echo "$alt_size_num * 1024 * 1024 * 1024" | bc) ;;
            M) alt_bytes=$(echo "$alt_size_num * 1024 * 1024" | bc) ;;
            K) alt_bytes=$(echo "$alt_size_num * 1024" | bc) ;;
            *) alt_bytes=$alt_size_num ;;
        esac
        
        # Check if alternative is > 100MB
        if (( $(echo "$alt_bytes > 104857600" | bc -l) )); then
            echo "$epoch,$index_size,$alt_size,$alt_path"
        fi
    fi
done < /tmp/index_check_results.csv | sort -n -t, -k1

echo ""
echo "=== SUMMARY ==="
echo -n "Total broken indexes: "
grep -c BROKEN /tmp/index_check_results.csv

echo -n "Broken indexes with >100MB alternatives: "
while IFS=',' read -r epoch yaml_path index_size status alt_path alt_size; do
    if [[ "$status" == "BROKEN" ]] && [[ "$alt_size" != "N/A" ]]; then
        alt_size_unit=$(echo "$alt_size" | grep -o '[KMG]')
        alt_size_num=$(echo "$alt_size" | grep -o '^[0-9.]*')
        
        case "$alt_size_unit" in
            G) alt_bytes=$(echo "$alt_size_num * 1024 * 1024 * 1024" | bc) ;;
            M) alt_bytes=$(echo "$alt_size_num * 1024 * 1024" | bc) ;;
            K) alt_bytes=$(echo "$alt_size_num * 1024" | bc) ;;
            *) alt_bytes=$alt_size_num ;;
        esac
        
        if (( $(echo "$alt_bytes > 104857600" | bc -l) )); then
            echo "$epoch"
        fi
    fi
done < /tmp/index_check_results.csv | wc -l