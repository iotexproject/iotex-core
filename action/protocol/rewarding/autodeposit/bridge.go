// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package autodeposit bridges the on-chain AutoDeposit contract into the
// IIP-59 protocol-native voter reward drain path.
//
// The bridge is a thin holder of the network-pinned contract address plus
// a factory (NewSlotBucketReader) that produces the per-drain BucketReader.
// Read semantics live in slot_reader.go — the production path bypasses the
// EVM's bucket(address) view and reads (registrants[voter], buckets[voter])
// directly from the contract's storage trie for the ~26× per-call speedup
// documented in docs/iip-59-perf-report.md.
//
// Unlike the DelegateProfile bridge (PR 4.5), AutoDeposit is read live at
// drain time, not frozen at PutPollResult — see IIP-59 §3.6.
package autodeposit

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

// Route classifies where a voter's per-epoch share is delivered. The wire
// value matches IIP-59 §3.5's DelegateDistributed.routings[] encoding, so
// PR 4.7's log emitter and this bridge share a single source of truth.
type Route uint8

const (
	// RouteCredit credits the share to the voter's unclaimed balance for
	// pull-claim via the existing ClaimFromRewardingFund action.
	RouteCredit Route = 0
	// RouteCompound calls into the native staking AddDeposit path against
	// the voter's registered bucket.
	RouteCompound Route = 1
)

// Decision is the per-voter routing verdict produced from LookupBucket plus
// the caller-side eligibility check.
type Decision struct {
	Route    Route
	BucketID uint64 // valid iff Route == RouteCompound
}

// ErrEmptyContractAddress is returned when the bridge is constructed
// without a target contract.
var ErrEmptyContractAddress = errors.New("autodeposit: empty contract address")

// Bridge holds the network-pinned AutoDeposit contract address and is the
// factory for per-drain BucketReaders. Construct once at protocol init;
// call NewSlotBucketReader once per epoch drain to get a reusable reader.
type Bridge struct {
	contract string
}

// New constructs a Bridge targeting contract. contract must be a valid
// IoTeX bech32 address; the caller is responsible for pinning it to the
// network-appropriate mainnet/testnet value.
func New(contract string) (*Bridge, error) {
	if contract == "" {
		return nil, ErrEmptyContractAddress
	}
	if _, err := address.FromString(contract); err != nil {
		return nil, errors.Wrap(err, "autodeposit: invalid contract address")
	}
	return &Bridge{contract: contract}, nil
}

// Contract returns the target contract address, mostly for logging.
func (b *Bridge) Contract() string { return b.contract }
