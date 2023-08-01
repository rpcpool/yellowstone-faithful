#!/usr/bin/env python3

import sys
import logging
import os

from os import environ

from io import StringIO

import json
import boto3
from subprocess import check_output, CalledProcessError

import csv
from botocore.exceptions import ClientError
from botocore.config import Config

import datetime


from prometheus_client import CollectorRegistry, write_to_textfile, Gauge, Counter

"""
import triton upload clients. see rpcpool/rpcpool for detail about this script.
"""
sys.path.append(os.path.abspath("/usr/local/lib/triton-py"))
from triton_upload_clients import BunnyCDNClient, S3Client

logging.basicConfig(level=logging.INFO)

fields = ['provider', 'deal_uuid', 'file_name', 'url', 'commp_piece_cid', 'file_size', 'padded_size', 'payload_cid']

if __name__ == '__main__':
    try:
        REPLICATION_FACTOR = 3
        endpoint = environ.get('STORAGE_ENDPOINT')
        key_id = environ.get('STORAGE_KEY_ID')
        application_key = environ.get('STORAGE_KEY')
        storage_name = environ.get('STORAGE_NAME')
        url_format = environ.get('PUBLIC_URL_FORMAT')
        upload_client = environ.get('UPLOAD_CLIENT')
        deals_folder = environ.get('DEALS_FOLDER')

        if upload_client == "S3":
            client = S3Client(endpoint, storage_name, url_format, key_id, application_key)
        else:
            client = BunnyCDNClient(endpoint, storage_name, url_format, "", application_key)

        if len(sys.argv) < 2:
            raise ValueError("Not enough arguments. usage: ", sys.argv[0], " <epoch>")

        client.connect()

        epoch_arg = sys.argv[1]
        epochs = []

        # if we've specified all the epochs then go through the directory and fetch all deals.csv
        if epoch_arg == "all":
            epoch_dir = client.get_directory("")
            for item in epoch_dir:
                logging.debug(item)
                if 'ObjectName' in item:
                    exists = client.check_exists(item['ObjectName'] + '/deals.csv')
                    if exists[0]:
                        # only add each epoch once
                        if item['ObjectName'] not in epochs:
                            epochs.append(item['ObjectName'])
                else:
                    logging.debug("invalid directory object")
        else:
            epochs = epoch_arg.split(',')

        registry = CollectorRegistry()
        g_provider_failures = Counter('storage_provider_failure', 'Provider failures', ['provider'], registry=registry)
        g_provider_sealing_failure = Counter('storage_provider_sealing_failure', 'Provider sealing failures', ['provider'], registry=registry)
        deal_summary = []

        for epoch in epochs:
            epoch_registry = CollectorRegistry()
            g_deals = Gauge('epoch_deals', 'Deals', ['epoch'], registry=epoch_registry)
            g_unique_pieces = Gauge('epoch_pieces', 'Epoch Pieces', ['epoch','type'], registry=epoch_registry)
            g_pieces = Gauge('epoch_pieces_replications', 'Individual deal pieces', ['epoch', 'count'], registry=epoch_registry)
            g_unknown_pieces = Gauge('epoch_unknown_pieces', 'Unknown pieces found in deals', ['epoch'], registry=epoch_registry)
            g_invalid_deals = Gauge('epoch_invalid_deals', 'Invalid deals', ['epoch'], registry=epoch_registry)
            g_providers = Gauge('epoch_providers', 'Providers', ['epoch'], registry=epoch_registry)
            g_deal_status = Gauge('epoch_deal_status', 'Deal status', ['epoch', 'status'], registry=epoch_registry)
            g_deal_sealing_status = Gauge('epoch_deals_sealing_status', 'Deal sealing status', ['epoch', 'status'], registry=epoch_registry)
            g_deals_failed_loading = Gauge('epoch_deals_loading_failed', 'Deals failed loading', ['epoch','error'], registry=epoch_registry)
            g_epoch_info = Gauge('warehouse_epoch_info', 'Epoch info', ['epoch', 'epoch_root_cid', 'index_root_cid'], registry=epoch_registry)

            epoch_pieces = {}
            regular_pieces = 0
            index_pieces = 0

            epoch_root_cid = ""
            epoch_cid_filename = "%s/epoch-%s.cid" % (epoch, epoch)
            if client.check_exists(epoch_cid_filename)[0] == True:
                epoch_root_cid = client.read_object(epoch_cid_filename).strip()

            # @TODO refactor
            if client.check_exists(epoch + '/metadata.csv')[0] == True:
                metadata_obj = client.read_object(epoch + '/metadata.csv')
                metadata_reader = StringIO(metadata_obj)
                metadata_split_lines = csv.reader(metadata_reader, delimiter=',')

                next(metadata_split_lines, None)  # skip the headers
                for line in metadata_split_lines:
                    if len(line) == 5:
                        commp_piece_cid = line[3]
                    elif len(line) == 6:
                        if epoch_root_cid == "":
                            epoch_root_cid = line[3]
                        elif epoch_root_cid != line[3]:
                            logging.error("epoch root cid mismatch %s %s" % (epoch_root_cid, line[3]))
                        commp_piece_cid = line[4]
                    else:
                        logging.error("incorrect line length %d" % line(line))
                        sys.exit(1)
                    epoch_pieces[commp_piece_cid] = 0

                regular_pieces = len(epoch_pieces)

            index_root_cid = ""
            if client.check_exists(epoch + '/index.csv')[0] == True:
                metadata_obj = client.read_object(epoch + '/index.csv')
                metadata_reader = StringIO(metadata_obj)
                metadata_split_lines = csv.reader(metadata_reader, delimiter=',')

                next(metadata_split_lines, None)  # skip the headers
                for line in metadata_split_lines:
                    if len(line) == 5:
                        commp_piece_cid = line[3]
                    elif len(line) == 6:
                        if index_root_cid == "":
                            index_root_cid = line[3]
                        elif index_root_cid != line[3]:
                            logging.error("index root cid mismatch %s %s" % (index_root_cid, line[3]))
                        commp_piece_cid = line[4]
                    else:
                        logging.error("incorrect line length %d" % line(line))
                        sys.exit(1)
                    epoch_pieces[commp_piece_cid] = 0
                index_pieces = len(epoch_pieces)-regular_pieces

            g_epoch_info.labels(epoch=epoch,epoch_root_cid=epoch_root_cid,index_root_cid=index_root_cid).set(1)
            g_unique_pieces.labels(epoch=epoch,type="data").set(regular_pieces)
            g_unique_pieces.labels(epoch=epoch,type="index").set(index_pieces)

            logging.debug("epoch pieces: %d" % len(epoch_pieces))

            deals_obj = client.read_object(epoch + '/deals.csv')

            deals_file = StringIO(deals_obj)

            reader = csv.DictReader(deals_file, fieldnames = fields)

            next(reader, None)  # skip the headers

            deal_status = []
            deal_statuses = {}
            deal_sealing_statuses = {}

            deal_providers = []

            fully_replicated_pieces = 0

            for deal in reader:
                params = {
                    'provider': deal['provider'],
                    'deal-uuid': deal['deal_uuid'],
                    'wallet': environ.get('WALLET'),
                }

                logging.debug(params)
                cmd = [ 'boost',
                    '--vv',
                    '--json=1',
                    'deal-status' ] + [ f"--{k}={v}" for k, v in params.items() ]

                try:
                    out = check_output(cmd, text=True).strip()
                except CalledProcessError as e:
                    deal_status.append({
                        "dealUuid": params['deal-uuid'],
                        "provider": params['provider'],
                        "error": e,
                        "clientWallet": params['wallet']
                    })
                    g_deals_failed_loading.labels(epoch=epoch, error='boost_exec').inc()
                    logging.warning(f"WARNING: boost status failed:")
                    logging.warning(e)
                    continue

                logging.debug(out)
                res = json.loads(out)

                if 'error' in res:
                    res['dealUuid'] = params['deal-uuid']
                    res['provider'] = params['provider']
                    res['clientWallet'] = params['wallet']
                    deal_status.append(res)

                    g_provider_failures.labels(provider=params['provider']).inc()
                    g_deals_failed_loading.labels(epoch=epoch, error='boost_error').inc()
                    logging.error("Couldn't load provider details from deal_status: " + ','.join('{}: {}'.format(key, val) for key, val in res.items()))
                else:
                    if res['provider'] not in deal_providers:
                        deal_providers.append(res['provider'])

                    g_deals.labels(epoch=epoch).inc()
                    g_deal_status.labels(epoch=epoch, status=res['status']).inc()
                    g_deal_sealing_status.labels(epoch=epoch, status=res['sealingStatus']).inc()
                    if res['sealingStatus'] == 'deal not found':
                        g_provider_sealing_failure.labels(provider=params['provider']).inc()

                    if 'commp_piece_cid' not in deal:
                        logging.error("commp_piece_cid not found in deal status: " + res['dealUuid'])
                        g_invalid_deals.labels(epoch=epoch).inc()
                    else:
                        if res['status'] == 'IndexedAndAnnounced':
                            # We consider indexed and announced deals to be succesffull
                            if deal['commp_piece_cid'] in epoch_pieces: # should already be in peices because we loaded from metadata.csv
                                epoch_pieces[deal['commp_piece_cid']] += 1
                            else:
                                epoch_pieces[deal['commp_piece_cid']] = 1
                                logging.warning("commp_piece_cid not found in epoch pieces: " + deal['commp_piece_cid'])
                                g_unknown_pieces.labels(epoch=epoch).inc()

                    logging.debug("loaded deal status: " + ','.join('{}: {}'.format(key, val) for key, val in res.items()))

                deal_status.append(res)

            for piece, count in epoch_pieces.items():
                logging.debug("piece: %s, count: %d" % (piece, count))
                g_pieces.labels(epoch=epoch, count=count).inc()
                if count >= REPLICATION_FACTOR:
                    fully_replicated_pieces += 1

            g_providers.labels(epoch=epoch).set(len(deal_providers))

            status_fields = ["chainDealId", "clientWallet", "dealUuid", "label", "provider", "publishCid", "sealingStatus", "status", "statusMessage", "error"]

            write_to_textfile(f'/var/lib/node_exporter/deals_{epoch}.prom', epoch_registry)

            deal_summary.append({
                "epoch": epoch,
                "pieces": regular_pieces,
                "index_pieces": index_pieces,
                "fully_replicated_pieces": fully_replicated_pieces,
                "epoch_root_cid": epoch_root_cid,
                "index_root_cid": index_root_cid
            })

            deal_status_filename = f'{deals_folder}/deal_status_{epoch}.csv'
            with open(deal_status_filename, 'w') as status_file:
                csv_writer = csv.DictWriter(status_file, fieldnames = status_fields)
                csv_writer.writerows(deal_status)

        if epoch_arg == "all":
            write_to_textfile(f'/var/lib/node_exporter/deals_status.prom', registry)

        summary_fields = ["epoch", "pieces", "index_pieces", "fully_replicated_pieces", "epoch_root_cid", "index_root_cid"]
        filetime = datetime.datetime.now().strftime("%Y%m%d%H%M%S")

        deal_summary_filename = f'{deals_folder}/deal_summary_{epoch_arg}_{filetime}.csv'
        with open(deal_summary_filename, 'w') as summary_file:
            csv_writer = csv.DictWriter(summary_file, fieldnames = summary_fields)
            csv_writer.writeheader()
            csv_writer.writerows(deal_summary)

    except Exception as e:
        logging.exception(e)
        sys.exit(1)




