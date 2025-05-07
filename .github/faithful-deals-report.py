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
    # skip the last 2 epochs as they will take longer to get online
    epochs = range(770, current_epoch - 2)
    all_results = []
    
    # Process epochs in parallel
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        future_to_epoch = {executor.submit(check_epoch, epoch): epoch for epoch in epochs}
        for future in concurrent.futures.as_completed(future_to_epoch):
            all_results.append(future.result())
    
    # Print report
    print("=== Epoch Deals Summary ===")
    total_errors = 0
    
    # Track totals for summary
    total_metadata_entries = 0
    total_deals_in_csv = 0
    total_deals_online = 0
    
    for epoch, metadata_count, total_pieces, online_pieces, errors in sorted(all_results):
        error_count = len(errors)
        total_errors += error_count
        online_percentage = (online_pieces / total_pieces * 100) if total_pieces > 0 else 0
        deals_to_metadata_percentage = (total_pieces / metadata_count * 100) if metadata_count > 0 else 0
        
        # Add to totals
        total_metadata_entries += metadata_count
        total_deals_in_csv += total_pieces
        total_deals_online += online_pieces
        
        print(f"Epoch {epoch}:")
        print(f"  Metadata entries: {metadata_count}")
        print(f"  Deals in CSV: {total_pieces}")
        print(f"  Deals to metadata ratio: {deals_to_metadata_percentage:.1f}%")
        print(f"  Deals online: {online_pieces}")
        print(f"  Percent online: {online_percentage:.1f}%")
        
        if errors:
            print(f"  Pieces not found: {error_count}")
            # Uncomment to print individual pieces
            # for piece in errors:
            #     print(f"    {piece}")
        
        print()  # Add a blank line between epochs
    
    # Calculate summary percentages
    overall_deals_to_metadata_percentage = (total_deals_in_csv / total_metadata_entries * 100) if total_metadata_entries > 0 else 0
    overall_online_percentage = (total_deals_online / total_deals_in_csv * 100) if total_deals_in_csv > 0 else 0
    
    # Print summary for all epochs
    print("=== Summary for All Epochs ===")
    print(f"  Metadata entries: {total_metadata_entries}")
    print(f"  Deals in CSV: {total_deals_in_csv}")
    print(f"  Deals to metadata ratio: {overall_deals_to_metadata_percentage:.1f}%")
    print(f"  Deals online: {total_deals_online}")
    print(f"  Percent online: {overall_online_percentage:.1f}%")
    print(f"  Pieces not found: {total_errors}")
    print()
    
    print(f"=== Total Errors: {total_errors} ===")

if __name__ == "__main__":
    main()