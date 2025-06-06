#!/usr/bin/env python3
import asyncio
import aiohttp
from typing import Dict, Optional
from dataclasses import dataclass

@dataclass
class EpochData:
    epoch: int
    car: str = "n/a"
    sha: str = "n/a"
    sha_url: str = "n/a"
    size: str = "n/a"
    poh: str = "n/a"
    poh_url: str = "n/a"
    txmeta: str = "n/a"
    txmeta_url: str = "n/a"
    deals: str = "n/a"
    indices: str = "n/a"
    indices_size: str = "n/a"
    slots_url: str = "n/a"

class FaithfulDataReport:
    def __init__(self):
        self.host = "https://files.old-faithful.net"
        self.deals_host = "https://filecoin-car-storage-cdn.b-cdn.net"
        self.txmeta_first_epoch = 92
        self.issues = []  # Track issues for summary report
        self.index_files = [
            "mainnet-cid-to-offset-and-size.index",
            "mainnet-sig-to-cid.index",
            "mainnet-sig-exists.index",
            "mainnet-slot-to-cid.index",
            "mainnet-slot-to-blocktime.index",
            "gsfa.index.tar.zstd"
        ]
        
    async def check_url(self, session: aiohttp.ClientSession, url: str) -> bool:
        try:
            async with session.head(url, allow_redirects=True) as response:
                return response.status == 200
        except:
            return False

    async def fetch_text(self, session: aiohttp.ClientSession, url: str) -> Optional[str]:
        try:
            async with session.get(url) as response:
                if response.status == 200:
                    return await response.text()
        except:
            pass
        return None

    async def get_size(self, session: aiohttp.ClientSession, url: str) -> str:
        try:
            async with session.head(url) as response:
                if response.status == 200:
                    size_bytes = int(response.headers.get('content-length', 0))
                    if size_bytes > 0:
                        size_gb = max(1, round(size_bytes / (1024 * 1024 * 1024)))
                        return str(size_gb)
        except:
            pass
        return "n/a"

    async def check_gsfa_magic(self, session: aiohttp.ClientSession, epoch: int) -> bool:
        """
        Validates the GSFA index manifest by checking its magic version
        Returns True if valid, False otherwise
        """
        manifest_url = f"{self.host}/{epoch}/gsfa.manifest"
        try:
            headers = {'Range': 'bytes=8-15'}
            async with session.get(manifest_url, headers=headers) as response:
                if response.status == 206:  # Partial Content
                    content = await response.read()
                    # Convert bytes to hex string
                    hex_content = ''.join([f'{b:02X}' for b in content])
                    return hex_content == '0500000000000000'
        except:
            pass
        return False

    async def get_indices(self, session: aiohttp.ClientSession, epoch: int) -> str:
        cid_url = f"{self.host}/{epoch}/epoch-{epoch}.cid"
        
        # Get the CID first
        bafy = await self.fetch_text(session, cid_url)
        if not bafy:
            self.issues.append((epoch, ["failed to get CID"]))
            return "n/a"

        # Check all regular index files excluding gsfa
        regular_files = self.index_files[:-1]  # All files except gsfa
        checks = await asyncio.gather(*[
            self.check_url(session, f"{self.host}/{epoch}/epoch-{epoch}-{bafy}-{file}")
            for file in regular_files
        ])
        
        # Track which files failed validation
        missing_files = []
        for i, exists in enumerate(checks):
            if not exists:
                missing_files.append(f"missing index file: {regular_files[i]}")
        
        # Check gsfa file existence and validate its magic version
        gsfa_file = self.index_files[-1]
        gsfa_exists = await self.check_url(session, f"{self.host}/{epoch}/epoch-{epoch}-{gsfa_file}")
        gsfa_valid = True
        if not gsfa_exists:
            missing_files.append("missing GSFA index file")
        else:
            gsfa_valid = await self.check_gsfa_magic(session, epoch)
            if not gsfa_valid:
                missing_files.append("GSFA index file failed magic validation")
        
        # Add all missing files to issues if any
        if missing_files:
            self.issues.append((epoch, missing_files))
        
        # Add gsfa validation result to checks
        checks.append(gsfa_exists and gsfa_valid)

        return f"{self.host}/{epoch}/epoch-{epoch}-indices" if all(checks) else "n/a"
    
    async def get_indices_size(self, session: aiohttp.ClientSession, epoch: int) -> str:
        cid_url = f"{self.host}/{epoch}/epoch-{epoch}.cid"
        
        # Get the CID first
        bafy = await self.fetch_text(session, cid_url)
        if not bafy:
            return "n/a"

        # Get sizes for all regular index files (excluding gsfa which has a different naming pattern)
        sizes = await asyncio.gather(*[
            self.get_size(session, f"{self.host}/{epoch}/epoch-{epoch}-{bafy}-{file}")
            for file in self.index_files[:-1]  # All files except gsfa
        ])
        
        # Get gsfa size separately since it doesn't include the bafy CID in its filename
        gsfa_size = await self.get_size(session, f"{self.host}/{epoch}/epoch-{epoch}-{self.index_files[-1]}")
        sizes.append(gsfa_size)

        # Convert sizes to integers, treating "n/a" as 0
        size_ints = [int(size) if size != "n/a" else 0 for size in sizes]
        
        # Sum up all sizes
        total_size = sum(size_ints)
        
        return str(total_size) if total_size > 0 else "n/a"

    async def get_deals(self, session: aiohttp.ClientSession, epoch: int) -> str:
        # Check both possible filenames for deals
        deals_url = f"{self.deals_host}/{epoch}/deals.csv"
        deals_metadata_url = f"{self.deals_host}/{epoch}/deals-metadata.csv"
        
        # check deals.csv
        deals_content = await self.fetch_text(session, deals_url)
        if deals_content and len(deals_content.splitlines()) > 1:
            return deals_url
            
        # check deals-metadata.csv (recent change)
        deals_metadata_content = await self.fetch_text(session, deals_metadata_url)
        if deals_metadata_content and len(deals_metadata_content.splitlines()) > 1:
            return deals_metadata_url

        return "n/a"

    async def get_epoch_data(self, session: aiohttp.ClientSession, epoch: int) -> EpochData:
        car_url = f"{self.host}/{epoch}/epoch-{epoch}.car"
        sha_url = f"{self.host}/{epoch}/epoch-{epoch}.sha256"
        poh_url = f"{self.host}/{epoch}/poh-check.log"
        txmeta_url = f"{self.host}/{epoch}/tx-metadata-check.log"

        # Check if CAR exists first
        car_exists = await self.check_url(session, car_url)
        if not car_exists:
            return EpochData(epoch=epoch)

        # Gather all data concurrently
        sha, size, poh, txmeta, indices, indices_size, deals = await asyncio.gather(
            self.fetch_text(session, sha_url),
            self.get_size(session, car_url),
            self.fetch_text(session, poh_url),
            self.fetch_text(session, txmeta_url),
            self.get_indices(session, epoch),
            self.get_indices_size(session, epoch),
            self.get_deals(session, epoch)
        )

        # Construct slots.txt URL
        slots_url = f"{self.host}/{epoch}/{epoch}.slots.txt"


        return EpochData(
            epoch=epoch,
            car=car_url,
            sha=sha if sha else "n/a",
            sha_url=sha_url,
            size=size,
            poh=poh if poh else "n/a",
            poh_url=poh_url,
            txmeta=txmeta if txmeta else "n/a",
            txmeta_url=txmeta_url,
            deals=deals,
            indices=indices,
            indices_size=indices_size,
            slots_url=slots_url
        )

    def format_row(self, data: EpochData) -> str:
        car_cell = f"[epoch-{data.epoch}.car]({data.car})" if data.car != "n/a" else "✗"
        sha_cell = f"[{data.sha[:7]}]({data.sha_url})" if data.sha != "n/a" else "✗"
        size_cell = f"{data.size} GB" if data.size != "n/a" else "✗"
        
        # Special handling for earlier epochs txmeta validation
        if 0 <= data.epoch < self.txmeta_first_epoch and data.txmeta != "n/a":
            txmeta_cell = f"[★]({data.txmeta_url})"
        else:
            txmeta_cell = f"[✗]({data.txmeta_url})" if data.txmeta != "n/a" and not validate_txmeta_output(data.txmeta) else \
                         f"[✓]({data.txmeta_url})" if data.txmeta != "n/a" else "✗"

        # epoch 208 POH validation is handled differently
        if data.epoch == 208 and data.poh != "n/a":
            poh_cell = f"[★★]({data.poh_url})"
        elif data.poh != "n/a" and not validate_poh_output(data.poh):
            poh_cell = f"[✗]({data.poh_url})"
        elif data.poh != "n/a":
            poh_cell = f"[✓]({data.poh_url})"
        else:
            poh_cell = "✗"

        indices_cell = "✓" if data.indices != "n/a" else "✗"
        indices_size_cell = f"{data.indices_size} GB" if data.indices_size != "n/a" else "✗"
        deals_cell = f"[✓]({data.deals})" if data.deals != "n/a" else "✗"
        slots_cell = f"[{data.epoch}.slots.txt]({data.slots_url})" if data.slots_url != "n/a" else "✗"

        # Track issues for summary report
        issues = []
        if data.car == "n/a": issues.append("missing CAR")
        if data.sha == "n/a": issues.append("missing SHA")
        if data.size == "n/a": issues.append("missing size")
        if data.poh == "n/a": issues.append("missing POH check")
        elif not validate_poh_output(data.poh): issues.append("failed POH check")
        if data.txmeta == "n/a": issues.append("missing tx meta check")
        elif not validate_txmeta_output(data.txmeta) and not (0 <= data.epoch < self.txmeta_first_epoch):
            issues.append("failed tx meta check")
        if data.indices == "n/a": issues.append("missing indices")
        if data.indices_size == "n/a": issues.append("missing indices size")
        #if data.deals == "n/a": issues.append("missing deals")
        
        if issues:
            self.issues.append((data.epoch, issues))

        return f"| {data.epoch} | {car_cell} | {sha_cell} | {size_cell} | {txmeta_cell} | {poh_cell} | {indices_cell} | {indices_size_cell} | {deals_cell} | {slots_cell} |"

    async def get_current_epoch(self) -> int:
        async with aiohttp.ClientSession() as session:
            async with session.post(
                'https://api.mainnet-beta.solana.com',
                json={"jsonrpc":"2.0","id":1, "method":"getEpochInfo"}
            ) as response:
                data = await response.json()
                return int(data['result']['epoch'])

    async def run(self):
        current_epoch = await self.get_current_epoch()
        epochs = range((current_epoch-1), -1, -1)  # descending order

        print("| Epoch #  | CAR  | CAR SHA256  | CAR filesize | tx meta check | poh check | Indices | Indices Size | Filecoin Deals | Slots")
        print("|---|---|---|---|---|---|---|---|---|---|")
        print("|%s|epoch is|ongoing||||||||" % current_epoch)

        # concurrency levels
        chunk_size = 20  
        
        async with aiohttp.ClientSession() as session:
            for i in range(0, len(epochs), chunk_size):
                chunk = epochs[i:i + chunk_size]
                results = await asyncio.gather(
                    *[self.get_epoch_data(session, epoch) for epoch in chunk]
                )
                
                # Print results in order
                for result in results:
                    print(self.format_row(result))

        print("\n★ = tx meta validation skipped (epochs 0-%s where tx meta wasn't enabled yet)" % self.txmeta_first_epoch)
        print("\n★★ = epoch 208 POH validation is handled differently, see more in https://docs.old-faithful.net/validation")

        # Print summary report
        if self.issues:
            print("\n### Summary of Issues")
            for epoch, issues in sorted(self.issues):
                print(f"- Epoch {epoch}: {', '.join(issues)}")

def validate_txmeta_output(txmeta_text: str) -> bool:
    """
    Validates that txmeta check output shows zero missing and zero parsing errors
    Returns True if valid, False otherwise
    """
    if txmeta_text == "n/a":
        return False
        
    try:
        return 'Transactions with missing metadata: 0' in txmeta_text and \
            'Transactions with metadata parsing error: 0' in txmeta_text
        
    except Exception as e:
        return False

def validate_poh_output(poh_text: str) -> bool:
    """
    Validates the PoH check output
    Returns True if valid, False otherwise
    """
    if poh_text == "n/a":
        return False
        
    try:
        return 'Successfully checked PoH on CAR file for epoch' in poh_text
        
    except:
        return False

def main():
    report = FaithfulDataReport()
    asyncio.run(report.run())

if __name__ == "__main__":
    main()