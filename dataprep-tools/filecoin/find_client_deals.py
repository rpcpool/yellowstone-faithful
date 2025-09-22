#!/usr/bin/env python3

import requests
import json
from os import environ

def find_deals_for_client(client_id):
    """Find deals for a specific client ID"""
    try:
        rpc_endpoint = environ.get('FILECOIN_RPC_ENDPOINT', 'https://api.node.glif.io')
        
        # Get all deals in the market
        payload = {
            "jsonrpc": "2.0",
            "method": "Filecoin.StateMarketDeals",
            "params": [None],
            "id": 1
        }
        
        print(f"Querying deals for client {client_id}...")
        response = requests.post(rpc_endpoint, json=payload, timeout=30)
        response.raise_for_status()
        
        result = response.json()
        if 'result' in result:
            deals = result['result']
            client_deals = []
            
            for deal_id, deal_data in deals.items():
                if deal_data and 'Proposal' in deal_data:
                    proposal = deal_data['Proposal']
                    if proposal.get('Client') == client_id:
                        client_deals.append({
                            'dealId': deal_id,
                            'startEpoch': proposal.get('StartEpoch'),
                            'endEpoch': proposal.get('EndEpoch'),
                            'provider': proposal.get('Provider'),
                            'pieceCid': proposal.get('PieceCID', {}).get('/')
                        })
            
            return client_deals
        else:
            print("No result in response")
            return None
            
    except Exception as e:
        print(f"Failed to get deals for client {client_id}: {e}")
        return None

if __name__ == '__main__':
    client_id = "f02144497"
    print(f"Searching for deals with client ID: {client_id}")
    
    deals = find_deals_for_client(client_id)
    if deals:
        print(f"\nFound {len(deals)} deals for client {client_id}:")
        for i, deal in enumerate(deals[:5]):  # Show first 5 deals
            print(f"  Deal {i+1}:")
            print(f"    Deal ID: {deal['dealId']}")
            print(f"    Start Epoch: {deal['startEpoch']}")
            print(f"    End Epoch: {deal['endEpoch']}")
            print(f"    Provider: {deal['provider']}")
            print(f"    Piece CID: {deal['pieceCid']}")
            print()
        
        if len(deals) > 5:
            print(f"... and {len(deals) - 5} more deals")
    else:
        print("No deals found or error occurred")