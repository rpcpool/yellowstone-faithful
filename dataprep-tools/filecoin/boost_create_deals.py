#!/usr/bin/env python3

import sys
import logging
import os
import json
import datetime
import socket
import requests
from os import environ

from io import StringIO
from subprocess import check_output, CalledProcessError

import csv

"""
Import triton upload clients. See rpcpool/rpcpool for detail about this script.
"""
sys.path.append(os.path.abspath("/usr/local/lib/triton-py"))
from triton_upload_clients import BunnyCDNClient, S3Client

VERSION = "0.0.2"  # Increment version for changes

start_epoch_head_offset = int(604800 / 30)
default_replication_factor = 3

# --- Configuration ---
DRY_RUN = environ.get("DRY_RUN") == "true"
STORAGE_ENDPOINT = environ.get("STORAGE_ENDPOINT")
STORAGE_KEY_ID = environ.get("STORAGE_KEY_ID")
STORAGE_KEY = environ.get("STORAGE_KEY")
STORAGE_NAME = environ.get("STORAGE_NAME")
PUBLIC_URL_FORMAT = environ.get("PUBLIC_URL_FORMAT")
UPLOAD_CLIENT = environ.get("UPLOAD_CLIENT")
FILECOIN_RPC_ENDPOINT = environ.get("FILECOIN_RPC_ENDPOINT", "https://api.node.glif.io")
DEALTYPE = environ.get("DEALTYPE")
DEALS_FOLDER = environ.get("DEALS_FOLDER")
CID_GRAVITY_KEY = environ.get("CID_GRAVITY_KEY")
USE_CID_GRAVITY = environ.get("USE_CID_GRAVITY")
PROVIDERS_ENV = environ.get("PROVIDERS")  # Renamed to avoid shadowing
REPLICATION_FACTOR_ENV = environ.get("REPLICATION_FACTOR")
WALLET = environ.get("WALLET")

# --- Logging Setup ---
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(filename)s:%(lineno)d - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


# --- Version Check ---
BOOST_VERSION = None
if not DRY_RUN:
    try:
        BOOST_VERSION = check_output(["boost", "--version"], text=True).strip()
        logger.info(f"Boost version: {BOOST_VERSION}")
    except FileNotFoundError:
        logger.error("boost command not found. Please ensure boost is installed and in PATH.")
        sys.exit(1)
    except CalledProcessError as e:
        logger.error(f"Error checking boost version: {e}")
        sys.exit(1)


# --- Input Validation ---
if not STORAGE_ENDPOINT:
    logger.error("STORAGE_ENDPOINT environment variable is not set.")
    sys.exit(1)
if not STORAGE_KEY:
    logger.error("STORAGE_KEY environment variable is not set.")
    sys.exit(1)
if not STORAGE_NAME:
    logger.error("STORAGE_NAME environment variable is not set.")
    sys.exit(1)
if not WALLET:
    logger.error("WALLET environment variable is not set.")
    sys.exit(1)
if not DEALS_FOLDER:
    logger.error("DEALS_FOLDER environment variable is not set.")
    sys.exit(1)
if USE_CID_GRAVITY and not CID_GRAVITY_KEY:
    logger.error("USE_CID_GRAVITY is true but CID_GRAVITY_KEY is not set.")
    sys.exit(1)
if not USE_CID_GRAVITY and not PROVIDERS_ENV:
    logger.error("USE_CID_GRAVITY is false but PROVIDERS environment variable is not set.")
    sys.exit(1)


# --- Client Initialization ---
if UPLOAD_CLIENT == "S3":
    client = S3Client(STORAGE_ENDPOINT, STORAGE_NAME, PUBLIC_URL_FORMAT, STORAGE_KEY_ID, STORAGE_KEY)
    logger.info("Using S3 client.")
else:
    client = BunnyCDNClient(STORAGE_ENDPOINT, STORAGE_NAME, PUBLIC_URL_FORMAT, "", STORAGE_KEY)
    logger.info("Using BunnyCDN client.")

online = DEALTYPE == "online"
replication_factor = int(REPLICATION_FACTOR_ENV or default_replication_factor)  # Use env var or default


def get_chain_head():
    """Fetches the latest chain head height from the Filecoin RPC endpoint."""
    try:
        response = requests.post(
            url=FILECOIN_RPC_ENDPOINT,
            json={"jsonrpc": "2.0", "id": 1, "method": "Filecoin.ChainHead", "params": []},
            timeout=10  # Add timeout to prevent indefinite hanging
        )
        response.raise_for_status()  # Raise HTTPError for bad responses (4xx or 5xx)
        resp = response.json()

        if "error" in resp:
            raise Exception(f"RPC error: {resp['error']}")
        if "result" not in resp or "Height" not in resp["result"]:
            raise ValueError(f"Unexpected response from ChainHead: {resp}")

        return resp["result"]["Height"]

    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to get chain head from {FILECOIN_RPC_ENDPOINT}: {e}")
        raise
    except json.JSONDecodeError as e:
        logger.error(f"Failed to decode JSON response from ChainHead: {e}")
        raise
    except Exception as e:
        logger.error(f"Error getting chain head: {e}")
        raise


def get_providers(piece_cid, size, padded_size):
    """
    Retrieves a list of providers for a given piece CID.

    Uses CID Gravity if enabled, otherwise uses the PROVIDERS environment variable.
    """
    if not USE_CID_GRAVITY:
        providers_str = PROVIDERS_ENV
        providers = [p.strip() for p in providers_str.split(",")]
        logger.info(f"Using providers from PROVIDERS environment variable: {providers}")
        return providers

    head = get_chain_head()
    start_epoch = head + start_epoch_head_offset

    headers = {"X-API-KEY": CID_GRAVITY_KEY}
    payload = {
        "pieceCid": piece_cid,
        "startEpoch": start_epoch,
        "duration": 1468800,
        "storagePricePerEpoch": 0,
        "verifiedDeal": True,
        "transferSize": size,
        "transferType": "http",
        "removeUnsealedCopy": False,
    }

    try:
        response = requests.post(
            url="https://service.cidgravity.com/private/v1/get-best-available-providers",
            headers=headers,
            json=payload,
            timeout=20 # Add timeout for CID Gravity request
        )
        response.raise_for_status()
        resp = response.json()

        if "error" in resp and resp["error"] is not None:
            raise Exception(f"CID Gravity API error: {resp['error']}")
        if "result" not in resp or "providers" not in resp["result"]:
            raise ValueError(f"Unexpected response from CID Gravity: {resp}")

        providers = resp["result"]["providers"]
        reason = resp["result"].get("reason")
        if not providers:
            if reason == "NO_PROVIDERS_AVAILABLE":
                logger.warning(f"No providers currently available for piece CID {piece_cid} (Reason: {reason})")
            else:
                logger.warning(f"No providers found for piece CID {piece_cid}" +
                             (f" (Reason: {reason})" if reason else ""))
        else:
            logger.info(f"Found {len(providers)} providers from CID gravity for {piece_cid}.")

        return providers

    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to get providers from CID gravity for {piece_cid}: {e}")
        return []  # Return empty list on failure, handle gracefully later
    except json.JSONDecodeError as e:
        logger.error(f"Failed to decode JSON response from CID Gravity: {e}")
        return []
    except Exception as e:
        logger.error(f"Error getting providers from CID gravity: {e}")
        return []


def process_metadata_line(line, epoch_str):
    """Processes a single line from the metadata CSV and returns deal data."""
    if len(line) != 5:
        raise ValueError(f"metadata.csv should have 5 columns, found {len(line)}, line: {line}")

    file_name = os.path.basename(line[0])
    commp_piece_cid = line[1]
    payload_cid = line[2]
    padded_size = line[3]

    check_obj = client.check_exists(f"{epoch_str}/{file_name}")
    if not check_obj[0]:
        raise FileNotFoundError(f"{file_name} not found in storage.")
    if check_obj[1] <= 1:
        raise ValueError(f"{file_name} size too small.")
    if check_obj[1] != int(padded_size):
        logger.debug(f"{file_name} size mismatch: storage {check_obj[1]}, metadata {padded_size}")

    public_url = client.get_public_url(f"{epoch_str}/{file_name}")
    check_url = client.check_public_url(public_url)
    if not check_url[0]:
        raise ValueError(f"{public_url} not accessible.")
    if int(check_url[1]) != int(check_obj[1]):
        raise ValueError(f"{public_url} size mismatch: public URL {check_url[1]}, storage {check_obj[1]}")

    return {
        "file_name": file_name,
        "url": public_url,
        "commp_piece_cid": commp_piece_cid,
        "file_size": check_obj[1],
        "padded_size": padded_size,
        "payload_cid": payload_cid,
    }


def execute_boost_deal(deal_arg, params):
    """Executes the boost deal command and returns the output."""
    cmd = ["boost", "--vv", "--json=1", deal_arg] + [f"--{k}={v}" for k, v in params.items()]
    logger.info(f"Executing boost command: {' '.join(cmd)}")

    if DRY_RUN:
        logger.info("Dry run mode: skipping boost execution.")
        return '{ "dealUuid": "dry-run-uuid", "dealState": "dry-run-state"}'

    try:
        out = check_output(cmd, text=True, stderr=sys.stderr).strip() # Capture stderr for better debugging
        logger.debug(f"Boost command output: {out}")
        return out
    except CalledProcessError as e:
        logger.warning(f"WARNING: boost deal failed for provider {params.get('provider', 'unknown')}:")
        logger.warning(f"Return code: {e.returncode}")
        logger.warning(f"Stdout: {e.stdout}")
        logger.warning(f"Stderr: {e.stderr}")
        return None # Indicate failure to process the output


def get_existing_replications(check_existing_deals, deals_url, deals_file, fields):
    replications = {}
    if check_existing_deals[0]:
        try:
            client.download_file(deals_url, deals_file)
            with open(deals_file, "r") as csv_file:
                reader = csv.DictReader(csv_file, fieldnames=fields)
                next(reader, None)  # skip header
                for row in reader:
                    if row["commp_piece_cid"] not in replications:
                        replications[row["commp_piece_cid"]] = []
                    replications[row["commp_piece_cid"]].append(row["provider"])
        except Exception as e:
            logger.warning(f"Error reading existing deals file {deals_url}: {e}. Proceeding with potentially duplicate deals.")
            replications = {} # Reset replications to avoid incorrect skipping
    else:
        logger.info(f"No existing deals file found at {deals_url}. Creating a new one.")

    return replications


def create_deals_for_metadata(metadata_obj, epoch_str, deal_type_suffix=""):
    """
    Creates deals based on the provided metadata CSV content.

    Handles locking, deal CSV management, provider selection, and boost command execution.
    """
    metadata_reader = StringIO(metadata_obj)
    metadata_split_lines = csv.reader(metadata_reader, delimiter=",")
    next(metadata_split_lines, None)  # skip the headers

    deal_data = []
    for line in metadata_split_lines:
        try:
            deal_data_item = process_metadata_line(line, epoch_str)
            deal_data.append(deal_data_item)
        except (FileNotFoundError, ValueError) as e:
            logger.error(f"Skipping line due to metadata processing error: {e}, line: {line}")
            continue # Skip to next line in metadata if error occurs

    if not deal_data:
        logger.info("No valid deal data found in metadata.csv. Skipping deal creation.")
        return 0

    deals_providers = {}
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

    deals_url = f"{epoch_str}/deals{deal_type_suffix}.csv"
    lockfile = f"{epoch_str}/deals{deal_type_suffix}.csv.lock"
    deals_file = os.path.join(DEALS_FOLDER, f"{epoch_str}_deals{deal_type_suffix}_{datetime.datetime.now().strftime('%Y%m%d%H%M%S')}.csv")

    # --- Locking ---
    lock_acquired = False
    try:
        if not client.check_exists(lockfile)[0]:
            client.upload_obj(StringIO(f"{socket.gethostname()}_{datetime.datetime.now().strftime('%Y%m%d%H%M%S')}"), lockfile)
            logger.info(f"Lock file created: {lockfile}")
            lock_acquired = True
        else:
            lock_data = client.read_object(lockfile)
            logger.error(f"Lock file exists, another process might be running. Exiting. Lock data: {lock_data}")
            return 1 # Indicate failure due to lock
    except Exception as e:
        logger.error(f"Error handling lock file {lockfile}: {e}")
        return 1

    if not lock_acquired:
        return 1 # Exit if lock not acquired

    check_existing_deals = client.check_exists(deals_url)
    replications = get_existing_replications(check_existing_deals, deals_file, deals_url, fields)

    deals_created_count = 0
    try:
        with open(deals_file, "a+", newline='') as log_file: # newline='' to prevent extra blank rows
            csv_writer = csv.DictWriter(log_file, fieldnames=fields)
            if not check_existing_deals[0]:
                csv_writer.writeheader()

            for file_item in deal_data:
                logger.info(f"Creating deals for file: {file_item['file_name']}")
                providers = get_providers(
                    piece_cid=file_item["commp_piece_cid"],
                    size=file_item["file_size"],
                    padded_size=file_item["padded_size"],
                )
                if not providers:
                    logger.warning(f"No providers found for {file_item['commp_piece_cid']}. Skipping deal creation for this file.")
                    continue # Skip to next file if no providers are available

                logger.info(f"Found {len(providers)} providers: {providers}")

                for provider in providers:
                    if file_item["commp_piece_cid"] not in replications:
                        replications[file_item["commp_piece_cid"]] = []
                    if not USE_CID_GRAVITY:
                        if provider in replications[file_item["commp_piece_cid"]]:
                            logger.info(f"Skipping deal for {file_item['commp_piece_cid']} with {provider}, already has a deal.")
                            continue
                        elif len(replications[file_item["commp_piece_cid"]]) >= replication_factor:
                            logger.info(f"Skipping deal for {file_item['commp_piece_cid']}, already replicated {len(replications[file_item['commp_piece_cid']])} times (replication factor: {replication_factor}).")
                            continue

                    params = {
                        "provider": provider,
                        "commp": file_item["commp_piece_cid"],
                        "piece-size": file_item["padded_size"],
                        "payload-cid": file_item["payload_cid"],
                        "storage-price": "0",
                        "start-epoch-head-offset": start_epoch_head_offset,
                        "verified": "true",
                        "duration": 1468800,
                        "wallet": WALLET,
                    }
                    if online:
                        params["http-url"] = file_item["url"]
                        params["car-size"] = file_item["file_size"]
                        deal_arg = "deal"
                    else:
                        deal_arg = "offline-deal"


                    boost_output = execute_boost_deal(deal_arg, params)
                    if boost_output: # Only process if boost command was successful
                        try:
                            res = json.loads(boost_output)
                            deal_uuid = res.get("dealUuid")
                            if deal_uuid:
                                deal_output = {
                                    "provider": provider,
                                    "deal_uuid": deal_uuid,
                                }
                                replications[file_item["commp_piece_cid"]].append(provider)
                                deal_output.update(file_item)
                                csv_writer.writerow(deal_output)
                                deals_created_count += 1

                                if provider not in deals_providers:
                                    deals_providers[provider] = []
                                deals_providers[provider].append(deal_output)
                            else:
                                logger.warning(f"No dealUuid found in boost output for provider {provider}. Output: {boost_output}")

                        except json.JSONDecodeError as e:
                            logger.error(f"Failed to decode JSON output from boost command for provider {provider}: {e}. Output: {boost_output}")

    except Exception as e:
        logger.error(f"Error writing to deals file {deals_file}: {e}")
        return 1 # Indicate file writing error
    finally: # Ensure lock is released even if errors occur
        if lock_acquired:
            try:
                if not DRY_RUN:
                    if not client.delete_file(lockfile):
                        logger.warning(f"WARNING: Could not delete lock file {lockfile}. Please delete it manually if necessary.")
                    else:
                        logger.info(f"Lock file {lockfile} deleted.")
            except Exception as e:
                logger.warning(f"WARNING: Error deleting lock file {lockfile}: {e}")


    if DRY_RUN:
        logger.info("Completed processing in dry run mode.")
    else:
        logger.info(f"Uploading deals file {deals_file} to {deals_url}")
        if client.upload_file(deals_file, deals_url):
            logger.info("Deals file upload successful.")
        else:
            logger.error("Deals file upload failed.")
            return 1 # Indicate upload failure

    # --- Summary Logging ---
    logger.info("Deal creation summary:")
    logger.info(f"Total deals created: {deals_created_count}")
    logger.info(f"Total providers used: {len(deals_providers)}")
    for provider, deals in deals_providers.items():
        logger.info(f"Provider {provider}: {len(deals)}/{len(deal_data)} deals.")
    logger.info("Replication summary:")
    for piece_cid, providers in replications.items():
        logger.info(f"Piece CID {piece_cid}: replicated {len(providers)} times.")

    return 0 # Indicate success


if __name__ == "__main__":
    if len(sys.argv) < 2:
        logger.error(f"Usage: {sys.argv[0]} <epoch> [<deal_type>]")
        sys.exit(1)

    logger.info(f"Boost create deals version {VERSION} (boost version: {BOOST_VERSION}).")

    epoch = sys.argv[1]
    deal_type = sys.argv[2] if len(sys.argv) == 3 else ""

    client.connect()
    logger.info(f"Connected to storage client: {client}")

    try:
        epoch_cid = client.read_object(f"{epoch}/epoch-{epoch}.cid").strip()
        logger.info(f"Creating deals for epoch {epoch} with payload CID {epoch_cid}")
    except Exception as e:
        logger.error(f"Error reading epoch CID for epoch {epoch}: {e}")
        sys.exit(1)

    ret = 0
    if not deal_type:
        logger.info("Deal type not specified, creating deals for both epoch objects and index files.")
        try:
            metadata_obj = client.read_object(f"{epoch}/metadata.csv")
            ret += create_deals_for_metadata(metadata_obj, epoch)
            logger.info(f"Created deals for epoch files. Result: {ret}")
        except Exception as e:
            logger.error(f"Error processing epoch metadata: {e}")
            ret += 1

        try:
            index_obj = client.read_object(f"{epoch}/index.csv")
            ret += create_deals_for_metadata(index_obj, epoch, "-index")
            logger.info(f"Created deals for index files. Result: {ret}")
        except Exception as e:
            logger.error(f"Error processing index metadata: {e}")
            ret += 1
    else:
        try:
            metadata_obj = client.read_object(f"{epoch}/{deal_type}.csv")
            ret += create_deals_for_metadata(metadata_obj, epoch, f"-{deal_type}")
            logger.info(f"Created deals for {deal_type} files. Result: {ret}")
        except Exception as e:
            logger.error(f"Error processing {deal_type} metadata: {e}")
            ret += 1

    sys.exit(ret)
