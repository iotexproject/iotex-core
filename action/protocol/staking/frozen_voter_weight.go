// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/state"
)

// NoSelfStakeBucketIndex is the sentinel a candidate carries when it has no
// self-stake bucket. Exported so the rewarding protocol, which freezes this
// index into its per-delegate work, uses the same sentinel.
const NoSelfStakeBucketIndex = uint64(candidateNoSelfStakeBucketIndex)

// FrozenSelfStake is an era's view of one candidate's self-stake bucket.
// FreezeHeight doubles as the presence flag: zero means the caller has no
// frozen era, while bucket index 0 remains a valid self-stake bucket.
type FrozenSelfStake struct {
	FreezeHeight uint64
	BucketIdx    uint64
}

// Known reports whether this value came from a real era freeze.
func (f FrozenSelfStake) Known() bool { return f.FreezeHeight > 0 }

// Covers reports whether bucketIdx was the candidate's self-stake bucket at
// the freeze height. It is always false when the era is unknown.
func (f FrozenSelfStake) Covers(bucketIdx uint64) bool {
	return f.Known() && f.BucketIdx == bucketIdx
}

// FrozenVoterWeight recomputes what one voter's buckets are worth to one
// candidate as of an era freeze height.
//
// evalHeight must be the era's freeze height. Contract buckets that are not
// timestamp-based measure their remaining duration against a block height, so
// using the current block would make their weight drift across drain chunks.
// selfStakeBucketIdx must likewise be the index frozen for this era, not the
// live candidate value.
func FrozenVoterWeight(
	sr protocol.StateReader,
	window eracow.Window,
	p *Protocol,
	candidate address.Address,
	voter address.Address,
	selfStakeBucketIdx uint64,
	evalHeight uint64,
) (*big.Int, error) {
	if p == nil {
		return nil, errors.New("staking: nil protocol")
	}
	total := new(big.Int)
	nativeReader := newCandidateStateReader(sr)
	contractReader := contractstaking.NewStateReader(sr)

	indices, err := nativeReader.FrozenNativeBucketIndices(window, voter)
	if err != nil {
		return nil, err
	}
	for _, index := range indices {
		bkt, err := nativeReader.FrozenNativeBucket(window, index)
		switch {
		case err == nil:
		case errors.Is(err, eracow.ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || !address.Equal(bkt.Candidate, candidate) || bkt.isUnstaked() {
			continue
		}
		selfStake := bkt.Index == selfStakeBucketIdx
		total.Add(total, p.calculateVoteWeight(bkt, selfStake))
	}

	refs, err := contractReader.FrozenBucketRefs(window, voter)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		bkt, err := contractReader.FrozenBucket(window, ref.Contract, ref.BucketID)
		switch {
		case err == nil:
		case errors.Is(err, eracow.ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || !address.Equal(bkt.Candidate, candidate) {
			continue
		}
		// Contract buckets never carry the self-stake bonus. Their converted
		// index is always zero, so comparing it with the native self-stake index
		// would incorrectly reward every contract bucket when that index is zero.
		total.Add(total, p.calculateContractBucketVoteWeight(bkt, evalHeight))
	}
	return total, nil
}
