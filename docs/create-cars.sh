#!/bin/bash

set -e
shopt -s nullglob

SNAPSHOT_FOLDER="{{ warehouse_processor_snapshot_folder }}"

# find out the right number
SHRED_REVISION_CHANGE_EPOCH="32"

CARDIR="{{ warehouse_processor_car_folder }}"

# add defaults?
RANGE_START=$1

RANGE_END=$2

get_archive_file() {
    find "$1" -type f \( -name "rocksdb.tar.zst" -o -name "rocksdb.tar.bz2" \)
}

cleanup_artifacts() {

    cardirepoch="$1"
    workdir="$2"
    set -x
    echo "cleaning up generated files"
    rm -rf $cardirepoch/*index
    rm -rf $cardirepoch/*gsfa.index.tar.bz2
    rm -rf $cardirepoch/*.car
    rm -rf $workdir/*
    set +x
}

# remove rocksdb data
cleanup_workfiles() {
    workdir="$1"
    prev_workdir="$2"

    if [[ "${prev_workdir}" != "" ]]; then
        rm -rf $prev_workdir/rocksdb*
    fi
    # only remove the archive, we might need prev snapshot on the next snapshot cargen
    rm -rf $workdir/rocksdb.tar*
}

metric() {
    EPOCH="$1"
    STATUS="$2"
    echo "cargen_status{epoch=\"$EPOCH\",status=\"$STATUS\"} 1" | sponge /var/lib/node_exporter/cargen_$EPOCH.prom
}


slack() {
    msg="$1"
    set -x
    curl -XPOST "https://hooks.slack.com/services/{{ warehouse_slack_token }}" -H "Content-type: application/json" -d "{\"text\": \"$msg\"}"
    set +x
}

find_ledgers_file_snapshot() {
    epoch="$1"
    ledgers_file="$SNAPSHOT_FOLDER/epoch-$epoch.ledgers"
    set -x
    if [ ! -f "$ledgers_file" ]; then
        # echo "Ledgers file not found for epoch $epoch: $ledgers_file"
        echo ""
        return 0
    fi

    cat "$ledgers_file"
    set +x
}


download_ledger() {
    epoch="$1"
    echo "pulling snapshot for $epoch..."
    metric $EPOCH "downloading"
    python3 download_ledgers_gcp.py $epoch eu
}

for EPOCH in $(seq $RANGE_START $RANGE_END); do

    download_ledger $EPOCH
    WORKDIR=`find_ledgers_file_snapshot $EPOCH | tail -n1`
    SNAPSHOT=$(basename "$WORKDIR")

    SHRED_REVISION_FLAG=""


    # if test ! -f "$WORKDIR/epoch"; then
    #     echo "no epoch file"
    #     # find way to calculate epoch instead of skipping
    #     # or manually create bounds file (only first 8 epochs)
    #     # or manually run the cargen
    #     continue
    # fi
    #
    # EPOCH=`cat $WORKDIR/epoch | tr -d '\n'`

    # epochs under SHRED_REVISION_CHANGE_EPOCH need different handling
    if [[ "${EPOCH}" -lt "${SHRED_REVISION_CHANGE_EPOCH}" ]]; then
        echo "setting shred-revision=1 for epoch under ${SHRED_REVISION_CHANGE_EPOCH}"
        SHRED_REVISION_FLAG="--shred-revision=1"
    fi

    # SHRED_REVISION_CHANGE_EPOCH epoch needs special handling at a specific slot
    if [[ "${EPOCH}" -eq "${SHRED_REVISION_CHANGE_EPOCH}" ]]; then
        echo "need special handling for epoch ${SHRED_REVISION_CHANGE_EPOCH}"
        SHRED_REVISION_FLAG="--shred-revision=1 --next-shred-revision-activation-slot=10367766"
    fi

    echo "Working on snapshot: $SNAPSHOT for epoch: $EPOCH"

    # remove incomplete work
    mkdir "$CARDIR/" || true
    rm "$CARDIR/epoch-$EPOCH.car" || true

    DBS=""

    # The <epoch>.ledgers file is created by download_ledgers.py
    # this will catch when previous snapshots are required from reading bounds.txt
    echo "reading ledgers file for $EPOCH..."
    while read -r line; do
        DBS+=" --db=${line}/rocksdb";
    done <<< "$(find_ledgers_file_snapshot $EPOCH)"

    # use check mode to check if we need to add next snapshot
    set -x
    if ! /usr/local/bin/radiance car create2 "$EPOCH" $DBS --out="$CARDIR/epoch-$EPOCH.car" --check; then
        # set +x

        echo "pulling next snapshot..."
        NEXT_SNAP=$((EPOCH + 1))
        download_ledger "$NEXT_SNAP"

        echo "reading ledgers file for $NEXT_SNAP..."
        while read -r line; do
            DBS+=" --db=${line}/rocksdb";
        done <<< "$(find_ledgers_file_snapshot $NEXT_SNAP)"

        # DBS="--db=$WORKDIR/rocksdb"
    fi
    # set +x

    echo "Using databases: ${DBS}"

    metric $EPOCH "cargen"

    # create car file
    set -x
    /usr/local/bin/radiance car create2 "$EPOCH" $DBS --out="$CARDIR/epoch-$EPOCH.car" $SHRED_REVISION_FLAG
    # set +x

    # cleanup work files
    echo "cleaning up work files"
    set -x

    # some epochs may need 2 snapshots
    # so we need to keep at least 2 snapshots going back
    PREV_WORKDIR_LEDGERS=$(find_ledgers_file_snapshot $((EPOCH-2)) | tail -n1)
    if [ -n "$PREV_WORKDIR_LEDGERS" ]; then
        cleanup_workfiles $WORKDIR $PREV_WORKDIR_CONTENT
    else
        cleanup_workfiles $WORKDIR
    fi
    # set +x

    # check file exists and non empty
    if ! test -s "$CARDIR/epoch-$EPOCH.car" ; then
        echo "car file is empty/non existant, skipping next steps"
        continue;
    fi

    if ! test -f "$CARDIR/$EPOCH.cid"; then
        echo "no root cid, generation probably failed"
        continue;
    fi

    # Create the cardir epoch dir
    mkdir -p "$CARDIR/$EPOCH" || true
    # move cid/recap/slots files
    mv $CARDIR/$EPOCH.cid $CARDIR/$EPOCH/epoch-$EPOCH.cid # IMP RENAME THE CID
    mv $CARDIR/$EPOCH.* $CARDIR/$EPOCH/
    /usr/local/bin/radiance version >$CARDIR/$EPOCH/radiance.version.txt

    # split into 30gb files
    echo "splitting car file..."
    metric $EPOCH "splitting"
    /usr/local/bin/split-epoch.sh $EPOCH $CARDIR/epoch-$EPOCH.car $CARDIR

    # creates the indexes and the appropriate car files
    # since split-epoch moves the epoch file, we need to also use a different path here for the epoch file
    echo "creating indexes..."
    metric $EPOCH "indexing"
    /usr/local/bin/index-epoch.sh all $EPOCH $CARDIR/$EPOCH/epoch-$EPOCH.car $CARDIR

    # upload
    echo "uploading car file..."
    metric $EPOCH "uploading"
    B2_ACCOUNT_INFO=/etc/b2/filecoin_account_info /usr/local/bin/upload-epoch.sh $EPOCH $CARDIR

    # clean up
    echo "cleaning up artifacts..."
    cleanup_artifacts "$CARDIR/$EPOCH" "$WORKDIR"

    slack "{{ inventory_hostname }} finished generating epoch $EPOCH car files"

    metric $EPOCH "complete"

    # done
    touch "$WORKDIR/.cargen"

    # reached end of range
    if [[ "${EPOCH}" -eq "${RANGE_END}" ]]; then
        echo "reached end of the given range, please restart job with new parameters to resume cargen"
        exit 0
    fi

done
