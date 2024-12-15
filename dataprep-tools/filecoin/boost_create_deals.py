#!/usr/bin/env python3

import csv
import datetime
import json
import logging
import math
import os
import socket
import sys
from dataclasses import dataclass
from io import StringIO
from subprocess import CalledProcessError, check_output
from typing import Any, Dict, List, Optional

import requests

# add triton upload clients
sys.path.append(os.path.abspath("/usr/local/lib/triton-py"))
from triton_upload_clients import BunnyCDNClient, S3Client

VERSION = "0.0.2"
DEFAULT_REPLICATION_FACTOR = 3
START_EPOCH_HEAD_OFFSET = int(604800 / 30)
DEAL_DURATION = 1468800  # 6 months

# Setup logging
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")

@dataclass
class StorageConfig:
    """Configuration for storage client"""
    endpoint: str
    key_id: str
    application_key: str
    storage_name: str
    url_format: str
    upload_client: str


@dataclass
class DealConfig:
    """Configuration for deal creation"""
    replication_factor: int
    wallet: str
    dry_run: bool
    deal_type: str
    deals_folder: str
    filecoin_rpc_endpoint: str
    cid_gravity_key: Optional[str]
    use_cid_gravity: bool
    boost_version: str
    online: bool

class DealCreationError(Exception):
    """Custom exception for deal creation errors."""
    pass

def load_storage_config() -> StorageConfig:
    """Load and validate storage configuration from environment variables."""
    try:
        endpoint = os.environ["STORAGE_ENDPOINT"]
        key_id = os.environ["STORAGE_KEY_ID"]
        application_key = os.environ["STORAGE_KEY"]
        storage_name = os.environ["STORAGE_NAME"]
        url_format = os.environ["PUBLIC_URL_FORMAT"]
        upload_client = os.environ["UPLOAD_CLIENT"]

        if not all([endpoint, key_id, application_key, storage_name, url_format, upload_client]):
            raise DealCreationError("Missing required storage environment variables")

        return StorageConfig(endpoint, key_id, application_key, storage_name, url_format, upload_client)
    except KeyError as e:
        raise DealCreationError(f"Missing required environment variable: {e}") from e


def load_deal_config() -> DealConfig:
    """Load and validate deal creation configuration from environment variables."""
    try:
        replication_factor = int(os.environ.get("REPLICATION_FACTOR", DEFAULT_REPLICATION_FACTOR))
        wallet = os.environ["WALLET"]
        dry_run = os.environ.get("DRY_RUN", "false").lower() == "true"
        deal_type = os.environ.get("DEALTYPE", "offline")
        deals_folder = os.environ["DEALS_FOLDER"]
        filecoin_rpc_endpoint = os.environ.get("FILECOIN_RPC_ENDPOINT", "https://api.node.glif.io")
        cid_gravity_key = os.environ.get("CID_GRAVITY_KEY")
        use_cid_gravity = os.environ.get("USE_CID_GRAVITY", "false").lower() == "true"
        online = os.environ.get("DEALTYPE") == "online"

        if not all([wallet, deals_folder]):
            raise DealCreationError("Missing required deal environment variables")

        if dry_run:
          boost_version = "dry-run-mode"
        else:
          try:
              boost_version = check_output(["boost", "--version"], text=True).strip()
          except CalledProcessError as e:
              raise DealCreationError(f"Could not get boost version: {e}") from e

        return DealConfig(
            replication_factor=replication_factor,
            wallet=wallet,
            dry_run=dry_run,
            deal_type=deal_type,
            deals_folder=deals_folder,
            filecoin_rpc_endpoint=filecoin_rpc_endpoint,
            cid_gravity_key=cid_gravity_key,
            use_cid_gravity=use_cid_gravity,
            boost_version=boost_version,
            online=online
          )
    except (KeyError, ValueError) as e:
        raise DealCreationError(f"Invalid deal config: {e}") from e

def get_storage_client(storage_config: StorageConfig) -> Any:
    """Initialize the storage client based on configuration."""
    if storage_config.upload_client == "S3":
        return S3Client(
            storage_config.endpoint,
            storage_config.storage_name,
            storage_config.url_format,
            storage_config.key_id,
            storage_config.application_key,
        )
    else:
        return BunnyCDNClient(
            storage_config.endpoint,
            storage_config.storage_name,
            storage_config.url_format,
            "",
            storage_config.application_key,
        )


def get_chain_head(rpc_endpoint: str) -> int:
    """Fetch the current chain head height from the Filecoin RPC."""
    try:
        response = requests.post(
            url=rpc_endpoint,
            json={"jsonrpc": "2.0", "id": 1, "method": "Filecoin.ChainHead", "params": []},
        )
        response.raise_for_status()  # Raise HTTPError for bad responses (4xx or 5xx)
        resp = response.json()
        if "result" not in resp or "Height" not in resp["result"]:
            raise DealCreationError(f"Invalid chain head response: {resp}")
        return resp["result"]["Height"]
    except requests.exceptions.RequestException as e:
        raise DealCreationError(f"Failed to get chain head: {e}") from e


def get_collateral(rpc_endpoint: str, padded_size: int, verified: bool = True) -> int:
    """Get the required collateral for a deal."""
    try:
      response = requests.post(
          url=rpc_endpoint,
          json={
              "jsonrpc": "2.0",
              "id": 1,
              "method": "Filecoin.StateDealProviderCollateralBounds",
              "params": [int(padded_size), verified, []],
          },
      )
      response.raise_for_status()
      resp = response.json()

      if "result" not in resp or "Min" not in resp["result"]:
        raise DealCreationError(f"Invalid collateral response: {resp}")

      return math.ceil(int(resp["result"]["Min"]) * 1.2)
    except requests.exceptions.RequestException as e:
        raise DealCreationError(f"Failed to get collateral: {e}") from e

def get_providers_from_cid_gravity(
        piece_cid: str, size: int, padded_size: int, deal_config: DealConfig
) -> List[str]:
    """Get a list of providers from CID gravity."""
    try:
        head = get_chain_head(deal_config.filecoin_rpc_endpoint)
        start_epoch = head + START_EPOCH_HEAD_OFFSET
        provider_collateral = get_collateral(
            rpc_endpoint=deal_config.filecoin_rpc_endpoint, padded_size=padded_size, verified=True
        )
        headers = {"X-API-KEY": deal_config.cid_gravity_key}
        response = requests.post(
            url="https://service.cidgravity.com/private/v1/get-best-available-providers",
            headers=headers,
            json={
                "pieceCid": piece_cid,
                "startEpoch": start_epoch,
                "duration": DEAL_DURATION,
                "storagePricePerEpoch": 0,
                "providerCollateral": provider_collateral,
                "verifiedDeal": True,
                "transferSize": size,
                "transferType": "http",
                "removeUnsealedCopy": False,
            },
        )
        response.raise_for_status()
        resp = response.json()

        if "result" not in resp or "providers" not in resp["result"]:
            raise DealCreationError(f"Invalid CID gravity response: {resp}")

        providers = resp["result"]["providers"]
        if not providers:
            raise DealCreationError(f"Empty list of providers returned from CID gravity: {resp}")
        return providers
    except requests.exceptions.RequestException as e:
        raise DealCreationError(f"Failed to get provider from CID gravity: {e}") from e

def get_providers(piece_cid: str, size: int, padded_size: int, deal_config: DealConfig) -> List[str]:
    """Get a list of providers from environment variables or CID gravity."""
    if not deal_config.use_cid_gravity:
        providers = os.environ.get("PROVIDERS")
        if not providers:
            raise DealCreationError("PROVIDERS environment variable must be set if not using CID gravity")
        return providers.split(",")

    if not deal_config.cid_gravity_key:
         raise DealCreationError("CID_GRAVITY_KEY environment variable must be set if using CID gravity")

    return get_providers_from_cid_gravity(piece_cid, size, padded_size, deal_config)

def validate_file_metadata(client: Any, epoch: str, file_name: str, padded_size: int) -> str:
    """Validate that the file exists and is the correct size."""
    check_obj = client.check_exists(f"{epoch}/{file_name}")
    if not check_obj[0]:
        raise DealCreationError(f"{file_name} not found")
    if check_obj[1] <= 1:
        raise DealCreationError(f"{file_name} size too small")
    if check_obj[1] != int(padded_size):
        raise DealCreationError(
            f"{file_name} size mismatch: {check_obj[1]} != {padded_size}"
        )

    public_url = client.get_public_url(f"{epoch}/{file_name}")
    check_url = client.check_public_url(public_url)
    if not check_url[0]:
        raise DealCreationError(f"{public_url} not accessible")
    if int(check_url[1]) != int(check_obj[1]):
        raise DealCreationError(
            f"{public_url} size mismatch {check_url[1]} != {check_obj[1]}"
        )
    return public_url

def process_metadata_line(client: Any, epoch: str, line: List[str]) -> Dict[str, Any]:
    """Process a single line from the metadata CSV."""
    if len(line) != 5:
       raise DealCreationError(f"metadata.csv should have 5 columns, instead found {len(line)}, {line}")

    file_name = os.path.basename(line[0])
    commp_piece_cid = line[1]
    payload_cid = line[2]
    padded_size = int(line[3])
    public_url = validate_file_metadata(client, epoch, file_name, padded_size)

    return {
        "file_name": file_name,
        "url": public_url,
        "commp_piece_cid": commp_piece_cid,
        "file_size": padded_size,
        "padded_size": padded_size,
        "payload_cid": payload_cid,
    }

def get_existing_deals(client: Any, deals_url: str, fields: List[str]) -> Dict[str, List[str]]:
    """Read existing deals from the remote storage."""
    replications = {}
    deals_file = "deals.csv"  # use a tmp file
    check_existing_deals = client.check_exists(deals_url)
    if check_existing_deals[0]:
        client.download_file(deals_url, deals_file)
        with open(deals_file, "r") as csv_file:
            reader = csv.DictReader(csv_file, fieldnames=fields)
            next(reader, None)  # skip the headers
            for row in reader:
                if row["commp_piece_cid"] not in replications:
                    replications[row["commp_piece_cid"]] = []
                replications[row["commp_piece_cid"]].append(row["provider"])
            csv_file.close()
        os.remove(deals_file)
    return replications

def create_boost_deal(
    deal_config: DealConfig,
    file_item: Dict[str, Any],
    provider: str
) -> Dict[str, Any]:
    """Create a deal using the boost CLI."""
    params = {
        "provider": provider,
        "commp": file_item["commp_piece_cid"],
        "piece-size": file_item["padded_size"],
        "car-size": file_item["file_size"],
        "payload-cid": file_item["payload_cid"],
        "storage-price": "0",
        "start-epoch-head-offset": START_EPOCH_HEAD_OFFSET,
        "verified": "true",
        "duration": DEAL_DURATION,
        "wallet": deal_config.wallet,
    }
    deal_arg = "deal"
    if deal_config.online:
        params["http-url"] = file_item["url"]
    else:
        deal_arg = "offline-deal"

    cmd = ["boost", "--vv", "--json=1", deal_arg] + [
        f"--{k}={v}" for k, v in params.items()
    ]
    logging.info(f"Executing boost command: {' '.join(cmd)}")

    if deal_config.dry_run:
        out = '{ "dealUuid": "dry-run-uuid", "dealState": "dry-run-state"}'
    else:
        try:
          out = check_output(cmd, text=True).strip()
        except CalledProcessError as e:
            logging.warning(f"WARNING: boost deal failed for {provider}: {e}")
            raise DealCreationError(f"boost deal failed for {provider}: {e}") from e

    try:
        res = json.loads(out)
        if not res.get("dealUuid"):
             raise DealCreationError(f"Invalid boost output no deal uuid: {out}")
        return {
            "provider": provider,
            "deal_uuid": res.get("dealUuid"),
        }
    except json.JSONDecodeError as e:
        raise DealCreationError(f"Failed to decode JSON output from boost: {e}, {out}") from e

def create_deals(client: Any, epoch: str, deal_config: DealConfig, metadata_obj: str, deal_type: str):
    """
    Create deals for the files in the metadata object provided as an argument.
    """
    metadata_reader = StringIO(metadata_obj)
    metadata_split_lines = csv.reader(metadata_reader, delimiter=",")

    next(metadata_split_lines, None)  # skip the headers

    deal_data = []
    for line in metadata_split_lines:
        try:
            deal_data_item = process_metadata_line(client, epoch, line)
            deal_data.append(deal_data_item)
        except DealCreationError as e:
            logging.warning(f"Skipping line due to error: {e}")
            continue

    deals_providers: Dict[str, List[Dict[str,Any]]] = {}
    fields = [
        "provider",
        "deal_uuid",
        "file_name",
        "url",
        "commp_piece_cid",
        "file_size",
        "padded_size",
        "payload_cid",
    ]

    deals_url = f"{epoch}/deals.csv"
    lockfile = f"{epoch}/deals.csv.lock"

    if deal_type == "index":
        # avoid overwritting deal files when doing index only deals
        deals_url = f"{epoch}/deals-index.csv"
        lockfile = f"{epoch}/deals-index.csv.lock"

    filetime = datetime.datetime.now().strftime("%Y%m%d%H%M%S")

    # Create a lock file for the epoch to ensure that no one else is working on it
    if not client.check_exists(lockfile)[0]:
        client.upload_obj(StringIO(socket.gethostname() + "_" + filetime), lockfile)
    else:
        lock_data = client.read_object(lockfile)
        raise DealCreationError(f"Lock file exists, exiting: {lock_data}")

    deals_file = f"{deal_config.deals_folder}/{epoch}_deals_{filetime}.csv"

    replications = get_existing_deals(client, deals_url, fields)


    with open(deals_file, "a+") as log_file:
        csv_writer = csv.DictWriter(log_file, fieldnames=fields)

        if not client.check_exists(deals_url)[0]:
            csv_writer.writeheader()

        for file_item in deal_data:
            logging.info(f"Creating deals for {file_item}")

            if file_item["commp_piece_cid"] not in replications:
                replications[file_item["commp_piece_cid"]] = []

            while len(replications[file_item["commp_piece_cid"]]) < deal_config.replication_factor:
                try:
                  providers = get_providers(
                      piece_cid=file_item["commp_piece_cid"],
                      size=file_item["file_size"],
                      padded_size=file_item["padded_size"],
                      deal_config=deal_config
                  )
                  logging.info(f"found providers: {providers}")
                except DealCreationError as e:
                    logging.warning(f"Skipping deal creation for {file_item['file_name']}: {e}")
                    break  # Skip the deal if no provider can be found

                for provider in providers:
                    if file_item["commp_piece_cid"] in replications:
                        if provider in replications[file_item["commp_piece_cid"]]:
                            logging.info(
                                f"skipping {file_item['commp_piece_cid']}, already have deal with {provider}"
                            )
                            continue

                    if (len(replications[file_item["commp_piece_cid"]])
                        >= deal_config.replication_factor
                    ):
                        logging.info(
                            f"skipping {file_item['commp_piece_cid']}, already replicated {len(replications[file_item['commp_piece_cid']])} times"
                        )
                        continue

                    try:
                        deal_output = create_boost_deal(deal_config, file_item, provider)
                        deal_output.update(file_item)
                        csv_writer.writerow(deal_output)
                        replications[file_item["commp_piece_cid"]].append(provider)
                        if provider not in deals_providers:
                            deals_providers[provider] = []
                        deals_providers[provider].append(deal_output)

                    except DealCreationError as e:
                        logging.warning(f"skipping provider {provider} due to error: {e}")
                        continue
            log_file.close()

    if deal_config.dry_run:
      logging.info("completed processing dry run mode")
    else:
        logging.info(f"uploading deals file {deals_file} to {deals_url}")
        if client.upload_file(deals_file, deals_url):
            logging.info("upload successful")
        else:
            logging.error("upload failed")

    # Print the number of replications
    logging.info(f"total providers: {len(deals_providers)}")
    for key, value in deals_providers.items():
        logging.info(f"{key} provider got {len(value)}/{len(deal_data)} deals")

    logging.info("replication summary")
    for key, value in replications.items():
        logging.info(f"{key} replicated {len(value)} times")

    if not client.delete_file(lockfile):
        logging.warning("WARNING: could not delete lock file")
        return 1

    return 0

def main():
    """Main function to execute the script."""
    if len(sys.argv) < 2:
        print("Not enough arguments. usage: ", sys.argv[0], " <epoch> [<deal_type>]", file=sys.stderr)
        sys.exit(1)

    epoch = sys.argv[1]
    deal_type = sys.argv[2] if len(sys.argv) == 3 else ""
    logging.info(f"boost create deals version {VERSION} (boost version: {deal_config.boost_version}).")

    try:
        storage_config = load_storage_config()
        deal_config = load_deal_config()
        client = get_storage_client(storage_config)
        client.connect()

        epoch_cid = client.read_object(f"{epoch}/epoch-{epoch}.cid").strip()
        logging.info(f"creating deals for epoch {epoch} with payload {epoch_cid}")
    except DealCreationError as e:
      logging.error(f"Failed to initialize: {e}")
      sys.exit(1)

    ret = 0

    try:
        if not deal_type:
            logging.info("deal type not specified, creating for both epoch objects and index files")
            metadata_obj = client.read_object(f"{epoch}/metadata.csv")
            ret += create_deals(client, epoch, deal_config, metadata_obj, "")
            logging.info(f"created deals for epoch files with return code {ret}")

            index_obj = client.read_object(f"{epoch}/index.csv")
            ret += create_deals(client, epoch, deal_config, index_obj, "index")
            logging.info(f"created deals for index files with return code {ret}")

        else:
            metadata_obj = client.read_object(f"{epoch}/{deal_type}.csv")
            ret += create_deals(client, epoch, deal_config, metadata_obj, deal_type)
            logging.info(f"created deals for {deal_type} files with return code {ret}")
    except DealCreationError as e:
        logging.error(f"Failed to create deals: {e}")
        ret = 1
    except Exception as e:
        logging.error(f"An unexpected error occurred: {e}")
        ret = 1
    finally:
         sys.exit(ret)


if __name__ == "__main__":
    main()
