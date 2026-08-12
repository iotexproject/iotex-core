// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

// ABIJSON declares the single event this package encodes. Field ordering
// and indexed flags mirror IIP-59 §3.2 exactly; off-chain verifiers pin
// the selector derived from this signature.
//
// Exported so that off-chain consumers in other repositories (the analyser
// indexer, explorers) decode against this exact definition rather than a
// hand-copied duplicate that can drift from it.
const ABIJSON = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true,  "name": "epoch",           "type": "uint64"},
			{"indexed": true,  "name": "delegate",        "type": "address"},
			{"indexed": false, "name": "rewardAddr",      "type": "address"},
			{"indexed": false, "name": "totalCommission", "type": "uint256"},
			{"indexed": false, "name": "totalVoterPool",  "type": "uint256"},
			{"indexed": false, "name": "snapshotHash",    "type": "bytes32"},
			{"indexed": false, "name": "voters",          "type": "address[]"},
			{"indexed": false, "name": "recipients",      "type": "address[]"},
			{"indexed": false, "name": "amounts",         "type": "uint256[]"},
			{"indexed": false, "name": "compoundBucketIds", "type": "uint64[]"},
			{"indexed": false, "name": "compounded",      "type": "bool[]"}
		],
		"name": "DelegateDistributed",
		"type": "event"
	}
]`

// EventName is the event's ABI name; used for method lookup and error text.
const EventName = "DelegateDistributed"

// EventSignature is the canonical Solidity signature. keccak256 of this
// string is Topics[0]. Kept as a const for the golden-selector test, and
// exported so off-chain consumers can pin the same topic without
// re-deriving it from a copied ABI.
//
// The trailing `bool[] compounded` is not redundant with `uint64[]
// compoundBucketIds`. Native bucket index 0 is a real, ordinary bucket, so
// `compoundBucketIds[i] == 0` cannot be read as "voter i was not compounded"
// -- it is indistinguishable from "voter i was compounded into bucket 0".
// The parallel bool is the authoritative discriminator; consumers must read
// compoundBucketIds[i] only when compounded[i] is true.
const EventSignature = "DelegateDistributed(uint64,address,address,uint256,uint256,bytes32,address[],address[],uint256[],uint64[],bool[])"
