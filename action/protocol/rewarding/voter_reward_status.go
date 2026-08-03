// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
)

func (p *Protocol) voterRewardStatus(
	ctx context.Context,
	sr protocol.StateReader,
	candidateID address.Address,
	voter address.Address,
) (*rewardingpb.VoterRewardStatus, error) {
	status := &rewardingpb.VoterRewardStatus{
		Status:       rewardingpb.VoterRewardStatus_NO_ACTIVE_SETTLEMENT,
		RewardAmount: new(big.Int).Bytes(),
	}
	cursor, err := p.readEpochDrainCursor(ctx, sr)
	if err != nil {
		return nil, err
	}
	if cursor == nil {
		return status, nil
	}
	status.TargetEra = cursor.TargetEra
	g := genesis.MustExtractGenesisContext(ctx)
	status.EraStartEpoch, status.EraEndEpoch = cursor.epochRange(g.EpochsPerRewardEra)
	status.SettlementCompleted = cursor.Completed
	status.CompletedHeight = cursor.CompletedHeight

	delegateIndex := -1
	for i := range cursor.Delegates {
		if bytes.Equal(cursor.Delegates[i].CandidateIdentifier, candidateID.Bytes()) {
			delegateIndex = i
			break
		}
	}
	if delegateIndex < 0 {
		status.Status = rewardingpb.VoterRewardStatus_CANDIDATE_NOT_INCLUDED
		return status, nil
	}

	work := &cursor.Delegates[delegateIndex]
	status.VoterStartIndex = work.VoterStartIndex
	snapshot, err := staking.PollSnapshotFor(sr, candidateID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			status.Status = rewardingpb.VoterRewardStatus_SNAPSHOT_UNAVAILABLE
			return status, nil
		}
		return nil, err
	}
	snapshotHash := snapshotHashFull(snapshot)
	if (len(work.SnapshotHash) > 0 && !bytes.Equal(work.SnapshotHash, snapshotHash[:])) ||
		safeBig(work.TotalWeight).Cmp(safeBig(snapshot.TotalWeight)) != 0 ||
		work.HasWeightedEntries != snapshot.HasWeightedEntries {
		status.Status = rewardingpb.VoterRewardStatus_SNAPSHOT_UNAVAILABLE
		return status, nil
	}

	voterBytes := voter.Bytes()
	physicalIndex := sort.Search(len(snapshot.Entries), func(i int) bool {
		entry := snapshot.Entries[i].Voter
		return entry == nil || bytes.Compare(entry.Bytes(), voterBytes) >= 0
	})
	if physicalIndex == len(snapshot.Entries) || snapshot.Entries[physicalIndex].Voter == nil ||
		!bytes.Equal(snapshot.Entries[physicalIndex].Voter.Bytes(), voterBytes) {
		status.Status = rewardingpb.VoterRewardStatus_VOTER_NOT_INCLUDED
		return status, nil
	}

	alloc := voterAllocatorForWork(snapshot, work)
	logicalIndex := alloc.logicalIndex(uint32(physicalIndex))
	amount, err := alloc.shareOf(logicalIndex)
	if err != nil {
		return nil, err
	}
	status.LogicalVoterIndex = logicalIndex
	status.RewardAmount = amount.Bytes()

	switch {
	case cursor.Completed:
		status.Status = rewardingpb.VoterRewardStatus_PROCESSED
	case uint32(delegateIndex) < cursor.DelegateIndex:
		status.Status = rewardingpb.VoterRewardStatus_PROCESSED
	case uint32(delegateIndex) > cursor.DelegateIndex:
		status.Status = rewardingpb.VoterRewardStatus_WAITING
	case logicalIndex < cursor.VoterIndex:
		status.Status = rewardingpb.VoterRewardStatus_PROCESSED
	default:
		status.Status = rewardingpb.VoterRewardStatus_WAITING
	}
	return status, nil
}
