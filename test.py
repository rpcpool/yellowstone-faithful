import asyncio
import aiohttp
from typing import Optional

async def fetch_text(session: aiohttp.ClientSession, url: str) -> Optional[str]:
    try:
        async with session.get(url) as response:
            if response.status == 200:
                return await response.text()
    except:
        pass
    return None

async def get_size(session: aiohttp.ClientSession, url: str) -> str:
    try:
        async with session.head(url) as response:
            if response.status == 200:
                size_bytes = int(response.headers.get('content-length', 0))
                size_gb = size_bytes / (1024 * 1024 * 1024)
                return str(size_gb)
    except:
        pass
    return "n/a"

async def check_indices_size(epoch: int):
    host = "https://files.old-faithful.net"
    
    async with aiohttp.ClientSession() as session:
        # Get the CID first
        cid_url = f"{host}/{epoch}/epoch-{epoch}.cid"
        bafy = await fetch_text(session, cid_url)
        print(f"Epoch {epoch} CID: {bafy}")
        
        if not bafy:
            print(f"Epoch {epoch}: Could not get CID")
            return
        
        # Check all required index files
        index_files = [
            f"epoch-{epoch}-{bafy}-mainnet-cid-to-offset-and-size.index",
            f"epoch-{epoch}-{bafy}-mainnet-sig-to-cid.index",
            f"epoch-{epoch}-{bafy}-mainnet-sig-exists.index",
            f"epoch-{epoch}-{bafy}-mainnet-slot-to-cid.index",
            f"epoch-{epoch}-{bafy}-mainnet-slot-to-blocktime.index",
            f"epoch-{epoch}-gsfa.index.tar.zstd"
        ]

        print(f"\nChecking files for epoch {epoch}:")
        sizes = []
        for file in index_files:
            url = f"{host}/{epoch}/{file}"
            size = await get_size(session, url)
            print(f"{file}: {size} GB")
            sizes.append(size)
        
        # Convert sizes to integers, treating "n/a" as 0
        size_ints = [int(size) if size != "n/a" else 0 for size in sizes]
        total_size = sum(size_ints)
        print(f"\nTotal size for epoch {epoch}: {total_size} GB")

async def main():
    for epoch in [678]:
        await check_indices_size(epoch)
        print("\n" + "="*50 + "\n")

# Run this line to execute:
asyncio.run(main())
