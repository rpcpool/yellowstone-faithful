# Recreate the full CAR archive

If you're ready for months of toiling away, going through 100s of terabytes of data over and over, spending some hunnids per month on hardware and bandwidth and going through all the error and trial required for this task please continue reading below.


## Hardware & other requirements

1. Servers
2. Object storage
3. Software

### Servers

You'll need _big_ machines:
- 20Tb working storage for juggling multiple snapshots + working data should be comfortable
- 10Gbit uplink can shave hours of downloading/uploading stuff for each epoch
- Good CPU power speeds up car generation

We've used this Ryzen 9 from Hetzner with great success

- AX102 with 10Gbit uplink + 4x7.68 NVMe (disks are pooled using LVM)

https://www.hetzner.com/dedicated-rootserver/ax102/configurator/#/

Order multiple servers to parallelize the work and speed up the process.

### Object storage:

We've uploaded all CAR files + indexes to Backblaze B2 due to storage cost + bandwidth alliance + egress fees.

If we had to start again we could've probably self-hosted as upload/download performance and reliability was lacking at times.

### Software:

- Ubuntu 20
- python3
  - gsutil
  - b2 (if you use B2 to store CAR files)
  - `python3 -m pip install --upgrade gsutil b2`
- s5cmd
- zstd
- /usr/local/bin/faithful-cli [faithful-cli](https://github.com/rpcpool/yellowstone-faithful)
- /usr/local/bin/filecoin-data-prep [go-fil-dataprep](https://github.com/rpcpool/go-fil-dataprep)
- Node Exporter


### Preparation

Find and replace the variables below with your own values (or use it as an Ansible template):

- {{ warehouse_processor_snapshot_folder }} - Where to store snapshots, i.e. `/snapshots`
- {{ warehouse_processor_car_folder }} - Where to save CAR files, i.e. `/cars`
- {{ warehouse_slack_token }} - Webhook token to send messages to your Slack Channel
- {{ inventory_hostname }} - Hostname where the script is installed, i.e. `$(hostname)`

## Running

We've used Rundeck as a controller node to initiate and run jobs on the multiple processing nodes.

On each, run the script and specify a range of _epochs_ to go through where the script will attempt to download all relevant snapshots from Solana Foundation GCP buckets in order to gather all relevant slots.


`create-cars.sh 100-200`

`create-cars.sh 300-500`

`create-cars.sh 600-620`

If not relevant you can comment out the code related to sending notifications to Slack and creating metric files in this file before running it.


## Costs

Using B2 object storage service and the Hetzner node recommended above your costs:

- servers: €500/month per processor node
- object storage: over 1300$/month when it's all done: which includes CAR + split file + index storage

## Dashboards

create-cars.sh is creating .prom files in `/var/lib/node_exporter/`, so by having node exporter running and being scraped (Prometheus) you can monitor the car generation progress on Grafana .

Import dashboard.yml in this directory into Grafana, update the variable with your node names (or use a query instead).

## Gotchas

1. In some cases the script won't find the required snapshots, you'll need to find and download all required snapshots and run radiance.
2. First 8 epochs are missing bounds.txt file so you may need to do it manually.
3. Around epoch 32 there was a change in `shred revision`, depending on what snapshot you use you may need to adjust `SHRED_REVISION_CHANGE_EPOCH` (currently set at 32, but could be 24)
3. Create cars tries to use download_ledgers_gcp.py to pull snapshots from EU region only, if it can't find you'll need to run it manually `download_ledgers_gcp.py <epoch> <ap|us>` to find the required snapshots in the other regions and run radiance with the necessary parameters.
