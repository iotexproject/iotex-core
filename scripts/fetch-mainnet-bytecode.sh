#!/usr/bin/env bash
# fetch-mainnet-bytecode.sh — capture DelegateProfile and AutoDeposit
# runtime bytecode from IoTeX mainnet into the e2etest fixture files.
#
# Idempotent. Overwrites the two fixture files with the current on-chain
# bytecode. Diff against git after running to see if the mainnet contract
# has changed between captures. Requires: curl, jq.
#
# Usage:
#   ./scripts/fetch-mainnet-bytecode.sh
#
# Fixtures produced:
#   e2etest/delegateprofile_bytecode
#   e2etest/autodeposit_bytecode
#
# Each file contains the runtime bytecode as one hex line (no 0x prefix,
# no trailing newline). Matches the layout of the existing
# e2etest/staking_contract_v2_bytecode fixture.
#
# Deployed addresses of the two contracts IIP-59 reads:
#
#              DelegateProfile                             AutoDepositRegister
#   mainnet    0xfa7f50866ac45d84adf54bc767c885f92750e258  0x79f1670BE20daecEfB134E33D97f9E77340fd2C0
#   testnet    0xd19ffB48a5C18B77c541D32c1B1ac2440c287774  0x40C578628ef75fe6e83FCD6DCE989f335D884bCE
#
# In bech32, the testnet pair is io16x0lkj99cx9h032p6vkpkxkzgsxzsam5t5sk5s and
# io1grzhsc5w7a07d6ple4kuaxylxdwcsj7w8r2zuu; those are the values
# genesis_testnet.yaml carries as delegateProfileContractAddress and
# autoDepositContractAddress.
#
# The testnet AutoDepositRegister was deployed by installing this script's
# autodeposit fixture verbatim, so its storage layout is byte-identical to
# mainnet's and the direct-slot constants in
# action/protocol/rewarding/autodeposit/slot_reader.go hold on both networks.

set -euo pipefail

RPC="${IOTEX_MAINNET_RPC:-https://babel-api.mainnet.iotex.io}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$REPO_ROOT/e2etest"

fetch_code() {
    local name="$1" addr="$2" out="$3"
    echo "fetching $name at $addr ..." >&2
    local hex
    hex=$(curl -sS -X POST -H "Content-Type: application/json" --max-time 30 \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$addr\",\"latest\"],\"id\":1}" \
        "$RPC" | jq -r '.result')
    if [[ -z "$hex" || "$hex" == "null" || "$hex" == "0x" ]]; then
        echo "ERROR: no bytecode returned for $name at $addr" >&2
        return 1
    fi
    # Strip 0x prefix; write without trailing newline.
    printf '%s' "${hex#0x}" > "$out"
    echo "wrote $(wc -c < "$out") bytes to ${out#$REPO_ROOT/}" >&2
}

fetch_code delegateprofile 0xfa7f50866ac45d84adf54bc767c885f92750e258 "$OUT_DIR/delegateprofile_bytecode"
fetch_code autodeposit     0x79f1670BE20daecEfB134E33D97f9E77340fd2C0 "$OUT_DIR/autodeposit_bytecode"
