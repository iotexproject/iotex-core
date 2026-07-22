// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

// abiJSON declares the single event this package encodes. Field ordering
// and indexed flags mirror IIP-59 §3.2 exactly; off-chain verifiers pin
// the selector derived from this signature.
const abiJSON = `[
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
			{"indexed": false, "name": "amounts",         "type": "uint256[]"},
			{"indexed": false, "name": "compoundBucketIds", "type": "uint64[]"}
		],
		"name": "DelegateDistributed",
		"type": "event"
	}
]`

// eventName is the event's ABI name; used for method lookup and error text.
const eventName = "DelegateDistributed"

// eventSignature is the canonical Solidity signature. keccak256 of this
// string is Topics[0]. Kept as a const for the golden-selector test.
const eventSignature = "DelegateDistributed(uint64,address,address,uint256,uint256,bytes32,address[],uint256[],uint64[])"
