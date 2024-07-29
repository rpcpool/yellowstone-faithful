#!/usr/bin/env python3

import math
import sys
import logging
import os
import json
import datetime
import socket
import requests
from os import environ

from random import shuffle

from io import StringIO
from subprocess import check_output, CalledProcessError

import csv

"""
import triton upload clients. see rpcpool/rpcpool for detail about this script.
"""
sys.path.append(os.path.abspath("/usr/local/lib/triton-py"))
from triton_upload_clients import BunnyCDNClient, S3Client

VERSION = "0.0.1"

start_epoch_head_offset = int(604800 / 30)

# This script creates deals for an epoch stored in s3.
#
# It expects the following environment variables to be set:
#   STORAGE_ENDPOINT: the endpoint of Bunny storage
#   STORAGE_KEY: the application key for storage
#   STORAGE_KEY_ID : application key id for storage provider
#   STORAGE_NAME: the bucket name to use for the deals
#   PROVIDERS: a comma separated list of providers to use for the deals
#   REPLICATION_FACTOR: the number of times to replicate each piece
#   WALLET: the wallet to use for the deals
#   DRY_RUN: if set to true, the script will not actually create deals
#   DEALTYPE: if set to online, the script will create online deals, otherwise offline deals
#   DEALS_FOLDER: the folder to store the deals csv output file in
#   FILECOIN_RPC_ENDPOINT: the filecoin rpc endpoint
#   CID_GRAVITY_KEY: api key for cid gravity
#
# It expects the following arguments:
#   epoch: the epoch to create deals for
#   deal_type: optional, it will make a deal for car + index if not defined, otherwise you can specify "index"
#
# It expects the following files to be present in the bucket:
#   epoch/payload.cid: the cid of the payload
#   epoch/metadata.csv: the metadata csv produced by split-and-commp
#
# It will create or update the following csv files in the bucket:
#   epoch/deals.csv: the deals csv file
#
# To run this manually
# (set -a  && source '/etc/default/boost_create_deals' && python3 /usr/local/bin/boost_create_deals.py 27 index)

dry_run = environ.get("DRY_RUN") == "true"

if dry_run:
    BOOST_VERSION = 1
else:
    try:
        BOOST_VERSION = check_output(["boost", "--version"], text=True).strip()
    except CalledProcessError as e:
        print("FATAL: could not get binary version(s)", e, file=sys.stderr)
        sys.exit(1)

logging.basicConfig(level=logging.INFO)

endpoint = environ.get("STORAGE_ENDPOINT")
key_id = environ.get("STORAGE_KEY_ID")
application_key = environ.get("STORAGE_KEY")
storage_name = environ.get("STORAGE_NAME")
url_format = environ.get("PUBLIC_URL_FORMAT")
upload_client = environ.get("UPLOAD_CLIENT")
rpc_endpoint = environ.get("FILECOIN_RPC_ENDPOINT", "https://api.node.glif.io")

if upload_client == "S3":
    client = S3Client(endpoint, storage_name, url_format, key_id, application_key)
else:
    client = BunnyCDNClient(endpoint, storage_name, url_format, "", application_key)

online = environ.get("DEALTYPE") == "online"


def get_chain_head():
    response = requests.post(
        url=rpc_endpoint,
        json={"jsonrpc": "2.0", "id": 1, "method": "Filecoin.ChainHead", "params": []},
    )
    assert response.status_code == 200, f"failed to get chain head: {response.text}"

    resp = response.json()
    assert "result" in resp, f"result key not found in ChainHead response: {resp}"
    assert (
        "Height" in resp["result"]
    ), f"Height key not found in ChainHead response: {resp}"

    return resp["result"]["Height"]


def get_collateral(padded_size, verified=True):
    response = requests.post(
        url=rpc_endpoint,
        json={
            "jsonrpc": "2.0",
            "id": 1,
            "method": "Filecoin.StateDealProviderCollateralBounds",
            "params": [int(padded_size), verified, []],
        },
    )
    assert response.status_code == 200, f"failed to get chain head: {response.text}"

    resp = response.json()
    assert (
        "result" in resp
    ), f"result key not found in StateDealProviderCollateralBounds response: {resp}"
    assert (
        "Min" in resp["result"]
    ), f"Min key not found in StateDealProviderCollateralBounds response result: {resp}"

    return math.ceil(
        int(resp["result"]["Min"]) * 1.2
    )  # add 20% to the min and round up to nearest integer


def get_providers(
    piece_cid,
    size,
    padded_size,
):

    head = get_chain_head()
    start_epoch = head + start_epoch_head_offset

    provider_collateral = get_collateral(padded_size=padded_size, verified=True)

    api_key = environ.get("CID_GRAVITY_KEY")
    headers = {"X-API-KEY": api_key}

    response = requests.post(
        url="https://service.cidgravity.com/private/v1/get-best-available-providers",
        headers=headers,
        json={
            "pieceCid": piece_cid,
            "startEpoch": start_epoch,
            "duration": 1468800,
            "storagePricePerEpoch": 0,
            "providerCollateral": provider_collateral,
            "verifiedDeal": True,
            "transferSize": size,
            "transferType": "http",
            "removeUnsealedCopy": False,
        },
    )

    assert (
        response.status_code == 200
    ), f"failed to get provider from CID gravity: {response.text}"

    resp = response.json()
    assert (
        "result" in resp
    ), f"result key not found in response from CID gravity: {resp}"
    assert (
        "providers" in resp["result"]
    ), f"providers key not found in response from CID gravity: {resp}"

    providers = resp["result"]["providers"]

    assert len(providers) > 0, f"empty list of providers returned: {resp}"

    return resp["result"]["providers"]


def create_deals(metadata_obj):
    """
    Create deals for the files in the metadata object provided as an argument.

    Will attempt to lock and update `deal.csv` in the remote storage container.
    """
    metadata_reader = StringIO(metadata_obj)
    metadata_split_lines = csv.reader(metadata_reader, delimiter=",")

    next(metadata_split_lines, None)  # skip the headers

    deal_data = []
    for line in metadata_split_lines:
        file_name = os.path.basename(line[0])
        # Only allow the new metadata
        assert (
            len(line) == 5
        ), f"metadata.csv should have 5 columns, instead found f{len(line)}, {line}"

        commp_piece_cid = line[1]
        padded_size = line[2]
        payload_cid = line[3]

        check_obj = client.check_exists(epoch + "/" + file_name)
        if not check_obj[0]:
            logging.info("%s not found" % file_name)
            sys.exit(1)
        elif check_obj[1] <= 1:
            logging.info("%s size too small" % file_name)
            sys.exit(1)
        elif check_obj[1] != int(padded_size):
            logging.debug(
                "%s size mismatch %s != %s" % (file_name, check_obj[1], padded_size)
            )

        public_url = client.get_public_url(epoch + "/" + file_name)
        check_url = client.check_public_url(public_url)
        if not check_url[0]:
            logging.info("%s not accessible" % public_url)
            continue
        elif int(check_url[1]) != int(check_obj[1]):
            logging.info(
                "%s size mismatch %s != %s" % (public_url, check_url[1], check_obj[1])
            )
            continue

        deal_data_item = {
            "file_name": file_name,
            "url": public_url,
            "commp_piece_cid": commp_piece_cid,
            "file_size": check_obj[1],
            "padded_size": padded_size,
            "payload_cid": payload_cid,
        }

        deal_data.append(deal_data_item)

    replication_factor = int(environ.get("REPLICATION_FACTOR"))
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
        logging.error("lock file exists, exiting: " + lock_data)
        return 1

    deals_folder = environ.get("DEALS_FOLDER")
    deals_file = f"{deals_folder}/{epoch}_deals_{filetime}.csv"

    replications = {}
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

    with open(deals_file, "a+") as log_file:
        csv_writer = csv.DictWriter(log_file, fieldnames=fields)

        if not check_existing_deals[0]:
            csv_writer.writeheader()

        for file_item in deal_data:
            logging.info("creating deal for ")
            logging.info(file_item)

            providers = get_providers(
                piece_cid=file_item["commp_piece_cid"],
                size=file_item["file_size"],
                padded_size=file_item["padded_size"],
            )
            logging.info(f"found providers: {providers}")

            for provider in providers:

                if file_item["commp_piece_cid"] in replications:
                    if provider in replications[file_item["commp_piece_cid"]]:
                        logging.info(
                            "skipping %s, already have deal with %s"
                            % (file_item["commp_piece_cid"], provider)
                        )
                        continue

                if file_item["commp_piece_cid"] not in replications:
                    replications[file_item["commp_piece_cid"]] = []
                elif (
                    len(replications[file_item["commp_piece_cid"]])
                    >= replication_factor
                ):
                    logging.info(
                        "skipping %s, already replicated %s times"
                        % (
                            file_item["commp_piece_cid"],
                            replications[file_item["commp_piece_cid"]],
                        )
                    )
                    continue

                params = {
                    "provider": provider,
                    "commp": file_item["commp_piece_cid"],
                    "piece-size": file_item["padded_size"],
                    "car-size": file_item["file_size"],
                    "payload-cid": file_item["payload_cid"],
                    "storage-price": "0",
                    "start-epoch-head-offset": start_epoch_head_offset,
                    "verified": "true",
                    "duration": 1468800,
                    "wallet": environ.get("WALLET"),
                }
                deal_arg = "deal"
                if online:
                    params["http-url"] = file_item["url"]
                else:
                    deal_arg = "offline-deal"

                logging.info(params)
                cmd = ["boost", "--vv", "--json=1", deal_arg] + [
                    f"--{k}={v}" for k, v in params.items()
                ]

                logging.info(cmd)

                if dry_run:
                    out = '{ "dealUuid": "dry-run-uuid", "dealState": "dry-run-state"}'
                else:
                    try:
                        out = check_output(cmd, text=True).strip()
                    except CalledProcessError as e:
                        logging.warning(f"WARNING: boost deal failed for {provider}:")
                        logging.warning(e)
                        continue

                logging.info(out)
                res = json.loads(out)

                deal_output = {
                    "provider": provider,
                    "deal_uuid": res.get("dealUuid"),
                }

                replications[file_item["commp_piece_cid"]].append(provider)

                deal_output.update(file_item)
                csv_writer.writerow(deal_output)
                if provider not in deals_providers:
                    deals_providers[provider] = []
                deals_providers[provider].append(deal_output)
        log_file.close()

    if dry_run:
        logging.info("completed processing dry run mode")
    else:
        logging.info(f"uploading deals file {deals_file} to {deals_url}")
        if client.upload_file(deals_file, deals_url):
            logging.info("upload successful")
        else:
            logging.info("upload failed")

    # Print the number of replications
    logging.info("total providers: " + str(len(deals_providers)))
    for key, value in deals_providers.items():
        logging.info(f"{key} provider got {len(value)}/{len(deal_data)} deals")
    logging.info("replication summary")
    for key, value in replications.items():
        logging.info(f"{key} replicated {len(value)} times")

    if not client.delete_file(lockfile):
        logging.warning("WARNING: could not delete lock file")
        return 1

    return 0


# Code below should be agnostic to storage backend
if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise ValueError(
            "Not enough arguments. usage: ", sys.argv[0], " <epoch> [<deal_type>]"
        )

    logging.info(
        "boost create deals version %s " "(boost version: %s).", VERSION, BOOST_VERSION
    )

    epoch = sys.argv[1]

    deal_type = ""
    if len(sys.argv) == 3:
        deal_type = sys.argv[2]

    client.connect()

    # Load the payload CI
    epoch_cid = client.read_object("%s/epoch-%s.cid" % (epoch, epoch)).strip()

    logging.info("creating deals for epoch %s with payload %s", epoch, epoch_cid)

    # Load metadata csv produced by split-and-commp
    ret = 0
    if len(deal_type) == 0:
        logging.info(
            "deal type not specified, creating for both epoch objects and index files"
        )

        metadata_obj = client.read_object(epoch + "/metadata.csv")
        ret += create_deals(metadata_obj)

        logging.info("created deals for epoch files %d", ret)

        index_obj = client.read_object(epoch + "/index.csv")
        ret += create_deals(index_obj)

        logging.info("created deals for index files %d", ret)
    else:
        metadata_obj = client.read_object(epoch + "/" + deal_type + ".csv")
        ret += create_deals(metadata_obj)
        logging.info("created deals for %s files %d", deal_type, ret)

    sys.exit(ret)
