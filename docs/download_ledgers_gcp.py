#!/usr/bin/env python3

import sys
import os
import re
import natsort
import bisect
import subprocess
import queue
import threading
import concurrent.futures
import contextvars
import time
import logging
import shutil

from pathlib import Path

def sync_bounds(bucket, bounds_dir):
    """
    Syncs bounds for all epochs on gcp
    """
    exclude_pattern = '.*hourly.*|.*tar.*|.*rocksdb.*|.*sst.*|.*gz|.*csv|.*zst'
    command = f"gsutil -m rsync -r -x '{exclude_pattern}'  gs://{bucket}/ {bounds_dir}"
    logging.debug(command)
    subprocess.run(command, shell=True, check=True)

def load_epoch_ranges(dir):
    """
    Loads the epoch ranges from a directory containing epoch bounds

    Returns a tuple of an array of start slots for the epochs contained in the dir and the end slots
    """
    snapshot_count = 0

    # Starts ccontains the starting slot of each ledger
    starts = []
    # Ranges contains the epoch details of eacch snapshot, indexed by its starting slot
    ranges = {}
    for subdir, dirs, files in natsort.natsorted(os.walk(dir)):
        for file in files:
            if file != "bounds.txt":
                continue

            snapshot_count = snapshot_count + 1
            bounds_file = os.path.join(subdir, file)

            f = open(bounds_file).readlines()[0]
            # earlier Solana releases have a shorter description without number of slots
            matches = re.search(r"Ledger has data for (?:[0-9]+ )?slots ([0-9]+) to ([0-9]+)", f)
            if matches == None:
                matches = re.search(r"Ledger contains data from slots ([0-9]+) to ([0-9]+)", f)
                if matches == None:
                    logging.error("Could not parse bounds file: " + bounds_file)
                    continue

            start = int(matches.group(1))
            end = int(matches.group(2))

            ranges[start] = {
                    "dir": os.path.basename(os.path.normpath(subdir)),
                    "subdir": subdir,
                    "start": start,
                    "end": end
            }

            starts.append(start)
    return (starts, ranges)

def find_snapshots_for_epochs(first_epoch, last_epoch, starts, ranges):
    """
    Finds complete set of snapshots for a given epoch.

    Returns a tuple cconsisting of a list of epochs for which complete snapshots could be found and a list of epochs for which snapshots are missing
    """
    epochs = []
    missing_epochs = []
    for i in range(first_epoch, last_epoch+1):
        epoch_start = i * 432000
        epoch_end = (i+1)*432000-1

        logging.debug(str(epoch_start) + " " + str(epoch_end))

        if i != 0:
            left = bisect.bisect_left(starts, epoch_start)
            if left == 0:
                missing_epochs.append(i)
                logging.debug("Couldn't find the right one ", left)
                continue
            else:
                left = left-1
        else:
            left = 0

        start = starts[left]
        snap = ranges[start]
        end = ranges[start]["end"]

        snapshots = []

        if start > epoch_start:
            logging.debug(f"Couldn't find the correct epoch start epoch={i} end={end} start={epoch_start}")
            missing_epochs.append(i)
            continue
        if end < epoch_start:
            logging.debug(f"Couldn't find the correct snapshot, end is less than start epoch={i} end={end} start={epoch_start}")
            missing_epochs.append(i)
            continue

        snapshots.append(snap)
        while end < epoch_end:
            if left >= len(starts):
                logging.debug("Couldn't find the correct ending snapshot, need more data")
                continue

            start = starts[left]
            snap = ranges[start]
            end = ranges[start]["end"]
            snapshots.append(snap)
            left += 1

        epochs.append({
            "epoch": i,
            "start": epoch_start,
            "end": epoch_end,
            "snapshots": snapshots
        })
    return (epochs, missing_epochs)

def find_snapshots_for_epoch(epoch, start, ranges):
    return find_snapshots_for_epochs(epoch, epoch, start, ranges)

def sync_snapshots_from_b2(epochs, bucket, downloads_dir):
    """
    Syncs snapshots from gcp for a specific set of epochs
    """
    for epoch in epochs:
        logging.info(f"Getting snapshot for {epoch}")
        for s in range(len(epoch['snapshots'])):
            snapshot = epoch['snapshots'][s]
            try:
                command = ['s5cmd', '--no-sign-request', '--endpoint-url', 'https://storage.googleapis.com', 'sync', '--exclude', '*snapshot-*', f"s3://{bucket}/{snapshot['dir']}/*", f"{downloads_dir}/{snapshot['dir']}/"]
                logging.debug(command)
                subprocess.run(command, check=True)
            except Exception as e:
                logging.error("error:", e, snapshot)


# Locks to allow parallel execution of snapshot extraction
# TODO: remove threading stuff altogether
processing_snapshots = {}
snapshot_lock = threading.Lock()


def move_rocksdb_snapshot(dir):
    """ makes sure rocksdb is moved from any subdirectory to the base snapshot dir"""
    for root, _, _ in os.walk(dir):

        rocksdb_dir = os.path.join(root, 'rocksdb')
        if os.path.isdir(rocksdb_dir):
            new_location = os.path.join(dir, 'rocksdb')
            logging.info("Moving rocksdb from %s to %s" % (rocksdb_dir, new_location))
            subprocess.run(['mv', rocksdb_dir, new_location])
    logging.info("RocksDB directories moved successfully.")
    #
    # doesn't seem to work
    # does it cause files like CURRENT to be missing? silent fail?
    #
    # for root, _, files in os.walk(dir):
    #     if 'rocksdb' in files and 'rocksdb' not in root.split(os.path.sep):
    #         rocksdb_path = os.path.join(root, 'rocksdb')
    #         target_path = os.path.join(directory, 'rocksdb')
    #
    #         if not os.path.exists(target_path):
    #             shutil.move(rocksdb_path, target_path)
    #             print(f"Moved 'rocksdb' to {target_path}")
    #             return
    #
    #         return

def rocksdb_is_extracted(snapshot_path):
    """
    determine if snapshot extracted by checking flag file and one of the decompressed files
    """
    return os.path.isfile(snapshot_path + '/.extracted') and os.path.isfile(snapshot_path + '/rocksdb/CURRENT')

def pigz_archived(snapshot_path):
    """
    if pigz was used to compress rocksdb, there'll be a 1000 .gz files
    instead of one tar archive - check CURRENT.gz exists to determine it
    """
    return os.path.isfile(snapshot_path + '/rocksdb/CURRENT.gz')


def extract_snapshot(snapshot, downloads_dir):
    """
    Extracts a snapshot rocksdb file. Will only extract the snapshot ones.
    Designed to be able to run in parallel with multiple calls for the same snapshot dir not creating a new exraction.
    """
    logging.debug("Called extract snapshot with snapshot: " + snapshot['dir'])
    snapshot_lock.acquire()
    if snapshot['dir'] not in processing_snapshots:
        # check if rocksdb has already been extracted
        # if not os.path.isdir('download/'+snapshot['dir']+'/rocksdb'):
        snapshot_path = downloads_dir+'/'+snapshot['dir']

        if rocksdb_is_extracted(snapshot_path):
            processing_snapshots[snapshot['dir']] = 'completed'
            Path(snapshot_path + '/.extracted').touch()
            Path(snapshot_path + '/.rocksdb').touch()
            snapshot_lock.release()
            logging.info("snapshot already extracted: " + snapshot['dir'])
            return 0

        extract_filename = ''
        decompress_tool = False
        if os.path.isfile(snapshot_path + '/rocksdb.tar.bz2'):
            extract_filename = 'rocksdb.tar.bz2'
            decompress_tool = "lbzip2"
        elif os.path.isfile(snapshot_path + '/rocksdb.tar.gz'):
            extract_filename = 'rocksdb.tar.gz'
        elif os.path.isfile(snapshot_path + '/rocksdb.tar.zst'):
            extract_filename = 'rocksdb.tar.zst'
            decompress_tool = "pzstd"
        elif pigz_archived(snapshot_path):
            # there's no single archive file, just bypass check below
            extract_filename = "n/a"

        if extract_filename != '':
            processing_snapshots[snapshot['dir']] = 'processing'
            snapshot_lock.release()

            # disabled - does not seem to be required
            # check CURRENT file exists in the tar file before extraction, it can be in a path
            # command = f"tar -tf {snapshot_path}/{extract_filename} | grep -qE 'CURRENT|MANIFEST'"
            # logging.debug(command)
            # subprocess.run(command, shell=True, check=True)

            command = ['tar', '-xf', snapshot_path+'/'+extract_filename, '-C', snapshot_path]

            if decompress_tool:
                command = ['tar', '-I', decompress_tool, '-xf', snapshot_path+'/'+extract_filename, '-C', snapshot_path]

            if pigz_archived(snapshot_path):
                command = f"pigz -d {snapshot_path}/rocksdb/*.gz"
                logging.debug(command)
                subprocess.run(command, shell=True, check=True)
            else:
                logging.debug(command)
                subprocess.run(command, check=True)

            move_rocksdb_snapshot(snapshot_path)

            Path(snapshot_path + '/.extracted').touch()
            Path(snapshot_path + '/.rocksdb').touch()

            snapshot_lock.acquire()
            processing_snapshots[snapshot['dir']] = 'completed'
            snapshot_lock.release()
        else:
            snapshot_lock.release()
            logging.error('could not find rocksdb folder nor file')
            return 1
    else:
        snapshot_lock.release()

        # wait for snapshot to finish
        while True:
            time.sleep(5)
            logging.debug('waiting for other thread to complete extraction')
            if processing_snapshots[snapshot['dir']] == 'completed':
                break


    logging.info('finished snapshot extraction')
    return 0

def process_epoch(epoch, downloads_dir):
    """
    Processes a single epoch, running extraction on each of the epoch's snapshots
    In the future we could add cargen here too I guess?
    """
    download_paths = []

    logging.debug("Processing snapshots for epoch:" + str(epoch['epoch']))

    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        future_to_snapshot = {executor.submit(extract_snapshot, snapshot, downloads_dir): snapshot for snapshot in epoch['snapshots']}
        for future in concurrent.futures.as_completed(future_to_snapshot):
            snapshot = future_to_snapshot[future]
            try:
                result = future.result()
                if result != 0:
                    logging.error('snapshot execution error: '+ str(result))
                    return result
            except Exception as exc:
                logging.exception('snapshot exception' + str(exc))
                return 1
            else:
                download_paths.append(downloads_dir + '/' + snapshot['dir'])
                logging.info('snapshot extracted for epoch ' + str(epoch['epoch']) + ", snapshot " + snapshot['dir'])

    with open(downloads_dir+'/epoch-'+ str(epoch['epoch']) + '.ledgers', 'w') as f:
        # @TODO fix duplicate entry in the set, not sure why it's there
        unique_paths = []
        for path in download_paths:
            if path not in unique_paths:
                unique_paths.append(path)

        for path in unique_paths:
            f.write(path + '\n')

    logging.info('finished processing processing snapshots for epoch: ' + str(epoch['epoch']) + ' found: ' + str(len(download_paths)) + ' paths. Writing to file: ' + downloads_dir+'/epoch-'+ str(epoch['epoch'])+ '.ledgers')
    return 0

logging.basicConfig(level=logging.DEBUG)

def main():
    downloads_dir = '/storage/gcp-sg-workdir'
    bucket = 'mainnet-beta-ledger-asia-sg1'

    if len(sys.argv) < 2:
        print('missing epoch argument, run with ', sys.argv[0], ' <start_epoch>[-<end_epoch>]')
        return 1

    epoch_arg = sys.argv[1].split("-", 1)
    start_epoch = int(epoch_arg[0])
    region = sys.argv[2]
    if len(epoch_arg) > 1:
        end_epoch = int(epoch_arg[1])
    else:
        end_epoch = start_epoch

    if region == "ap":
        bucket = 'mainnet-beta-ledger-asia-sg1'
        downloads_dir = '/storage/gcp-sg-workdir'
    if region == "eu":
        bucket = 'mainnet-beta-ledger-europe-fr2'
        downloads_dir = '/storage/gcp-fra-workdir'
    if region == "us":
        bucket = 'mainnet-beta-ledger-us-ny5'
        downloads_dir = '/storage/gcp-nyc-workdir'

    sync_bounds(bucket, downloads_dir)
    starts, ranges  = load_epoch_ranges(downloads_dir)

    logging.debug("Loaded " + str(len(starts)) + " epochs")

    epochs, missing_epochs = find_snapshots_for_epochs(start_epoch, end_epoch, starts, ranges)

    logging.debug("Found " + str(len(epochs)) + " epochs" + " missing " + str(len(missing_epochs)))

    sync_snapshots_from_b2(epochs, bucket, downloads_dir)

    failed_epochs = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        future_to_epoch = {executor.submit(process_epoch, epoch, downloads_dir): epoch for epoch in epochs}
        for future in concurrent.futures.as_completed(future_to_epoch):
            epoch = future_to_epoch[future]
            try:
                result = future.result()
                if result != 0:
                    failed_epochs.append(str(epoch['epoch']))
            except Exception as exc:
                logging.exception("exception occurred: "+str(exc))
                failed_epochs.append(str(epoch['epoch']))
            else:
                logging.info("epoch succeeded: " + str(epoch['epoch']))

    if len(missing_epochs) > 0:
        logging.error('missing epochs: ' + ','.join(missing_epochs))

    if len(failed_epochs) > 0:
        logging.error('failed epochs: ' + ','.join(failed_epochs))


    return 0

if __name__ == "__main__":
    ret = main()
    sys.exit(ret)
