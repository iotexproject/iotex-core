// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

// ABIJSON declares the single event this package encodes. Field ordering
// and indexed flags mirror IIP-59 §3.2 exactly; off-chain verifiers pin
// the selector derived from this signature.
//
// It is exported so off-chain consumers decode against the protocol's exact
// definition instead of maintaining copies that can drift.
const ABIJSON = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true,  "name": "epoch",           "type": "uint64"},
			{"indexed": true,  "name": "delegate",        "type": "address"},
			{"indexed": false, "name": "voterAmount",     "type": "uint256"},
			{"indexed": false, "name": "voters",          "type": "address[]"},
			{"indexed": false, "name": "recipients",      "type": "address[]"},
			{"indexed": false, "name": "amounts",         "type": "uint256[]"},
			{"indexed": false, "name": "compoundBucketIds", "type": "uint64[]"},
			{"indexed": false, "name": "compounded",      "type": "bool[]"}
		],
		"name": "DelegateVoterRewardsDistributed",
		"type": "event"
	}
]`

// EventName is the event's ABI name.
const EventName = "DelegateVoterRewardsDistributed"

// EventSignature is the canonical Solidity signature. Its keccak256 is
// Topics[0].
//
// The trailing `bool[] compounded` is not redundant with `uint64[]
// compoundBucketIds`. Native bucket index 0 is a real, ordinary bucket, so
// `compoundBucketIds[i] == 0` cannot be read as "voter i was not compounded"
// -- it is indistinguishable from "voter i was compounded into bucket 0".
// The parallel bool is the authoritative discriminator; consumers must read
// compoundBucketIds[i] only when compounded[i] is true.
const EventSignature = "DelegateVoterRewardsDistributed(uint64,address,uint256,address[],address[],uint256[],uint64[],bool[])"
