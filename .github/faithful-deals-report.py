import requests
import json
import concurrent.futures
from collections import defaultdict

def get_current_epoch() -> int:
    response = requests.post(
        'https://api.mainnet-beta.solana.com',
        json={"jsonrpc":"2.0","id":1, "method":"getEpochInfo"}
    )
    data = response.json()
    return int(data['result']['epoch'])

def check_epoch(epoch):
    errors = []
    total_pieces = 0
    metadata_count = 0
    
    try:
        # Check metadata.csv first
        metadata_response = requests.get(f"https://filecoin-car-storage-cdn.b-cdn.net/{epoch}/metadata.csv")
        metadata_data = metadata_response.text
        metadata_lines = metadata_data.strip().split("\n")
        metadata_count = len(metadata_lines) - 1 # Subtract 1 to skip header row
        
        # Try deals-metadata.csv first, then fall back to deals.csv if needed
        deals_response = requests.get(f"https://filecoin-car-storage-cdn.b-cdn.net/{epoch}/deals-metadata.csv")
        
        # If deals-metadata.csv returns an error status code, try deals.csv instead
        if deals_response.status_code != 200:
            deals_response = requests.get(f"https://filecoin-car-storage-cdn.b-cdn.net/{epoch}/deals.csv")
        
        deals_data = deals_response.text
        
        # Skip header line and extract pieces
        pieces = [line.split(",")[4] for line in deals_data.strip().split("\n")[1:] if len(line.split(",")) > 4]
        total_pieces = len(pieces)
        
        for piece in pieces:
            search_response = requests.get(f"https://api.filecoin.tools/api/search?filter={piece}")
            search_data = search_response.json()
            
            if "data" in search_data and len(search_data["data"]) == 0:
                errors.append(piece)
    except Exception as e:
        print(f"Error processing epoch {epoch}: {str(e)}")
    
    return epoch, metadata_count, total_pieces, len(pieces) - len(errors), errors

def main():
    # Get the current epoch
    current_epoch = get_current_epoch()
    epochs = range(0, current_epoch)
    all_results = []
    
    # Process epochs in parallel
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        future_to_epoch = {executor.submit(check_epoch, epoch): epoch for epoch in epochs}
        for future in concurrent.futures.as_completed(future_to_epoch):
            all_results.append(future.result())
    
    # Print report in Markdown format
    print("# Faithful Deals Report\n")
    print("## Epoch Deals Summary\n")
    
    # Print table header
    print("| Epoch | Metadata Entries | Deals in CSV | Deals to Metadata Ratio | Deals Active | Percent Active | Pieces Not Found |")
    print("|-------|------------------|--------------|-------------------------|--------------|----------------|------------------|")
    
    total_errors = 0
    
    # Track totals for summary
    total_metadata_entries = 0
    total_deals_in_csv = 0
    total_deals_active = 0
    
    # Sort results in descending order by epoch
    for epoch, metadata_count, total_pieces, online_pieces, errors in sorted(all_results, key=lambda x: x[0], reverse=True):
        error_count = len(errors)
        total_errors += error_count
        active_percentage = (online_pieces / total_pieces * 100) if total_pieces > 0 else 0
        deals_to_metadata_percentage = (total_pieces / metadata_count * 100) if metadata_count > 0 else 0
        
        # Add to totals
        total_metadata_entries += metadata_count
        total_deals_in_csv += total_pieces
        total_deals_active += online_pieces
        
        # Print table row
        # Show "Current Epoch" for the current epoch, "Pending Cargen" if metadata_count is 0
        if epoch == current_epoch:
            metadata_display = "current epoch"
        elif metadata_count == 0:
            metadata_display = "pending cargen"
        else:
            metadata_display = f"{metadata_count:,}"
            
        print(f"| {epoch} | {metadata_display} | {total_pieces:,} | {deals_to_metadata_percentage:.1f}% | {online_pieces:,} | {active_percentage:.1f}% | {error_count:,} |")
    
    # Calculate summary percentages
    overall_deals_to_metadata_percentage = (total_deals_in_csv / total_metadata_entries * 100) if total_metadata_entries > 0 else 0
    overall_active_percentage = (total_deals_active / total_deals_in_csv * 100) if total_deals_in_csv > 0 else 0
    
    # Add totals row to the table
    print(f"| **Total** | {total_metadata_entries:,} | {total_deals_in_csv:,} | {overall_deals_to_metadata_percentage:.1f}% | {total_deals_active:,} | {overall_active_percentage:.1f}% | {total_errors:,} |")
    
    # Print summary for all epochs
    print("\n## Summary for All Epochs\n")
    print(f"- **Metadata Entries**: {total_metadata_entries:,}")
    print(f"- **Deals in CSV**: {total_deals_in_csv:,}")
    print(f"- **Deals to Metadata Ratio**: {overall_deals_to_metadata_percentage:.1f}%")
    print(f"- **Deals Active**: {total_deals_active:,}")
    print(f"- **Percent Active**: {overall_active_percentage:.1f}%")
    print(f"- **Pieces Not Found**: {total_errors:,}")

if __name__ == "__main__":
    main()