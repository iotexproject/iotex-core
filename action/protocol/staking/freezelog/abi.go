// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package freezelog encodes the event emitted when an IIP-59 era freezes a
// delegate's reward configuration.
//
// It is a self-contained ABI rather than an entry in
// action/native_staking_contract_abi.json because that file is hand-kept in sync
// with native_staking_contract_interface.sol with no Makefile target to catch
// drift, and the two have already diverged -- setVoterRewardOptIn and
// VoterRewardOptInSet are in the JSON and absent from the .sol.
package freezelog

// ABIJSON declares the single event this package encodes.
//
// It is exported so off-chain consumers decode against the protocol's exact
// definition instead of maintaining copies that can drift.
const ABIJSON = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true,  "name": "era",                  "type": "uint64"},
			{"indexed": true,  "name": "delegate",             "type": "address"},
			{"indexed": false, "name": "freezeHeight",         "type": "uint64"},
			{"indexed": false, "name": "blockCommissionBps",   "type": "uint64"},
			{"indexed": false, "name": "epochCommissionBps",   "type": "uint64"},
			{"indexed": false, "name": "commissionConfigured", "type": "bool"},
			{"indexed": false, "name": "totalWeight",          "type": "uint256"},
			{"indexed": false, "name": "selfStakeBucketIdx",   "type": "uint64"}
		],
		"name": "DelegateRewardFrozen",
		"type": "event"
	}
]`

// EventName is the event's ABI name.
const EventName = "DelegateRewardFrozen"

// EventSignature is the canonical Solidity signature. Its keccak256 is Topics[0].
//
// commissionConfigured is not redundant with the two basis-point fields. When a
// delegate has published no portions the protocol freezes it at 10000/10000, so
// those values alone cannot tell "this delegate chose to take everything" apart
// from "this delegate never configured anything" -- and the two mean opposite
// things to a voter deciding where to stake. The bool is the discriminator.
const EventSignature = "DelegateRewardFrozen(uint64,address,uint64,uint64,uint64,bool,uint256,uint64)"
