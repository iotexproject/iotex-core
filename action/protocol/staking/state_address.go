// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// This file holds the single address expression for each native staking key
// that the IIP-59 era copy-on-write layer covers.
//
// Why one expression rather than a (namespace, key) pair spelled at each use
// site: a covered key is read twice by two different mechanisms — live, through
// candSM/candSR, and as-of-the-freeze-height, through eracow.Resolve's live
// fallback. If the two derive the address separately they can drift, and the
// drift does not fail loudly: the frozen read misses, FrozenVoterWeight skips
// the bucket (frozen_voter_weight.go), and the voter is silently underpaid. Sharing the
// constructor makes that class of divergence unrepresentable.
//
// The contract-staking counterparts are
// ContractStakingStateReader.BucketStateOpts and .OwnerIndexStateOpts, which
// additionally carry the reader's construction-time global options. The native
// side has no such options today — candSM wraps a bare protocol.StateManager —
// which is exactly why the address must come from one place before it does.

// nativeBucketStateOpts addresses one native vote bucket.
func nativeBucketStateOpts(index uint64) []protocol.StateOption {
	return []protocol.StateOption{
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(bucketKey(index)),
	}
}

// nativeBucketIndexStateOpts addresses one address's bucket index list.
//
// prefix stays a parameter because putBucketIndex/delBucketIndex serve both
// _voterIndex and _candIndex through one code path. Only _voterIndex is covered
// by the era copy-on-write layer (see SnapshotNativeVoterIndex), but both
// share the key shape and must share the constructor, or changing the shape
// would fix one and break the other.
func nativeBucketIndexStateOpts(addr address.Address, prefix byte) []protocol.StateOption {
	return []protocol.StateOption{
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(AddrKeyWithPrefix(addr, prefix)),
	}
}
