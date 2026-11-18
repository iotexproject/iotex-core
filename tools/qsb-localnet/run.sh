#!/usr/bin/env bash
# Start a single-node local chain with IIP-65 (quantum-safe staking buckets)
# enabled, for trying `ioctl stake2 guard` end to end.
#
#   tools/qsb-localnet/run.sh [--reset] IO_ADDRESS [IO_ADDRESS ...]
#
# Each IO_ADDRESS is funded in the genesis block. The node listens on
# localhost:14014 (gRPC) and localhost:15014 (Web3 JSON-RPC). Data, logs,
# genesis and config live in $QSB_LOCALNET_DIR (default: .qsb-localnet in the
# repository root). --reset deletes that chain's data first.
#
# Why a script: the bucket guard is on only when IsToBeEnabled(h) AND
# IsZanzibarGamma(h) (action/protocol/context.go). A YAML genesis that sets
# Zanzibar Gamma must also set every earlier fork no later than it, and the
# default config points at /var/data. The script writes both files for you.
# Local test chain only: never use these files for testnet or mainnet.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
DIR=${QSB_LOCALNET_DIR:-$ROOT/.qsb-localnet}

reset=false
if [[ ${1:-} == "--reset" ]]; then
  reset=true
  shift
fi
if [[ $# -lt 1 ]]; then
  echo "usage: $0 [--reset] IO_ADDRESS [IO_ADDRESS ...]" >&2
  exit 2
fi
for a in "$@"; do
  [[ $a == io1* ]] || { echo "not an io1 address: $a" >&2; exit 2; }
done

if $reset; then
  rm -rf "$DIR/data" "$DIR/log"
fi
mkdir -p "$DIR/bin" "$DIR/data" "$DIR/log"

echo "building server into $DIR/bin ..."
(cd "$ROOT" && go build -o "$DIR/bin/server" ./server)

# Every fork height declared in genesis.Blockchain, including
# toBeEnabledHeight, so that a fork added later is picked up too.
forks=$(grep -oE '^\s+[A-Z][A-Za-z]*BlockHeight\s+uint64\s+`yaml:"[A-Za-z]+"`' \
  "$ROOT/blockchain/genesis/genesis.go" | grep -oE 'yaml:"[^"]+"' | cut -d'"' -f2)

{
  echo "account:"
  echo "  initBalances:"
  for a in "$@"; do
    echo "    $a: \"100000000000000000000000000\"" # 100,000,000 IOTX each
  done
  echo "blockchain:"
  echo "  blockInterval: 5s"
  echo "  numSubEpochs: 15"
  echo "  timeBasedRotation: true"
  for k in $forks; do
    echo "  $k: 1"
  done
  echo "poll:"
  echo "  enableGravityChainVoting: false"
} > "$DIR/genesis.yaml"

D="$DIR/data"
cat > "$DIR/config.yaml" <<EOF
chain:
  chainDBPath: $D/chain.db
  trieDBPath: $D/trie.db
  trieDBPatchFile: ""
  stakingPatchDir: $D
  indexDBPath: $D/index.db
  bloomfilterIndexDBPath: $D/bloomfilter.index.db
  candidateIndexDBPath: $D/candidate.index.db
  stakingIndexDBPath: $D/staking.index.db
  sgdIndexDBPath: $D/sgd.index.db
  contractStakingIndexDBPath: $D/contractstaking.index.db
  blobStoreDBPath: $D/blob.db
  gravityChainDB:
    dbPath: $D/poll.db
actPool:
  store:
    datadir: $D/actpool.cache
consensus:
  rollDPoS:
    consensusDBPath: $D/consensus.db
system:
  systemLogDBPath: $DIR/log/systemlog.db
log:
  zap:
    level: info
    encoding: json
    disableStacktrace: true
    outputPaths: ["$DIR/log/server.log"]
    errorOutputPaths: ["$DIR/log/server.log"]
EOF

echo "starting a local chain in $DIR (Ctrl-C to stop); logs in $DIR/log/server.log"
exec "$DIR/bin/server" -plugin=gateway \
  -config-path="$DIR/config.yaml" -genesis-path="$DIR/genesis.yaml"
