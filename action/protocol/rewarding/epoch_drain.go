// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// epochDrainChunkSize is the per-block delegate quota. Zero disables
// chunking (single-block drain, pre-IIP-59 behavior). The feature gate
// suppresses chunking pre-fork: even a non-zero genesis config value is
// ignored until NoVoterRewardDistribution flips false.
func (p *Protocol) epochDrainChunkSize(ctx context.Context) uint64 {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return 0
	}
	return p.cfg.EpochDrainChunkSize
}

// freezeEpochDrainWork runs Phase A of the multi-block epoch drain: it
// asserts the sentinel + last-block guards, applies the unproductive-
// delegate slash (which does not chunk — the slash TransactionLog lands
// on the same block as the drain start), computes the epoch-reward split,
// captures per-delegate work + foundation bonus + orphan targets into a
// fresh cursor, and returns the delta to apply against unclaimedBalance
// (negative slash contribution; per-delegate + foundation contributions
// are applied by the chunk/Coda helpers as they land).
//
// Voter-level state drift limitation: the frozen delegate list carries
// (epoch_amount, pool_amount) but chunks still call
// distributeCombinedReward, which reads the LIVE CandidatePollSnapshot.
// When a chunk runs in epoch N+1, PutPollResult may have overwritten the
// N-epoch snapshot before the chunk executes, and the voter split will
// see the N+1 voter list / weights. This bounds correctness to
// "chunkSize >= max delegate count" (single-block drain) until a
// follow-up freezes per-voter allocation into the cursor. Consensus is
// unaffected — every validator observes the same drifted state on the
// same block — but pay-outs are not byte-identical to the single-block
// path once drift kicks in.
func (p *Protocol) freezeEpochDrainWork(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
) (
	[]*action.TransactionLog,
	[]*action.Log,
	*epochDrainCursor,
	*big.Int,
	error,
) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))

	a := admin{}
	if _, err := p.state(ctx, sm, _adminKey, &a); err != nil {
		return nil, nil, nil, nil, err
	}

	e := exempt{}
	if _, err := p.state(ctx, sm, _exemptKey, &e); err != nil {
		return nil, nil, nil, nil, err
	}
	exemptAddrs := make(map[string]interface{}, len(e.addrs))
	for _, addr := range e.addrs {
		exemptAddrs[addr.String()] = nil
	}

	uqdMap := make(map[string]uint64)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	if featureWithHeightCtx.GetUnproductiveDelegates(epochStartHeight) || !featureCtx.NotSlashUnproductiveDelegates {
		var err error
		uqdMap, err = poll.MustGetProtocol(protocol.MustGetRegistry(ctx)).CalculateUnproductiveDelegates(ctx, sm)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	candidates, err := poll.MustGetProtocol(protocol.MustGetRegistry(ctx)).Candidates(ctx, sm)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	preludeTxLogs := make([]*action.TransactionLog, 0)
	preludeLogs := make([]*action.Log, 0)
	delta := new(big.Int)
	if !featureCtx.NotSlashUnproductiveDelegates {
		slashAmount, slashLogs, err := p.slashUqd(ctx, sm, blkCtx.BlockHeight, actionCtx.ActionHash, candidates, a.blockReward, uqdMap)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if slashAmount.Sign() > 0 {
			preludeTxLogs = append(preludeTxLogs, &action.TransactionLog{
				Type:      iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND,
				Amount:    slashAmount,
				Sender:    address.StakingBucketPoolAddr,
				Recipient: address.RewardingPoolAddr,
			})
		}
		preludeLogs = append(preludeLogs, slashLogs...)
		delta.Sub(delta, slashAmount)
	}

	epochRewardSplitUqdMap := make(map[string]uint64)
	if featureWithHeightCtx.GetUnproductiveDelegates(epochStartHeight) {
		epochRewardSplitUqdMap = uqdMap
	}
	rewardedCandidates, addrs, amounts, err := p.splitEpochReward(candidates, a.epochReward, a.numDelegatesForEpochReward, exemptAddrs, epochRewardSplitUqdMap)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Build the frozen delegate work list. Every entry from splitEpochReward
	// lands in the cursor including nil-candidate slots — they are skipped
	// at chunk time but occupy an index so DelegateIndex arithmetic matches
	// the pre-chunking loop shape exactly. Pool amounts are read here so the
	// per-delegate balance is fixed at Phase A time, not re-read from state
	// that has already advanced to epoch N+1's block-reward credits.
	visitedPoolIDs := make(map[string]bool)
	frozenDelegates := make([]epochDrainDelegateWork, 0, len(rewardedCandidates))
	for i, cand := range rewardedCandidates {
		if cand == nil {
			frozenDelegates = append(frozenDelegates, epochDrainDelegateWork{
				EpochAmount:      new(big.Int),
				PoolAmountFrozen: new(big.Int),
			})
			continue
		}
		candBytes, err := candidateIdentifierBytes(cand.Address)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		poolAmt, err := p.readPendingBlockRewardPool(ctx, sm, candBytes)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		epochAmt := amounts[i]
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		entry := epochDrainDelegateWork{
			CandidateAddress: cand.Address,
			EpochAmount:      new(big.Int).Set(epochAmt),
			PoolAmountFrozen: new(big.Int).Set(poolAmt),
		}
		if addrs[i] != nil {
			entry.HasRewardAddress = true
			entry.RewardAddress = addrs[i].Bytes()
			// Only mark the pool ID visited when the delegate has a reward
			// address — otherwise the entry belongs to the orphan sweep.
			visitedPoolIDs[string(candBytes)] = true
		}
		frozenDelegates = append(frozenDelegates, entry)
	}

	// Freeze the foundation bonus recipient list. Empty when this epoch is
	// outside every configured bonus window.
	frozenBonus := make([]epochDrainFoundationBonusWork, 0)
	if a.grantFoundationBonus(epochNum) || (epochNum >= p.cfg.FoundationBonusP2StartEpoch && epochNum <= p.cfg.FoundationBonusP2EndEpoch) {
		for i, count := 0, uint64(0); i < len(candidates) && count < a.numDelegatesForFoundationBonus; i++ {
			if _, ok := exemptAddrs[candidates[i].Address]; ok {
				continue
			}
			if candidates[i].Votes.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			count++
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			rewardAddr, err := address.FromString(candidates[i].RewardAddress)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			frozenBonus = append(frozenBonus, epochDrainFoundationBonusWork{
				RewardAddressStr: candidates[i].RewardAddress,
				RewardAddress:    rewardAddr.Bytes(),
				Amount:           new(big.Int).Set(a.foundationBonus),
			})
		}
	}

	// Freeze the orphan list: pool IDs recorded in the enumeration index
	// that are NOT in visitedPoolIDs. Target addresses are resolved now
	// against the live staking view so continuation blocks — which see a
	// stale poll list — do not have to reason about who the reward should
	// route to.
	frozenOrphans, err := p.freezeEpochDrainOrphans(ctx, sm, visitedPoolIDs)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cursor := &epochDrainCursor{
		TargetEpoch:     epochNum,
		DelegateIndex:   0,
		Delegates:       frozenDelegates,
		FoundationBonus: frozenBonus,
		Orphans:         frozenOrphans,
	}
	return preludeTxLogs, preludeLogs, cursor, delta, nil
}

// freezeEpochDrainOrphans resolves the target reward address for each
// pool ID left unvisited by the Phase A rewarded-delegate loop. Address
// resolution uses the live staking view (correct only within the Phase A
// block); the resolved bytes are frozen so chunk / Coda code never
// re-reads the view.
func (p *Protocol) freezeEpochDrainOrphans(
	ctx context.Context,
	sm protocol.StateManager,
	visited map[string]bool,
) ([]epochDrainOrphanWork, error) {
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var csr staking.CandidateStateReader
	orphans := make([]epochDrainOrphanWork, 0)
	for _, candID := range ids {
		if visited[string(candID)] {
			continue
		}
		poolAmt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		if err != nil {
			return nil, err
		}
		entry := epochDrainOrphanWork{
			CandidateIdentifier: append([]byte(nil), candID...),
			PoolAmountFrozen:    new(big.Int).Set(poolAmt),
		}
		if poolAmt.Sign() > 0 {
			candAddr, addrErr := address.FromBytes(candID)
			if addrErr == nil {
				if csr == nil {
					csr, err = staking.ConstructBaseView(sm)
					if err != nil {
						return nil, errors.Wrap(err, "rewarding: construct base view for orphan freeze")
					}
				}
				if cand := csr.GetCandidateByOwner(candAddr); cand != nil && cand.Reward != nil {
					entry.TargetAddress = cand.Reward.Bytes()
					entry.TargetAddressStr = cand.Reward.String()
				}
			} else {
				log.L().Warn("rewarding: orphan pool ID does not decode to an address; will refund",
					zap.Binary("candID", candID),
					zap.Error(addrErr))
			}
		}
		orphans = append(orphans, entry)
	}
	return orphans, nil
}

// runEpochDrainChunk processes cursor.Delegates[start:start+chunkSize],
// where chunkSize==0 means "consume everything remaining in a single
// pass". Returns the delta to apply against unclaimedBalance from the
// per-delegate grants processed by this call, the reward logs collected,
// and a done flag set when the delegate list is fully drained (Coda
// should follow).
//
// Each delegate is atomic within one call — this preserves the batched
// DelegateDistributed log guarantee per IIP-59 §3.7 (one log per
// delegate per epoch, never a fractional split across blocks).
func (p *Protocol) runEpochDrainChunk(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
	chunkSize uint64,
) ([]*action.Log, *big.Int, bool, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	delta := new(big.Int)
	logs := make([]*action.Log, 0)
	start := int(cursor.DelegateIndex)
	end := len(cursor.Delegates)
	if chunkSize > 0 {
		if candidate := start + int(chunkSize); candidate < end {
			end = candidate
		}
	}
	for i := start; i < end; i++ {
		d := cursor.Delegates[i]
		if d.CandidateAddress == "" {
			// nil-candidate slot from splitEpochReward — no work.
			continue
		}
		candBytes, err := candidateIdentifierBytes(d.CandidateAddress)
		if err != nil {
			return nil, nil, false, err
		}
		if !d.HasRewardAddress {
			// The orphan sweep (Coda) owns any pool entry parked under a
			// candidate with no reward address.
			continue
		}
		rewardAddr, err := address.FromBytes(d.RewardAddress)
		if err != nil {
			return nil, nil, false, errors.Wrapf(err, "rewarding: invalid frozen reward address for candidate %s", d.CandidateAddress)
		}
		poolAmt := d.PoolAmountFrozen
		if poolAmt == nil {
			poolAmt = new(big.Int)
		}
		epochAmt := d.EpochAmount
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		if poolAmt.Sign() == 0 && epochAmt.Sign() == 0 {
			continue
		}
		// Reconstruct a minimal Candidate for distributeCombinedReward;
		// the callee only reads .Address. Voter-level state (poll snapshot,
		// buckets) is read live inside distributeCombinedReward — see the
		// drift note on freezeEpochDrainWork.
		candForDist := &state.Candidate{Address: d.CandidateAddress}
		iip59Logs, handled, err := p.distributeCombinedReward(
			ctx, sm, candForDist, rewardAddr, poolAmt, epochAmt,
			blkCtx.BlockHeight, actionCtx.ActionHash,
		)
		if err != nil {
			return nil, nil, false, err
		}
		if handled {
			logs = append(logs, iip59Logs...)
			delta.Add(delta, epochAmt)
			if poolAmt.Sign() > 0 {
				if err := p.deletePendingBlockRewardPool(ctx, sm, candBytes); err != nil {
					return nil, nil, false, err
				}
			}
			continue
		}
		// Legacy fallback path: pool balance is refunded via the delegate's
		// live reward address as a BLOCK_REWARD grant, epoch amount as an
		// EPOCH_REWARD grant. Matches pre-chunking semantics.
		if poolAmt.Sign() > 0 {
			if err := p.grantToAccount(ctx, sm, rewardAddr, poolAmt); err != nil {
				return nil, nil, false, err
			}
			if err := p.deletePendingBlockRewardPool(ctx, sm, candBytes); err != nil {
				return nil, nil, false, err
			}
			data, err := p.encodeRewardLog(rewardingpb.RewardLog_BLOCK_REWARD, rewardAddr.String(), poolAmt)
			if err != nil {
				return nil, nil, false, err
			}
			logs = append(logs, &action.Log{
				Address:     p.addr.String(),
				Topics:      nil,
				Data:        data,
				BlockHeight: blkCtx.BlockHeight,
				ActionHash:  actionCtx.ActionHash,
			})
		}
		if epochAmt.Sign() == 0 {
			continue
		}
		if err := p.grantToAccount(ctx, sm, rewardAddr, epochAmt); err != nil {
			return nil, nil, false, err
		}
		data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr.String(), epochAmt)
		if err != nil {
			return nil, nil, false, err
		}
		logs = append(logs, &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		})
		delta.Add(delta, epochAmt)
	}
	cursor.DelegateIndex = uint32(end)
	done := end >= len(cursor.Delegates)
	return logs, delta, done, nil
}

// runEpochDrainCoda drains the frozen orphan list and foundation bonus
// list captured at Phase A, in that order to match the pre-chunking
// sequence. Returns the delta to apply against unclaimedBalance (only
// foundation bonuses contribute; orphan grants are internal fund
// transfers whose amounts were already debited at block time).
func (p *Protocol) runEpochDrainCoda(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
) ([]*action.Log, *big.Int, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	logs := make([]*action.Log, 0)
	delta := new(big.Int)

	for _, o := range cursor.Orphans {
		poolAmt := o.PoolAmountFrozen
		if poolAmt == nil {
			poolAmt = new(big.Int)
		}
		if poolAmt.Sign() == 0 {
			if err := p.deletePendingBlockRewardPool(ctx, sm, o.CandidateIdentifier); err != nil {
				return nil, nil, err
			}
			continue
		}
		if len(o.TargetAddress) > 0 {
			target, err := address.FromBytes(o.TargetAddress)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "rewarding: invalid frozen orphan target for %x", o.CandidateIdentifier)
			}
			if err := p.grantToAccount(ctx, sm, target, poolAmt); err != nil {
				return nil, nil, err
			}
		} else {
			if err := p.refundPendingBlockRewardPool(ctx, sm, poolAmt); err != nil {
				return nil, nil, err
			}
		}
		data, err := p.encodeRewardLog(rewardingpb.RewardLog_BLOCK_REWARD, o.TargetAddressStr, poolAmt)
		if err != nil {
			return nil, nil, err
		}
		logs = append(logs, &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		})
		if err := p.deletePendingBlockRewardPool(ctx, sm, o.CandidateIdentifier); err != nil {
			return nil, nil, err
		}
	}

	for _, fb := range cursor.FoundationBonus {
		amount := fb.Amount
		if amount == nil {
			amount = new(big.Int)
		}
		if amount.Sign() == 0 {
			continue
		}
		rewardAddr, err := address.FromBytes(fb.RewardAddress)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "rewarding: invalid frozen foundation-bonus reward address %q", fb.RewardAddressStr)
		}
		if err := p.grantToAccount(ctx, sm, rewardAddr, amount); err != nil {
			return nil, nil, err
		}
		data, err := p.encodeRewardLog(rewardingpb.RewardLog_FOUNDATION_BONUS, fb.RewardAddressStr, amount)
		if err != nil {
			return nil, nil, err
		}
		logs = append(logs, &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		})
		delta.Add(delta, amount)
	}
	return logs, delta, nil
}
