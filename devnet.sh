#!/bin/bash

SCRIPT=$(basename ${BASH_SOURCE[0]})
QTD=2 #QTD=2 == 3 nodes since it counts from zero
TYPE=branch
DUSK_VERSION=v0.3.0
INIT=true
MOCK=true
VOUCHER=true
FILEBEAT=false
MOCK_ADDRESS=127.0.0.1:9191
SEND_BID=false

currentDir=$(pwd)
RACE=false
DEBUG=false

# define command exec location
duskCMD="../bin/dusk"

usage() {
  echo "Usage: $SCRIPT"
  echo "Optional command line arguments"
  echo "-c <number>  -- TYPE to use. eg: release"
  echo "-q <number>  -- QTD of validators to run. eg: 5"
  echo "-s <number>  -- DUSK_VERSION version. eg: v0.3.0"
  echo "-i <boolean>  -- INIT instances (true or false). default: true"
  echo "-m <boolean>  -- MOCK instances (true or false). default: true"
  echo "-f <boolean>  -- FILEBEAT instances (true or false). default: false"
  echo "-b <boolean>  -- SEND_BID tx to instances (true or false). default: false"
  echo "-r <number>  -- RACE enabled (true or false). eg: true"
  echo "-d <number>  -- DEBUG enabled (true or false). eg: true"

  exit 1
}


while getopts "h?c:q:s:i:m:f:b:r:d:" args; do
case $args in
    h|\?)
      usage;
      exit;;
    c ) TYPE=${OPTARG};;
    q ) QTD=${OPTARG};;
    s ) DUSK_VERSION=${OPTARG};;
    i ) INIT=${OPTARG};;
    m ) MOCK=${OPTARG};;
    f ) FILEBEAT=${OPTARG};;
    b ) SEND_BID=${OPTARG};;
    r ) RACE=${OPTARG};;
    d ) DEBUG=${OPTARG};;
  esac
done

set -euxo pipefail
if [[ "${TYPE}" == "branch" && "${RACE}" == "true" && "${DEBUG}" == "true" ]]; then
  GOBIN=$(pwd)/bin go run scripts/build.go install -race -debug
elif [[ "${TYPE}" == "branch" && "${RACE}" == "true" ]]; then
  GOBIN=$(pwd)/bin go run scripts/build.go install -race
elif [[ "${TYPE}" == "branch" && "${DEBUG}" == "true" ]]; then
  GOBIN=$(pwd)/bin go run scripts/build.go install -debug
elif [[ "${TYPE}" == "branch" ]]; then
  GOBIN=$(pwd)/bin go run scripts/build.go install
fi

#init dusk
init_dusk_func() {
  echo "init Dusk node $i ..."
  DDIR="${currentDir}/devnet/dusk_data/dusk${i}"
  LOGS="${currentDir}/devnet/dusk_data/logs"

  # cleanup
  rm -rf "${DDIR}"
  rm -rf "${LOGS}"
  rm -rf "${DDIR}/walletDB/"

  mkdir -p "${DDIR}"
  mkdir -p "${LOGS}"
  mkdir -p "${DDIR}/walletDB/"

  cat <<EOF > "${DDIR}"/dusk.toml
[consensus]
  defaultamount = 50
  defaultlocktime = 1000
  defaultoffset = 10
  consensustimeout = 5

[database]
  dir = "${DDIR}/chain/"
  driver = "heavy_v0.1.0"

[general]
  network = "testnet"
  walletonly = "false"
  safecallbacklistener = "true"

# Timeout cfg for rpcBus calls
[timeout]
  timeoutsendbidtx = 5
  timeoutgetlastcommittee = 5
  timeoutgetlastcertificate = 5
  timeoutgetmempooltxsbysize = 4
  timeoutgetlastblock = 5
  timeoutgetcandidate = 5
  timeoutclearwalletdatabase = 0
  timeoutverifycandidateblock = 5
  timeoutsendstaketx = 5
  timeoutgetmempooltxs = 3
  timeoutgetroundresults = 5
  timeoutbrokergetcandidate = 2
  timeoutreadwrite = 60
  timeoutkeepalivetime = 30
[genesis]
  legacy = true

[gql]
  address = "127.0.0.1:$((9500+$i))"
  enabled = "true"
  network = "tcp"

[logger]
  output = "stdout"
  #output = "${DDIR}/dusk"
  level = "trace"
  format = "json"
[logger.monitor]
# enabling log based monitoring
enabled = false

[mempool]
  maxinvitems = "10000"
  maxsizemb = "100"
  pooltype = "hashmap"
  prealloctxs = "100"

[network]
  port = "$((7100+$i))"

  [network.seeder]
    addresses = ["127.0.0.1:8081"]

[rpc]
  address = "${DDIR}/dusk-grpc.sock"
  enabled = "true"
  network = "unix"

  [rpc.rusk]
    address = "127.0.0.1:$((10000+$i))"
    contracttimeout = 6000
    defaulttimeout = 1000
    network = "tcp"

[wallet]
  file = "${currentDir}/harness/data/wallet-$((9000+$i)).dat"
  store = "${DDIR}/walletDB/"
[api]
  enabled=false
  enableTLS = false
  address="127.0.0.1:$((9490+$i))"
  dbfile="${DDIR}/chain/api.db"
  #5 mins
  expirationtime=300


EOF

  echo "init Dusk node $i, done."
}

#init dusk
init_filebeat_func() {
  echo "init FILEBEAT node $i ..."
  DDIR="${currentDir}/devnet/dusk_data/dusk${i}"
  rm -rf "${DDIR}"/filebeat

  cat <<EOF > "${DDIR}"/filebeat.json.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - ${currentDir}/devnet/dusk_data/logs/dusk$((7100+$i)).log
  exclude_files: ['\.gz$']
  encoding: plain
  json.keys_under_root: true
name: dusk$((7100+$i))
output.logstash:
  hosts: ["0.0.0.0:5000"]
  #index: "devnet-v1"
  #tags: ["devnet","dusk${i}"]
  #ignore_older: "5h"
  #close_inactive: "4h"
  #close_renamed: true
  #clean_inactive: true
  #clean_removed: true
  #tail_files: true
  encoding: plain
#setup.dashboards.enabled: true
#setup.kibana.host: "localhost:5601"
#setup.kibana.protocol: "http"
#setup.kibana.username: elastic
#setup.kibana.password: changeme
#setup.dashboards.retry.enabled: true
#setup.dashboards.retry_interval: 10
fields:
  env: dusk$((7100+$i))
#  type: node
processors:
#  - add_host_metadata: ~
#  - add_cloud_metadata: ~
#  - decode_json_fields:
#      fields: ["field1", "field2", ...]
#      process_array: false
#      max_depth: 1
#      target: ""
#      overwrite_keys: false
#      add_error_key: true
  - convert:
      fields:
        - {from: "msg", type: "string"}
        - {from: "level", to: "msg_level", type: "string"}
        - {from: "category", to: "msg_category", type: "string"}
        - {from: "process", to: "msg_process", type: "string"}
        - {from: "topic", to: "msg_topic", type: "string"}
        - {from: "recipients", to: "msg_recipients", type: "string"}
        - {from: "step", to: "msg_step", type: "string"}
        - {from: "round", to: "msg_round", type: "string"}
        - {from: "id", to: "msg_id", type: "string"}
        - {from: "sender", to :"msg_sender", type: "string"}
        - {from: "agreement", to: "msg_agreement", type: "string"}
        - {from: "quorum", to: "msg_quorum", type: "string"}
        - {from: "new_best", to: "msg_new_best", type: "string"}
        - {from: "block_hash", to: "msg_block_hash", type: "string"}
        - {from: "prefix", to: "msg_prefix", type: "string"}
        - {from: "hash", to: "msg_hash", type: "string"}
        - {from: "error", to: "msg_error", type: "string"}
        - {from: "score", to: "msg_score", type: "string"}
        - {from: "count", to: "msg_count", type: "string"}
        - {from: "last_height", to: "msg_last_height", type: "string"}
        - {from: "height", to: "msg_height", type: "string"}
        - {from: "sender", to: "msg_sender", type: "string"}
      ignore_missing: true
      fail_on_error: false
      mode: rename
  - drop_fields:
      fields: ["agent", "ecs.version", "input", "log", "@metadata", "host"]

EOF

  chmod go-w "${DDIR}"/filebeat.json.yml

  echo "init FILEBEAT node $i, done."
}

# start dusk mocks
start_dusk_mock_rusk_func() {
  echo "starting Dusk Rusk mock $i ..."

  DDIR="${currentDir}/devnet/dusk_data/dusk${i}"
  WALLET_FILE="${currentDir}/harness/data/wallet-$((9000+$i)).dat"

  CMD="${currentDir}/bin/utils mockrusk --rusknetwork tcp --ruskaddress 127.0.0.1:$((10000+$i)) --walletstore ${DDIR}/walletDB/ --walletfile ${WALLET_FILE}"
  ${CMD} >> "${currentDir}/devnet/dusk_data/logs/mock$i.log" 2>&1 &

  EXEC_PID=$!
  echo "started Dusk Rusk Mock node $i, pid=$EXEC_PID"
}

# start dusk
start_dusk_func() {
  echo "starting Dusk node $i ..."

  DDIR="${currentDir}/devnet/dusk_data/dusk${i}"

  CMD="${currentDir}/bin/dusk --config ${DDIR}/dusk.toml"
  ${CMD} >> "${currentDir}/devnet/dusk_data/logs/dusk$i.log" 2> "${currentDir}/devnet/dusk_data/logs/dusk$i.err" &

  EXEC_PID=$!
  echo "started Dusk node $i, pid=$EXEC_PID"
  sleep 1

  # load wallet cmd
  LOADWALLET_CMD="./bin/utils walletutils --grpchost unix://${DDIR}/dusk-grpc.sock --walletcmd loadwallet --walletpassword password"
  ${LOADWALLET_CMD} >> "${currentDir}/devnet/dusk_data/logs/load_wallet$i.log" 2>&1 &

}

start_dusk_mock_func() {
  echo "starting Dusk Utils mock ..."

  CMD="${currentDir}/bin/utils mock --grpcmockhost $MOCK_ADDRESS"
  ${CMD} >> "${currentDir}/devnet/dusk_data/logs/mock.log" 2>&1 &

  EXEC_PID=$!
  echo "started Dusk Utils mock, pid=$EXEC_PID"
}

# send_bid_func
send_bid_func(){
  echo "Sending bid to node $i ..."

  # send bid cmd
  SENDBID_CMD="./bin/utils transactions --grpchost unix://${DDIR}/dusk-grpc.sock --txtype consensus --amount 10 --locktime 10"
  ${SENDBID_CMD} >> "${currentDir}/devnet/dusk_data/logs/sendbid_bid$i.log" 2>&1 &
}

start_voucher_func() {
  echo "starting Dusk Voucher ..."

  CMD="${currentDir}/bin/voucher"
  ${CMD} >> "${currentDir}/devnet/dusk_data/logs/voucher.log" 2>&1 &

  EXEC_PID=$!
  echo "started Dusk Voucher, pid=$EXEC_PID"
}

start_filebeat_func() {
  echo "starting FILEBEAT node $i ..."

  DDIR="${currentDir}/devnet/dusk_data/dusk${i}"
  filebeat -e -path.config "${DDIR}" -path.data "${DDIR}/filebeat" -c filebeat.json.yml -d "*" >> "${currentDir}/devnet/dusk_data/logs/filebeat${i}.log" 2>&1 &

  EXEC_PID=$!
  echo "started FILEBEAT, pid=$EXEC_PID"
}

if [ "${INIT}" == "true" ]; then

  for i in $(seq 0 "$QTD"); do
    init_dusk_func "$i"
  done

fi

sleep 1

if [ "${VOUCHER}" == "true" ]; then
  start_voucher_func
fi

sleep 1

if [ "${MOCK}" == "true" ]; then
  start_dusk_mock_func

  for i in $(seq 0 "$QTD"); do
    start_dusk_mock_rusk_func "$i"
  done
fi

if [ "${FILEBEAT}" == "true" ]; then

  for i in $(seq 0 "$QTD"); do
    init_filebeat_func "$i"
    start_filebeat_func "$i"
  done

fi

sleep 1

for i in $(seq 0 "$QTD"); do
  start_dusk_func "$i"
done

sleep 5

if [ "${SEND_BID}" == "true" ]; then
  for i in $(seq 0 "$QTD"); do
    send_bid_func "$i"
  done
fi

echo "done starting Dusk stack"
exit 0