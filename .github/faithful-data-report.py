#!/usr/bin/env python3
import asyncio
import json
import random
import sys
import aiohttp
from typing import Dict, Optional
from dataclasses import dataclass

# Statuses that answer the question. Anything else (429, 5xx, 403 challenge) is retried.
DEFINITIVE_STATUSES = {200, 206, 404, 410, 416}
MAX_ATTEMPTS = 6
MAX_BACKOFF_SECONDS = 120


class UnreliableResponse(Exception):
    """A request never returned a definitive status, so the report would be wrong."""


@dataclass
class Response:
    status: int
    headers: Dict[str, str]
    body: Optional[bytes]


@dataclass
class EpochData:
    epoch: int
    car: str = "n/a"
    sha: str = "n/a"
    sha_url: str = "n/a"
    b3: str = "n/a"
    b3_url: str = "n/a"
    size: str = "n/a"
    poh: str = "n/a"
    poh_url: str = "n/a"
    txmeta: str = "n/a"
    txmeta_url: str = "n/a"
    indices: str = "n/a"
    indices_size: str = "n/a"
    slot_range_index: str = "n/a"
    slot_range_index_issue: Optional[str] = None
    slots_url: str = "n/a"

class FaithfulDataReport:
    SLOT_RANGE_INDEX_SIZE = 5_184_000
    # Last epoch with a GSFA index. We stopped generating them after this epoch.
    GSFA_LAST_EPOCH = 1020

    def __init__(self):
        self.host = "https://files.old-faithful.net"
        self.txmeta_first_epoch = 10
        self.issues = []  # Track issues for summary report
        self.index_files = [
            "mainnet-cid-to-offset-and-size.index",
            "mainnet-sig-to-cid.index",
            "mainnet-sig-exists.index",
            "mainnet-slot-to-cid.index",
            "mainnet-slot-to-blocktime.index",
            "gsfa.index.tar.zstd"
        ]
        # Add tracking variables for totals
        self.total_car_count = 0
        self.total_car_size = 0
        self.total_indices_size = 0
        
    async def request(
        self, session: aiohttp.ClientSession, method: str, url: str, **kwargs
    ) -> Response:
        last_error = ""
        for attempt in range(MAX_ATTEMPTS):
            retry_after = None
            try:
                async with session.request(method, url, allow_redirects=True, **kwargs) as response:
                    if response.status in DEFINITIVE_STATUSES:
                        body = await response.read() if method != "HEAD" else None
                        return Response(response.status, dict(response.headers), body)
                    last_error = f"HTTP {response.status}"
                    retry_after = response.headers.get("Retry-After")
            except (aiohttp.ClientError, asyncio.TimeoutError) as e:
                last_error = f"{type(e).__name__}: {e}"

            if attempt + 1 == MAX_ATTEMPTS:
                break
            delay = min(MAX_BACKOFF_SECONDS, 2 ** (attempt + 1)) + random.uniform(0, 1)
            if retry_after and retry_after.isdigit():
                delay = max(delay, min(MAX_BACKOFF_SECONDS, int(retry_after)))
            await asyncio.sleep(delay)

        raise UnreliableResponse(f"{method} {url}: {last_error} after {MAX_ATTEMPTS} attempts")

    async def check_url(self, session: aiohttp.ClientSession, url: str) -> bool:
        response = await self.request(session, "HEAD", url)
        return response.status == 200

    async def fetch_text(self, session: aiohttp.ClientSession, url: str) -> Optional[str]:
        response = await self.request(session, "GET", url)
        if response.status == 200 and response.body is not None:
            return response.body.decode("utf-8", errors="replace")
        return None

    async def get_size(self, session: aiohttp.ClientSession, url: str) -> str:
        response = await self.request(session, "HEAD", url)
        if response.status == 200:
            size_bytes = int(response.headers.get('Content-Length', 0))
            if size_bytes > 0:
                size_gb = max(1, round(size_bytes / (1024 * 1024 * 1024)))
                return str(size_gb)
        return "n/a"

    async def get_slot_range_index(
        self, session: aiohttp.ClientSession, epoch: int
    ) -> tuple[str, Optional[str]]:
        url = f"{self.host}/{epoch}/epoch-{epoch}-slot-ranges.raw"
        response = await self.request(session, "HEAD", url)
        if response.status != 200:
            return "n/a", "missing slot range index"

        try:
            size = int(response.headers.get("Content-Length", 0))
        except ValueError:
            return "n/a", "failed to validate slot range index"
        if size != self.SLOT_RANGE_INDEX_SIZE:
            return "n/a", (
                f"invalid slot range index size ({size} bytes, "
                f"expected {self.SLOT_RANGE_INDEX_SIZE})"
            )

        return url, None

    async def check_gsfa_magic(self, session: aiohttp.ClientSession, epoch: int) -> bool:
        """
        Validates the GSFA index manifest by checking its magic version
        Returns True if valid, False otherwise
        """
        manifest_url = f"{self.host}/{epoch}/gsfa.manifest"
        headers = {'Range': 'bytes=8-15'}
        response = await self.request(session, "GET", manifest_url, headers=headers)
        if response.status == 206 and response.body is not None:  # Partial Content
            # Convert bytes to hex string
            hex_content = ''.join([f'{b:02X}' for b in response.body])
            return hex_content == '0500000000000000'
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
        
        # Check gsfa file existence and validate its magic version.
        # Epochs after GSFA_LAST_EPOCH have no GSFA index, so skip the check.
        if epoch <= self.GSFA_LAST_EPOCH:
            gsfa_file = self.index_files[-1]
            gsfa_exists = await self.check_url(session, f"{self.host}/{epoch}/epoch-{epoch}-{gsfa_file}")
            gsfa_valid = True
            if not gsfa_exists:
                missing_files.append("missing GSFA index file")
            else:
                gsfa_valid = await self.check_gsfa_magic(session, epoch)
                if not gsfa_valid:
                    missing_files.append("GSFA index file failed magic validation")

            # Add gsfa validation result to checks
            checks.append(gsfa_exists and gsfa_valid)

        # Add all missing files to issues if any
        if missing_files:
            self.issues.append((epoch, missing_files))

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
        if epoch <= self.GSFA_LAST_EPOCH:
            gsfa_size = await self.get_size(session, f"{self.host}/{epoch}/epoch-{epoch}-{self.index_files[-1]}")
            sizes.append(gsfa_size)

        # Convert sizes to integers, treating "n/a" as 0
        size_ints = [int(size) if size != "n/a" else 0 for size in sizes]
        
        # Sum up all sizes
        total_size = sum(size_ints)
        
        return str(total_size) if total_size > 0 else "n/a"

    async def get_epoch_data(self, session: aiohttp.ClientSession, epoch: int) -> EpochData:
        car_url = f"{self.host}/{epoch}/epoch-{epoch}.car"
        sha_url = f"{self.host}/{epoch}/epoch-{epoch}.sha256"
        b3_url = f"{self.host}/{epoch}/epoch-{epoch}.b3sum"
        poh_url = f"{self.host}/{epoch}/poh-check.log"
        txmeta_url = f"{self.host}/{epoch}/tx-metadata-check.log"

        # The slot range index is expected for every completed epoch, even if
        # other report data is unavailable.
        car_exists, slot_range_result = await asyncio.gather(
            self.check_url(session, car_url),
            self.get_slot_range_index(session, epoch),
        )
        slot_range_index, slot_range_index_issue = slot_range_result
        if not car_exists:
            return EpochData(
                epoch=epoch,
                slot_range_index=slot_range_index,
                slot_range_index_issue=slot_range_index_issue,
            )

        # Gather all data concurrently
        sha, size, poh, txmeta, indices, indices_size, b3 = await asyncio.gather(
            self.fetch_text(session, sha_url),
            self.get_size(session, car_url),
            self.fetch_text(session, poh_url),
            self.fetch_text(session, txmeta_url),
            self.get_indices(session, epoch),
            self.get_indices_size(session, epoch),
            self.fetch_text(session, b3_url),
        )

        # Construct slots.txt URL
        slots_url = f"{self.host}/{epoch}/{epoch}.slots.txt"

        return EpochData(
            epoch=epoch,
            car=car_url,
            sha=sha if sha else "n/a",
            sha_url=sha_url,
            b3=b3 if b3 else "n/a",
            b3_url=b3_url,
            size=size,
            poh=poh if poh else "n/a",
            poh_url=poh_url,
            txmeta=txmeta if txmeta else "n/a",
            txmeta_url=txmeta_url,
            indices=indices,
            indices_size=indices_size,
            slot_range_index=slot_range_index,
            slot_range_index_issue=slot_range_index_issue,
            slots_url=slots_url
        )

    def format_row(self, data: EpochData) -> str:
        car_cell = f"[epoch-{data.epoch}.car]({data.car})" if data.car != "n/a" else "❌"
        sha_cell = f"[{data.sha[:7]}]({data.sha_url})" if data.sha != "n/a" else "❌"
        size_cell = f"{data.size} GB" if data.size != "n/a" else "❌"
        b3_cell = f"[{data.b3[:7]}]({data.b3_url})" if data.b3 != "n/a" else "❌"
        # Update totals
        if data.car != "n/a":
            self.total_car_count += 1
        if data.size != "n/a":
            self.total_car_size += int(data.size)
        if data.indices_size != "n/a":
            self.total_indices_size += int(data.indices_size)

        # Special handling for earlier epochs txmeta validation
        if 0 <= data.epoch < self.txmeta_first_epoch and data.txmeta != "n/a":
            txmeta_cell = f"[★]({data.txmeta_url})"
        else:
            txmeta_cell = f"[❌]({data.txmeta_url})" if data.txmeta != "n/a" and not validate_txmeta_output(data.txmeta) else \
                         f"[✅]({data.txmeta_url})" if data.txmeta != "n/a" else "❌"

        # epoch 208 POH validation is handled differently
        if data.epoch == 208 and data.poh != "n/a":
            poh_cell = f"[★★]({data.poh_url})"
        elif data.poh != "n/a" and not validate_poh_output(data.poh):
            poh_cell = f"[❌]({data.poh_url})"
        elif data.poh != "n/a":
            poh_cell = f"[✅]({data.poh_url})"
        else:
            poh_cell = "❌"

        indices_cell = "✅" if data.indices != "n/a" else "❌"
        indices_size_cell = f"{data.indices_size} GB" if data.indices_size != "n/a" else "❌"
        slot_range_index_cell = (
            f"[✅]({data.slot_range_index})" if data.slot_range_index != "n/a" else "❌"
        )
        slots_cell = f"[{data.epoch}.slots.txt]({data.slots_url})" if data.slots_url != "n/a" else "❌"

        # Track issues for summary report
        issues = []
        if data.car == "n/a": issues.append("missing CAR")
        if data.sha == "n/a": issues.append("missing SHA")
        if data.size == "n/a": issues.append("missing size")
        # POH cannot be validated for epoch 208, 
        # see more in https://docs.old-faithful.net/validation
        if data.poh == "n/a": issues.append("missing POH check")
        elif not validate_poh_output(data.poh) and data.epoch != 208: issues.append("failed POH check")
        if data.txmeta == "n/a": issues.append("missing tx meta check")
        elif not validate_txmeta_output(data.txmeta) and not (0 <= data.epoch < self.txmeta_first_epoch):
            issues.append("failed tx meta check")
        if data.indices == "n/a": issues.append("missing indices")
        if data.indices_size == "n/a": issues.append("missing indices size")
        if data.slot_range_index_issue: issues.append(data.slot_range_index_issue)
        
        if issues:
            self.issues.append((data.epoch, issues))

        return f"| {data.epoch} | {car_cell} | {sha_cell} | {b3_cell} | {size_cell} | {txmeta_cell} | {poh_cell} | {indices_cell} | {indices_size_cell} | {slot_range_index_cell} | {slots_cell} |"

    async def get_current_epoch(self, session: aiohttp.ClientSession) -> int:
        url = 'https://api.mainnet-beta.solana.com'
        response = await self.request(
            session, "POST", url, json={"jsonrpc":"2.0","id":1, "method":"getEpochInfo"}
        )
        if response.status != 200 or response.body is None:
            raise UnreliableResponse(f"POST {url}: HTTP {response.status}")
        data = json.loads(response.body)
        if 'result' not in data:
            raise UnreliableResponse(f"POST {url}: {data.get('error')}")
        return int(data['result']['epoch'])

    async def run(self):
        # Fewer parallel connections keep the run under the host's rate limit.
        connector = aiohttp.TCPConnector(limit=20)
        timeout = aiohttp.ClientTimeout(total=60)
        async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
            await self.report(session)

    async def report(self, session: aiohttp.ClientSession):
        current_epoch = await self.get_current_epoch(session)
        epochs = range((current_epoch-1), -1, -1)  # descending order

        print("| Epoch #  | CAR  | CAR SHA256 | CAR B3 | CAR filesize | tx meta check | poh check | Indices | Indices Size | Slot Range Index | Slots |")
        print("|---|---|---|---|---|---|---|---|---|---|---|")
        print("|%s|ongoing|-|-|-|-|-|-|-|-|-|" % current_epoch)

        # concurrency levels
        chunk_size = 50  
        
        for i in range(0, len(epochs), chunk_size):
            chunk = epochs[i:i + chunk_size]
            results = await asyncio.gather(
                *[self.get_epoch_data(session, epoch) for epoch in chunk]
            )

            # Print results in order
            for result in results:
                print(self.format_row(result))

        # Print totals row
        car_size_tb = int(round(self.total_car_size / 1024, 0))
        indices_size_tb = int(round(self.total_indices_size / 1024, 0))
        total_epochs = len(epochs)
        missing_cars = total_epochs - self.total_car_count
        print(f"| **Total** | {self.total_car_count} | ({missing_cars} behind) | - | {car_size_tb:,} TB | - | - | - | {indices_size_tb:,} TB | - | - |")

        print("\n★ = tx meta validation skipped (epochs 0-%s where tx meta wasn't enabled yet)" % self.txmeta_first_epoch)
        print("\n★★ = epoch 208 POH validation is handled differently, see more in https://docs.old-faithful.net/validation")

        print("\nGSFA indexes exist up to epoch %s only. The report does not check for them after that epoch." % self.GSFA_LAST_EPOCH)

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
    try:
        asyncio.run(report.run())
    except UnreliableResponse as e:
        print(f"report aborted, data source did not answer reliably: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
