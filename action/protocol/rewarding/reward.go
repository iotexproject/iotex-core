// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/enc"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type RewardHistory = rewardHistory

// rewardHistory is the dummy struct to record a reward. Only key matters.
type rewardHistory struct{}

// Serialize serializes reward history state into bytes
func (b rewardHistory) Serialize() ([]byte, error) {
	gen := rewardingpb.RewardHistory{}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into reward history state
func (b *rewardHistory) Deserialize(data []byte) error { return nil }

func (b *rewardHistory) Encode(suffix []byte) (systemcontracts.GenericValue, error) {
	height := enc.MachineEndian.Uint64(suffix)
	data, err := proto.Marshal(&rewardingpb.RewardHistory{
		Height: height,
	})
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{
		PrimaryData: data,
	}, nil
}

func (b *rewardHistory) Decode(suffix []byte, v systemcontracts.GenericValue) error {
	rh := &rewardingpb.RewardHistory{}
	if err := proto.Unmarshal(v.PrimaryData, rh); err != nil {
		return err
	}
	height := enc.MachineEndian.Uint64(suffix)
	if rh.Height != height {
		return errors.Wrapf(state.ErrStateNotExist, "expected height %d, got %d", height, rh.Height)
	}
	return nil
}

// rewardAccount stores the unclaimed balance of an account
type rewardAccount struct {
	balance *big.Int
}

// Serialize serializes account state into bytes
func (a rewardAccount) Serialize() ([]byte, error) {
	gen := rewardingpb.Account{
		Balance: a.balance.String(),
	}
	return proto.Marshal(&gen)
}

// Deserialize deserializes bytes into account state
func (a *rewardAccount) Deserialize(data []byte) error {
	gen := rewardingpb.Account{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	balance, ok := new(big.Int).SetString(gen.Balance, 10)
	if !ok {
		return errors.New("failed to set reward account balance")
	}
	a.balance = balance
	return nil
}

func (a *rewardAccount) Encode() (systemcontracts.GenericValue, error) {
	data, err := a.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{
		AuxiliaryData: data,
	}, nil
}

func (a *rewardAccount) Decode(v systemcontracts.GenericValue) error {
	return a.Deserialize(v.AuxiliaryData)
}

// GrantBlockReward grants the block reward (token) to the block producer
func (p *Protocol) GrantBlockReward(
	ctx context.Context,
	sm protocol.StateManager,
) (*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	fCtx := protocol.MustGetFeatureCtx(ctx)

	if fCtx.UseV2Storage {
		// mark v1 reward history as not granted during the transition period
		var indexBytes [8]byte
		enc.MachineEndian.PutUint64(indexBytes[:], blkCtx.BlockHeight)
		key := append(_blockRewardHistoryKeyPrefix, indexBytes[:]...)
		err := p.deleteStateV1(sm, key, &rewardHistory{}, protocol.ErigonStoreOnlyOption())
		if err != nil && !errors.Is(err, state.ErrErigonStoreNotSupported) {
			return nil, err
		}
		// revert the changese for erigon storage optimazation
		defer func() {
			err = p.putStateV1(sm, key, &rewardHistory{}, protocol.ErigonStoreOnlyOption())
			if err != nil && !errors.Is(err, state.ErrErigonStoreNotSupported) {
				log.L().Panic("failed to put block reward history in Erigon store", zap.Error(err))
			}
		}()
	}

	if err := p.assertNoRewardYet(ctx, sm, _blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, err
	}

	producerAddrStr := blkCtx.Producer.String()
	rewardAddrStr := ""
	pp := poll.FindProtocol(protocol.MustGetRegistry(ctx))
	if pp != nil {
		candidates, err := pp.Candidates(ctx, sm)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if candidate.Address == producerAddrStr {
				rewardAddrStr = candidate.RewardAddress
				break
			}
		}
	}
	// If reward address doesn't exist, do nothing
	if rewardAddrStr == "" {
		log.S().Debugf("Producer %s doesn't have a reward address", producerAddrStr)
		return nil, nil
	}
	rewardAddr, err := address.FromString(rewardAddrStr)
	if err != nil {
		return nil, err
	}
	totalReward, blockReward, effectiveTip, err := p.calculateTotalRewardAndTip(ctx, sm)
	if err != nil {
		return nil, err
	}
	if err := p.updateAvailableBalance(ctx, sm, totalReward); err != nil {
		return nil, err
	}

	// IIP-59 §3.2 opt-in check: when the fork is on and the producer's
	// frozen poll snapshot has VoterRewardOnchainOptIn=true and a valid
	// DelegateProfile registration, route the base block reward into the
	// per-delegate pending pool. The pool is drained at epoch close by
	// GrantEpochReward, which emits a single batched DelegateDistributed
	// log covering both streams. Priority tip is fee income and stays
	// with the producer directly.
	optInPool := false
	var producerCandAddr address.Address
	if !fCtx.NoVoterRewardDistribution {
		producerCandAddr, err = address.FromString(producerAddrStr)
		if err != nil {
			return nil, err
		}
		snap, snapErr := staking.PollSnapshotFor(sm, producerCandAddr)
		switch {
		case snapErr == nil:
			optInPool = snap.VoterRewardOnchainOptIn && snap.Registered
		case errors.Is(snapErr, state.ErrStateNotExist):
			// Pre-fork block or delegate registered after the last freeze;
			// legacy grantToAccount path.
		default:
			return nil, errors.Wrapf(snapErr, "rewarding: read poll snapshot for %s", producerAddrStr)
		}
	}

	if optInPool {
		if err := p.creditPendingBlockRewardPool(ctx, sm, producerCandAddr.Bytes(), blockReward); err != nil {
			return nil, err
		}
		if effectiveTip.Sign() > 0 {
			if err := p.grantToAccount(ctx, sm, rewardAddr, effectiveTip); err != nil {
				return nil, err
			}
		}
	} else {
		if err := p.grantToAccount(ctx, sm, rewardAddr, totalReward); err != nil {
			return nil, err
		}
	}
	if err := p.updateRewardHistory(ctx, sm, _blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, err
	}

	// Legacy pre-dynamic-fee format: a bare RewardLog. Suppress entirely
	// on the opt-in pool path — the batched log is emitted at epoch close.
	if !fCtx.EnableDynamicFeeTx {
		if optInPool {
			return nil, nil
		}
		data, err := proto.Marshal(&rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   rewardAddrStr,
			Amount: blockReward.String(),
		})
		if err != nil {
			return nil, err
		}
		return &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		}, nil
	}

	// Post-dynamic-fee format: RewardLogs wrapper. On the opt-in pool
	// path the BLOCK_REWARD entry is omitted; the PRIORITY_BONUS entry
	// still applies because the tip is not folded into the voter split.
	var rewardLogs []*rewardingpb.RewardLog
	if !optInPool {
		rewardLogs = append(rewardLogs, &rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   rewardAddrStr,
			Amount: blockReward.String(),
		})
	}
	if !isZero(effectiveTip) {
		rewardLogs = append(rewardLogs, &rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_PRIORITY_BONUS,
			Addr:   rewardAddrStr,
			Amount: effectiveTip.String(),
		})
	}
	if len(rewardLogs) == 0 {
		return nil, nil
	}
	data, err := proto.Marshal(&rewardingpb.RewardLogs{Logs: rewardLogs})
	if err != nil {
		return nil, err
	}
	return &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actionCtx.ActionHash,
	}, nil
}

// GrantEpochReward dispatches the epoch-last-block work along one of
// two entirely separate paths:
//
//   - Pre-fork (NoVoterRewardDistribution=true): legacy single-block
//     grant. Slashing → per-delegate grantToAccount → foundation bonus
//     → sentinel. Never touches the IIP-59 cursor, pending pool, or
//     compound bridge. See grantLegacyEpochReward.
//   - Post-fork: IIP-59 Phase A only. Slashing → build + persist the
//     epochDrainCursor → apply slashing delta to fund. Voter reward
//     distribution (Phase B chunks + Phase C coda: orphan drain,
//     foundation bonus, sentinel, cursor delete) is deferred entirely
//     to GrantVoterRewardChunk on subsequent non-boundary blocks.
//
// Both paths share the pre-A checks (epoch-last block, no prior
// sentinel) and the slashing step. Post-fork additionally rejects any
// live cursor at entry — that's unambiguous corrupt state left by a
// previous drain's coda.
func (p *Protocol) GrantEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.TransactionLog, []*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)

	// Pre-A checks: must be epoch-last, must not have already granted.
	if err := p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	if err := p.assertLastBlockInEpoch(blkCtx.BlockHeight, epochNum, rp); err != nil {
		return nil, nil, err
	}

	// Post-fork: a live cursor at entry means the previous drain's coda
	// failed to delete it. Fail loud rather than silently continue.
	if !featureCtx.NoVoterRewardDistribution {
		existing, err := p.readEpochDrainCursor(ctx, sm)
		if err != nil {
			return nil, nil, err
		}
		if existing != nil {
			return nil, nil, errors.Errorf(
				"rewarding: cursor unexpectedly live at Phase A entry (era %d, index %d)",
				existing.TargetEra, existing.DelegateIndex)
		}
	}

	a, exemptAddrs, uqdMap, candidates, rewardedCandidates, addrs, amounts, err :=
		p.loadEpochDistributionInputs(ctx, sm, epochNum)
	if err != nil {
		return nil, nil, err
	}

	transactionLogs := make([]*action.TransactionLog, 0)
	rewardLogs := make([]*action.Log, 0)
	// actualTotalReward accumulates this block's net debit against
	// fund.unclaimedBalance. Slashing sends value back to the pool
	// (negative); grants pay it out (positive).
	actualTotalReward := big.NewInt(0)

	// Slashing runs on both paths — it is gated by its own feature flag
	// (NotSlashUnproductiveDelegates), independent of IIP-59.
	if !featureCtx.NotSlashUnproductiveDelegates {
		slashAmount, slashLogs, err := p.slashUqd(
			ctx, sm, blkCtx.BlockHeight, actionCtx.ActionHash,
			candidates, a.blockReward, uqdMap,
		)
		if err != nil {
			return nil, nil, err
		}
		if slashAmount.Sign() > 0 {
			transactionLogs = append(transactionLogs, &action.TransactionLog{
				Type:      iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND,
				Amount:    slashAmount,
				Sender:    address.StakingBucketPoolAddr,
				Recipient: address.RewardingPoolAddr,
			})
		}
		rewardLogs = append(rewardLogs, slashLogs...)
		actualTotalReward = new(big.Int).Sub(actualTotalReward, slashAmount)
	}

	if featureCtx.NoVoterRewardDistribution {
		// Pre-fork legacy: full single-block flow, no IIP-59 state.
		return p.grantLegacyEpochReward(
			ctx, sm, a, exemptAddrs, candidates,
			rewardedCandidates, addrs, amounts, epochNum,
			transactionLogs, rewardLogs, actualTotalReward,
		)
	}

	// Post-fork Phase A: freeze the delegate work list into a cursor,
	// then per-delegate: (a) IIP-59 path — grant commission to the reward
	// address, leaving voter pool distribution to Phase B; (b) fallback
	// path (opted out / not registered / no snapshot / nil reward addr) —
	// pay the full amount here via legacy grantToAccount and drain the
	// pool entry so Phase B has nothing to do for this delegate.
	// GrantBlockReward may keep crediting the pool after this point, but
	// the frozen snapshot pins the distribution basis so chunks stay
	// consistent across blocks.
	cursor, err := p.buildEpochDrainCursor(ctx, sm, epochNum, rewardedCandidates)
	if err != nil {
		return nil, nil, err
	}
	phaseALogs, phaseADebit, err := p.runPhaseADelegateGrants(
		ctx, sm, cursor, rewardedCandidates, addrs, amounts,
	)
	if err != nil {
		return nil, nil, err
	}
	rewardLogs = append(rewardLogs, phaseALogs...)
	actualTotalReward = new(big.Int).Add(actualTotalReward, phaseADebit)
	if err := p.writeEpochDrainCursor(ctx, sm, cursor); err != nil {
		return nil, nil, err
	}
	if err := p.updateAvailableBalance(ctx, sm, actualTotalReward); err != nil {
		return nil, nil, err
	}
	return transactionLogs, rewardLogs, nil
}

// runPhaseADelegateGrants performs the per-delegate Phase A grants inside
// the post-fork branch of GrantEpochReward. For each rewarded candidate
// with a live reward address it either (a) IIP-59 path — grants
// totalCommission to the reward address, leaving voter pool distribution
// to Phase B (VoterRewardChunk), or (b) fallback path — pays the full
// pool + epoch amount to the reward address via legacy grantToAccount and
// drains the pool entry. Returns the emitted reward logs and the
// unclaimed-balance debit accumulated across delegates (block-pool
// amounts already came out of unclaimed at GrantBlockReward time, so the
// debit only accumulates the epoch-stream portion of each grant).
func (p *Protocol) runPhaseADelegateGrants(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
	rewardedCandidates []*state.Candidate,
	addrs []address.Address,
	amounts []*big.Int,
) ([]*action.Log, *big.Int, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	rewardLogs := make([]*action.Log, 0)
	debit := new(big.Int)

	for i, cand := range rewardedCandidates {
		if cand == nil {
			continue
		}
		poolAmt := cursor.Delegates[i].PoolAmountFrozen
		if poolAmt == nil {
			poolAmt = new(big.Int)
		}
		epochAmt := amounts[i]
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		if addrs[i] == nil {
			// No reward address: leave the pool entry alone. Phase C's
			// orphan-drain sweep will route it to the delegate's live
			// reward address or refund unclaimedBalance.
			continue
		}
		if poolAmt.Sign() == 0 && epochAmt.Sign() == 0 {
			continue
		}

		alloc, err := p.prepareDelegateAllocation(ctx, sm, cand, addrs[i], poolAmt, epochAmt)
		if err != nil {
			return nil, nil, err
		}
		if alloc == nil {
			// Fallback (opt-out / no snapshot / unregistered): full
			// legacy pay + drain pool. Mirrors the pre-C2.1 Phase B
			// legacy branch, moved forward so VoterRewardChunk stays
			// voter-only.
			candBytes := cursor.Delegates[i].CandidateIdentifier
			if poolAmt.Sign() > 0 {
				if err := p.grantToAccount(ctx, sm, addrs[i], poolAmt); err != nil {
					return nil, nil, err
				}
				if err := p.deletePendingBlockRewardPool(ctx, sm, candBytes); err != nil {
					return nil, nil, err
				}
				data, err := p.encodeRewardLog(rewardingpb.RewardLog_BLOCK_REWARD, addrs[i].String(), poolAmt)
				if err != nil {
					return nil, nil, err
				}
				rewardLogs = append(rewardLogs, &action.Log{
					Address:     p.addr.String(),
					Topics:      nil,
					Data:        data,
					BlockHeight: blkCtx.BlockHeight,
					ActionHash:  actionCtx.ActionHash,
				})
			}
			if epochAmt.Sign() > 0 {
				if err := p.grantToAccount(ctx, sm, addrs[i], epochAmt); err != nil {
					return nil, nil, err
				}
				data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, addrs[i].String(), epochAmt)
				if err != nil {
					return nil, nil, err
				}
				rewardLogs = append(rewardLogs, &action.Log{
					Address:     p.addr.String(),
					Topics:      nil,
					Data:        data,
					BlockHeight: blkCtx.BlockHeight,
					ActionHash:  actionCtx.ActionHash,
				})
				debit = new(big.Int).Add(debit, epochAmt)
			}
			continue
		}

		// IIP-59 path: grant commission only. Voter pool distribution
		// (and the DelegateDistributed log) run in Phase B on the
		// continuation VoterRewardChunk block(s). The empty-voter
		// fallback surfaces here as totalCommission == poolAmt+epochAmt
		// with voterPool == 0, so Phase A grants everything and Phase B
		// still emits an attestation log with zero voter shares.
		if alloc.totalCommission.Sign() > 0 {
			if err := p.grantToAccount(ctx, sm, addrs[i], alloc.totalCommission); err != nil {
				return nil, nil, errors.Wrapf(err,
					"rewarding: credit commission to %s failed", addrs[i].String())
			}
			debit = new(big.Int).Add(debit, alloc.epochCommission)
		}
	}
	return rewardLogs, debit, nil
}

// grantLegacyEpochReward runs the pre-IIP-59 single-block epoch grant:
// iterate all candidates, pay epoch reward directly to each reward
// address, then foundation bonus, then write the epoch sentinel. Never
// touches the IIP-59 cursor, pending block-reward pool, or compound
// bridge — those state slots are inert on pre-fork chains.
//
// Callers pass in any pre-accumulated logs and balance delta from the
// slashing step (which runs before the fork branch in GrantEpochReward).
func (p *Protocol) grantLegacyEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
	a *admin,
	exemptAddrs map[string]interface{},
	candidates []*state.Candidate,
	rewardedCandidates []*state.Candidate,
	addrs []address.Address,
	amounts []*big.Int,
	epochNum uint64,
	transactionLogs []*action.TransactionLog,
	rewardLogs []*action.Log,
	actualTotalReward *big.Int,
) ([]*action.TransactionLog, []*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	for i, cand := range rewardedCandidates {
		if cand == nil || addrs[i] == nil {
			continue
		}
		epochAmt := amounts[i]
		if epochAmt == nil || epochAmt.Sign() == 0 {
			continue
		}
		if err := p.grantToAccount(ctx, sm, addrs[i], epochAmt); err != nil {
			return nil, nil, err
		}
		data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, addrs[i].String(), epochAmt)
		if err != nil {
			return nil, nil, err
		}
		rewardLogs = append(rewardLogs, &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actionCtx.ActionHash,
		})
		actualTotalReward = new(big.Int).Add(actualTotalReward, epochAmt)
	}

	if a.grantFoundationBonus(epochNum) || (epochNum >= p.cfg.FoundationBonusP2StartEpoch && epochNum <= p.cfg.FoundationBonusP2EndEpoch) {
		for i, count := 0, uint64(0); i < len(candidates) && count < a.numDelegatesForFoundationBonus; i++ {
			if _, ok := exemptAddrs[candidates[i].Address]; ok {
				continue
			}
			if candidates[i].Votes.Cmp(big.NewInt(0)) == 0 {
				// hard probation
				continue
			}
			count++
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			rewardAddr, err := address.FromString(candidates[i].RewardAddress)
			if err != nil {
				return nil, nil, err
			}
			if err := p.grantToAccount(ctx, sm, rewardAddr, a.foundationBonus); err != nil {
				return nil, nil, err
			}
			data, err := p.encodeRewardLog(rewardingpb.RewardLog_FOUNDATION_BONUS, candidates[i].RewardAddress, a.foundationBonus)
			if err != nil {
				return nil, nil, err
			}
			rewardLogs = append(rewardLogs, &action.Log{
				Address:     p.addr.String(),
				Topics:      nil,
				Data:        data,
				BlockHeight: blkCtx.BlockHeight,
				ActionHash:  actionCtx.ActionHash,
			})
			actualTotalReward = new(big.Int).Add(actualTotalReward, a.foundationBonus)
		}
	}

	if err := p.updateAvailableBalance(ctx, sm, actualTotalReward); err != nil {
		return nil, nil, err
	}
	if err := p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	return transactionLogs, rewardLogs, nil
}

// GrantVoterRewardChunk advances one chunk of an in-progress IIP-59
// era-boundary drain. Emitted by CreatePostSystemActions on every
// non-epoch-boundary block while a cursor is live; the final chunk
// runs the coda (orphan drain, foundation bonus, sentinel, cursor
// delete) inline.
//
// Epoch-scoped inputs (admin, exempt, candidates, splitEpochReward) are
// derived from cursor.TargetEra — the epoch that triggered the drain —
// NOT the current block's epoch, so cross-era continuation runs use the
// same partitioning Phase A froze into the cursor.
func (p *Protocol) GrantVoterRewardChunk(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.TransactionLog, []*action.Log, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		// Defense in depth: CreatePostSystemActions never emits this
		// action pre-fork, and Validate rejects manually crafted ones.
		return nil, nil, errors.New("rewarding: voter reward chunk action requires IIP-59 fork")
	}

	cursor, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	if cursor == nil {
		// Dispatcher invariant: cursor must be live when this handler
		// runs. Reaching here means CreatePostSystemActions or a state
		// migration got out of sync.
		return nil, nil, errors.New("rewarding: voter reward chunk dispatched without a live cursor")
	}

	a, exemptAddrs, _, candidates, rewardedCandidates, addrs, amounts, err :=
		p.loadEpochDistributionInputs(ctx, sm, cursor.TargetEra)
	if err != nil {
		return nil, nil, err
	}

	return p.runVoterDistributionChunk(
		ctx, sm, cursor, a, exemptAddrs, candidates,
		rewardedCandidates, addrs, amounts, cursor.TargetEra,
		make([]*action.TransactionLog, 0),
		make([]*action.Log, 0),
		big.NewInt(0),
	)
}

// loadEpochDistributionInputs re-derives the deterministic epoch-scoped
// state that both Phase A and continuation chunks need: admin config,
// exempt set, uqdMap, poll candidates, and the splitEpochReward
// partition (rewardedCandidates, addrs, amounts). epochNum is the
// original epoch the drain targets, so continuation blocks pass
// cursor.TargetEra rather than the block's own epoch.
func (p *Protocol) loadEpochDistributionInputs(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
) (
	*admin, map[string]interface{}, map[string]uint64,
	[]*state.Candidate, []*state.Candidate,
	[]address.Address, []*big.Int, error,
) {
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	pp := poll.MustGetProtocol(protocol.MustGetRegistry(ctx))

	a := &admin{}
	if _, err := p.state(ctx, sm, _adminKey, a); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	e := exempt{}
	if _, err := p.state(ctx, sm, _exemptKey, &e); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	exemptAddrs := make(map[string]interface{})
	for _, addr := range e.addrs {
		exemptAddrs[addr.String()] = nil
	}

	uqdMap := make(map[string]uint64)
	epochStartHeight := rp.GetEpochHeight(epochNum)
	if featureWithHeightCtx.GetUnproductiveDelegates(epochStartHeight) || !featureCtx.NotSlashUnproductiveDelegates {
		var err error
		uqdMap, err = pp.CalculateUnproductiveDelegates(ctx, sm)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	candidates, err := pp.Candidates(ctx, sm)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	epochRewardSplitUqdMap := make(map[string]uint64)
	if featureWithHeightCtx.GetUnproductiveDelegates(epochStartHeight) {
		epochRewardSplitUqdMap = uqdMap
	}
	rewardedCandidates, addrs, amounts, err := p.splitEpochReward(
		candidates, a.epochReward, a.numDelegatesForEpochReward, exemptAddrs, epochRewardSplitUqdMap,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	return a, exemptAddrs, uqdMap, candidates, rewardedCandidates, addrs, amounts, nil
}

// runVoterDistributionChunk processes the next [cursor.DelegateIndex,
// +chunkSize) slice of the frozen work list. If the slice reaches the
// end of the list it also runs the coda (orphan drain, foundation
// bonus, sentinel, cursor delete). Mid-drain runs persist the advanced
// cursor and apply this block's balance delta.
//
// Post-fork only — GrantVoterRewardChunk is the sole caller.
// chunkSize == 0 (governance opt-out) collapses the loop to a single
// pass over the entire list.
func (p *Protocol) runVoterDistributionChunk(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
	a *admin,
	exemptAddrs map[string]interface{},
	candidates []*state.Candidate,
	rewardedCandidates []*state.Candidate,
	addrs []address.Address,
	amounts []*big.Int,
	epochNum uint64,
	transactionLogs []*action.TransactionLog,
	rewardLogs []*action.Log,
	actualTotalReward *big.Int,
) ([]*action.TransactionLog, []*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	// Phase B: process the next [startIdx, endIdx) slice of the frozen
	// work list.
	chunkSize := p.epochDrainChunkSize(ctx)
	startIdx := cursor.DelegateIndex
	endIdx := uint32(len(cursor.Delegates))
	if chunkSize > 0 && startIdx+chunkSize < endIdx {
		endIdx = startIdx + chunkSize
	}
	for i := startIdx; i < endIdx; i++ {
		cand := rewardedCandidates[i]
		if cand == nil {
			continue
		}
		candBytes := cursor.Delegates[i].CandidateIdentifier
		poolAmt := cursor.Delegates[i].PoolAmountFrozen
		if poolAmt == nil {
			poolAmt = new(big.Int)
		}
		epochAmt := amounts[i]
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		// No reward address: leave the pool entry (if any) alone — the
		// orphan-drain sweep in the coda routes it to the delegate's
		// live reward address or refunds to unclaimedBalance.
		if addrs[i] == nil {
			continue
		}
		// No streams to distribute for this candidate.
		if poolAmt.Sign() == 0 && epochAmt.Sign() == 0 {
			continue
		}
		// Phase B (voter-only). Commission was already granted in Phase A
		// (GrantEpochReward). Fallback delegates (opt-out, no snapshot,
		// unregistered, etc.) had their full amount paid in Phase A too
		// — for those, prepareDelegateAllocation returns nil and this
		// chunk simply skips.
		alloc, err := p.prepareDelegateAllocation(ctx, sm, cand, addrs[i], poolAmt, epochAmt)
		if err != nil {
			return nil, nil, err
		}
		if alloc == nil {
			continue
		}
		iip59Logs, err := p.distributeVoterFromAllocation(
			ctx, sm, cand, addrs[i], alloc,
			blkCtx.BlockHeight, actionCtx.ActionHash,
		)
		if err != nil {
			return nil, nil, err
		}
		rewardLogs = append(rewardLogs, iip59Logs...)
		// Phase B unclaimed-balance debit = epoch voter pool. Phase A
		// already debited epochCommission for this delegate.
		actualTotalReward = new(big.Int).Add(actualTotalReward, alloc.epochVoterPool)
		if poolAmt.Sign() > 0 {
			if err := p.deletePendingBlockRewardPool(ctx, sm, candBytes); err != nil {
				return nil, nil, err
			}
		}
	}

	if endIdx < uint32(len(cursor.Delegates)) {
		// Drain still in progress. Persist the cursor, apply this
		// block's balance delta, and let CreatePostSystemActions emit
		// the continuation VoterRewardChunk grant on the next block.
		cursor.DelegateIndex = endIdx
		if err := p.writeEpochDrainCursor(ctx, sm, cursor); err != nil {
			return nil, nil, err
		}
		if err := p.updateAvailableBalance(ctx, sm, actualTotalReward); err != nil {
			return nil, nil, err
		}
		return transactionLogs, rewardLogs, nil
	}

	// Phase C (coda): orphan drain, foundation bonus, sentinel, delete
	// cursor. Runs on the last chunk of the drain (which equals Phase A
	// when the delegate count fits in a single block).
	//
	// visited covers every delegate in the frozen work list that had a
	// reward address across all chunks, mirroring the single-block
	// visitedPoolIDs semantics. Pool entries not in visited are either
	// delegates that dropped their reward address mid-epoch or delegates
	// that fell off the poll list before the drain reached them; both
	// are routed by the orphan sweep.
	visitedPoolIDs := make(map[string]bool, len(cursor.Delegates))
	for i, d := range cursor.Delegates {
		if addrs[i] == nil {
			continue
		}
		visitedPoolIDs[string(d.CandidateIdentifier)] = true
	}
	orphanLogs, err := p.drainPendingBlockRewardOrphans(
		ctx, sm, visitedPoolIDs, blkCtx.BlockHeight, actionCtx.ActionHash,
	)
	if err != nil {
		return nil, nil, err
	}
	rewardLogs = append(rewardLogs, orphanLogs...)

	// Foundation bonus (unchunked — small, bounded work).
	if a.grantFoundationBonus(epochNum) || (epochNum >= p.cfg.FoundationBonusP2StartEpoch && epochNum <= p.cfg.FoundationBonusP2EndEpoch) {
		for i, count := 0, uint64(0); i < len(candidates) && count < a.numDelegatesForFoundationBonus; i++ {
			if _, ok := exemptAddrs[candidates[i].Address]; ok {
				continue
			}
			if candidates[i].Votes.Cmp(big.NewInt(0)) == 0 {
				// hard probation
				continue
			}
			count++
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			rewardAddr, err := address.FromString(candidates[i].RewardAddress)
			if err != nil {
				return nil, nil, err
			}
			if err := p.grantToAccount(ctx, sm, rewardAddr, a.foundationBonus); err != nil {
				return nil, nil, err
			}
			data, err := p.encodeRewardLog(rewardingpb.RewardLog_FOUNDATION_BONUS, candidates[i].RewardAddress, a.foundationBonus)
			if err != nil {
				return nil, nil, err
			}
			rewardLogs = append(rewardLogs, &action.Log{
				Address:     p.addr.String(),
				Topics:      nil,
				Data:        data,
				BlockHeight: blkCtx.BlockHeight,
				ActionHash:  actionCtx.ActionHash,
			})
			actualTotalReward = new(big.Int).Add(actualTotalReward, a.foundationBonus)
		}
	}

	if err := p.updateAvailableBalance(ctx, sm, actualTotalReward); err != nil {
		return nil, nil, err
	}
	// Sentinel is written against the era that triggered the drain,
	// not the current block's epoch — matters for cross-era continuation.
	if err := p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, cursor.TargetEra); err != nil {
		return nil, nil, err
	}
	if err := p.deleteEpochDrainCursor(ctx, sm); err != nil {
		return nil, nil, err
	}
	return transactionLogs, rewardLogs, nil
}

// epochDrainChunkSize returns the maximum number of delegates to process
// per block during the IIP-59 era-boundary drain. Zero means unbounded —
// the loop runs to completion in a single block and no cursor is written.
// This is the behavior before the fork gate opens and whenever
// CompoundBatchSize is left at 0 (single-block genesis parity).
func (p *Protocol) epochDrainChunkSize(ctx context.Context) uint32 {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return 0
	}
	return uint32(p.cfg.CompoundBatchSize)
}

// buildEpochDrainCursor freezes the delegate work list for a fresh Phase
// A: for each rewarded candidate it captures the identifier bytes and the
// current pool balance. Later chunks read PoolAmountFrozen so continued
// GrantBlockReward credits into the same delegate's pool don't inflate
// this drain's payout.
func (p *Protocol) buildEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateReader,
	epochNum uint64,
	rewardedCandidates []*state.Candidate,
) (*epochDrainCursor, error) {
	c := &epochDrainCursor{
		TargetEra: epochNum,
		Delegates: make([]epochDrainDelegateWork, len(rewardedCandidates)),
	}
	for i, cand := range rewardedCandidates {
		if cand == nil {
			c.Delegates[i] = epochDrainDelegateWork{PoolAmountFrozen: new(big.Int)}
			continue
		}
		candBytes, err := candidateIdentifierBytes(cand.Address)
		if err != nil {
			return nil, err
		}
		poolAmt, err := p.readPendingBlockRewardPool(ctx, sm, candBytes)
		if err != nil {
			return nil, err
		}
		c.Delegates[i] = epochDrainDelegateWork{
			CandidateIdentifier: candBytes,
			PoolAmountFrozen:    poolAmt,
		}
	}
	return c, nil
}

func (p *Protocol) encodeRewardLog(
	rewardType rewardingpb.RewardLog_RewardType,
	addr string,
	amount *big.Int,
) ([]byte, error) {
	rewardLog := rewardingpb.RewardLog{
		Type:   rewardType,
		Addr:   addr,
		Amount: amount.String(),
	}
	return proto.Marshal(&rewardLog)
}

// Claim claims the token from the rewarding fund
func (p *Protocol) Claim(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	claimFrom address.Address,
) (*action.TransactionLog, error) {
	if err := p.assertAmount(amount); err != nil {
		return nil, err
	}
	if err := p.updateTotalBalance(ctx, sm, amount); err != nil {
		return nil, err
	}
	if err := p.claimFromAccount(ctx, sm, claimFrom, amount); err != nil {
		return nil, err
	}

	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND,
		Sender:    address.RewardingPoolAddr,
		Recipient: claimFrom.String(),
		Amount:    amount,
	}, nil
}

// UnclaimedBalance returns unclaimed balance of a given address
func (p *Protocol) UnclaimedBalance(
	ctx context.Context,
	sm protocol.StateReader,
	addr address.Address,
) (*big.Int, uint64, error) {
	acc := rewardAccount{}
	accKey := append(_adminKey, addr.Bytes()...)
	height, err := p.state(ctx, sm, accKey, &acc)
	if err == nil {
		return acc.balance, height, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		return big.NewInt(0), height, nil
	}
	return nil, height, err
}

func (p *Protocol) updateTotalBalance(ctx context.Context, sm protocol.StateManager, amount *big.Int) error {
	f := fund{}
	if _, err := p.state(ctx, sm, _fundKey, &f); err != nil {
		return err
	}
	totalBalance := big.NewInt(0).Sub(f.totalBalance, amount)
	if totalBalance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough total balance")
	}
	f.totalBalance = totalBalance
	return p.putState(ctx, sm, _fundKey, &f)
}

func (p *Protocol) updateAvailableBalance(ctx context.Context, sm protocol.StateManager, amount *big.Int) error {
	f := fund{}
	if _, err := p.state(ctx, sm, _fundKey, &f); err != nil {
		return err
	}
	availableBalance := big.NewInt(0).Sub(f.unclaimedBalance, amount)
	if availableBalance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough available balance")
	}
	f.unclaimedBalance = availableBalance
	return p.putState(ctx, sm, _fundKey, &f)
}

func (p *Protocol) grantToAccount(ctx context.Context, sm protocol.StateManager, addr address.Address, amount *big.Int) error {
	acc := rewardAccount{}
	accKey := append(_adminKey, addr.Bytes()...)
	_, fromLegacy, err := p.stateCheckLegacy(ctx, sm, accKey, &acc)
	if err != nil {
		if errors.Cause(err) != state.ErrStateNotExist {
			return err
		}
		acc = rewardAccount{
			balance: big.NewInt(0),
		}
	} else {
		// entry exist
		// check if from legacy, and we have started using v2, delete v1
		if fromLegacy && useV2Storage(ctx) {
			if err := p.deleteStateV1(sm, accKey, &rewardAccount{}); err != nil {
				return err
			}
		}
	}
	acc.balance = big.NewInt(0).Add(acc.balance, amount)
	return p.putState(ctx, sm, accKey, &acc)
}

func (p *Protocol) claimFromAccount(ctx context.Context, sm protocol.StateManager, addr address.Address, amount *big.Int) error {
	// Update reward account
	acc := rewardAccount{}
	accKey := append(_adminKey, addr.Bytes()...)
	_, fromLegacy, err := p.stateCheckLegacy(ctx, sm, accKey, &acc)
	if err != nil {
		return err
	}
	balance := big.NewInt(0).Sub(acc.balance, amount)
	if balance.Cmp(big.NewInt(0)) < 0 {
		return errors.New("no enough available balance")
	}
	// TODO: we may want to delete the account when the unclaimed balance becomes 0
	acc.balance = balance
	if err := p.putState(ctx, sm, accKey, &acc); err != nil {
		return err
	}
	if fromLegacy && useV2Storage(ctx) {
		if err := p.deleteStateV1(sm, accKey, &rewardAccount{}); err != nil {
			return err
		}
	}
	accountCreationOpts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
	}
	// Update primary account
	primAcc, err := accountutil.LoadOrCreateAccount(sm, addr, accountCreationOpts...)
	if err != nil {
		return err
	}
	if err := primAcc.AddBalance(amount); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, addr, primAcc)
}

func (p *Protocol) calculateTotalRewardAndTip(ctx context.Context, sm protocol.StateManager) (*big.Int, *big.Int, *big.Int, error) {
	a := admin{}
	if _, err := p.state(ctx, sm, _adminKey, &a); err != nil {
		return nil, nil, nil, err
	}
	var (
		blkCtx       = protocol.MustGetBlockCtx(ctx)
		featureCtx   = protocol.MustGetFeatureCtx(ctx)
		totalReward  = &big.Int{}
		blockReward  = (&big.Int{}).Set(a.blockReward)
		effectiveTip = &big.Int{}
	)
	if featureCtx.EnableDynamicFeeTx {
		if blkCtx.AccumulatedTips.Sign() > 0 {
			effectiveTip.Set(&blkCtx.AccumulatedTips)
		}
	}
	totalReward.Add(blockReward, effectiveTip)
	return totalReward, blockReward, effectiveTip, nil
}

func (p *Protocol) updateRewardHistory(ctx context.Context, sm protocol.StateManager, prefix []byte, index uint64) error {
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], index)
	return p.putState(ctx, sm, append(prefix, indexBytes[:]...), &rewardHistory{})
}

func (p *Protocol) slashDelegate(
	ctx context.Context,
	sm protocol.StateManager,
	stakingProtocol *staking.Protocol,
	blockHeight uint64,
	actionHash hash.Hash256,
	candidate *state.Candidate,
	amount *big.Int,
) (*action.Log, error) {
	var candidateAddr address.Address
	var err error
	switch {
	case !protocol.MustGetFeatureWithHeightCtx(ctx).CandidateWithoutIdentity(blockHeight):
		if candidate.Identity != "" {
			candidateAddr, err = address.FromString(candidate.Identity)
			if err != nil {
				return nil, err
			}
			if err := stakingProtocol.SlashCandidateByID(ctx, sm, candidateAddr, amount); err != nil {
				return nil, errors.Wrapf(err, "failed to slash candidate %s", candidate.Identity)
			}
			break
		}
		fallthrough
	case protocol.MustGetFeatureCtx(ctx).CandidateSlashByOwner:
		candidateAddr, err = address.FromString(candidate.Address)
		if err != nil {
			return nil, err
		}
		if err := stakingProtocol.SlashCandidateByID(ctx, sm, candidateAddr, amount); err != nil {
			return nil, errors.Wrapf(err, "failed to slash candidate %s", candidate.Address)
		}
		break
	default:
		candidateAddr, err = address.FromString(candidate.Address)
		if err != nil {
			return nil, err
		}
		if err := stakingProtocol.SlashCandidateByOperator(ctx, sm, candidateAddr, amount); err != nil {
			return nil, errors.Wrapf(err, "failed to slash candidate %s", candidate.Address)
		}
	}
	data, err := p.encodeRewardLog(rewardingpb.RewardLog_UNPRODUCTIVE_SLASH, candidateAddr.String(), amount)
	if err != nil {
		return nil, err
	}

	return &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: blockHeight,
		ActionHash:  actionHash,
	}, nil
}

func (p *Protocol) slashUqd(
	ctx context.Context,
	sm protocol.StateManager,
	blockHeight uint64,
	actionHash hash.Hash256,
	candidates []*state.Candidate,
	slashRate *big.Int,
	uqdMap map[string]uint64,
) (*big.Int, []*action.Log, error) {
	totalSlashAmount := big.NewInt(0)
	stakingProtocol := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProtocol == nil {
		return nil, nil, errors.New("staking protocol not found")
	}
	view, err := sm.ReadView(stakingProtocol.Name())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read view of staking protocol")
	}
	slashLogs := make([]*action.Log, 0)
	snapshot := view.Snapshot()
	fCtx := protocol.MustGetFeatureCtx(ctx)
	usingOperator := protocol.MustGetFeatureWithHeightCtx(ctx).CandidateWithoutIdentity(blockHeight)
	for _, candidate := range candidates {
		id := candidate.Identity
		if usingOperator {
			id = candidate.Address
		}
		if missed, ok := uqdMap[id]; ok {
			if missed == 0 {
				// hard probation, no slash
				continue
			}
			amount := big.NewInt(0).Mul(slashRate, big.NewInt(0).SetUint64(missed))
			actLog, err := p.slashDelegate(ctx, sm, stakingProtocol, blockHeight, actionHash, candidate, amount)
			switch errors.Cause(err) {
			case nil:
				slashLogs = append(slashLogs, actLog)
				totalSlashAmount.Add(totalSlashAmount, amount)
			case staking.ErrNoSelfStakeBucket:
				log.S().Errorf("Candidate %s doesn't have self-stake bucket, no slash", id)
			case staking.ErrCandidateNotExist:
				if !fCtx.CandidateSlashByOwner {
					log.S().Errorf("Candidate %s doesn't exist, ignore slash", id)
					continue
				}
				fallthrough
			default:
				if err := view.Revert(snapshot); err != nil {
					return nil, nil, errors.Wrap(err, "failed to revert view")
				}
				return nil, nil, err
			}
		}
	}
	return totalSlashAmount, slashLogs, nil
}

func (p *Protocol) splitEpochReward(
	candidates []*state.Candidate,
	totalAmount *big.Int,
	numDelegatesForEpochReward uint64,
	exemptAddrs map[string]interface{},
	uqd map[string]uint64,
) ([]*state.Candidate, []address.Address, []*big.Int, error) {
	filteredCandidates := make([]*state.Candidate, 0)
	for _, candidate := range candidates {
		if _, ok := exemptAddrs[candidate.Address]; ok {
			continue
		}
		filteredCandidates = append(filteredCandidates, candidate)
	}
	candidates = filteredCandidates
	if len(candidates) == 0 {
		return nil, nil, nil, nil
	}
	// We at most allow numDelegatesForEpochReward delegates to get the epoch reward
	if uint64(len(candidates)) > numDelegatesForEpochReward {
		candidates = candidates[:numDelegatesForEpochReward]
	}
	totalWeight := big.NewInt(0)
	rewardAddrs := make([]address.Address, 0)
	for _, candidate := range candidates {
		var rewardAddr address.Address
		var err error
		if candidate.RewardAddress != "" {
			rewardAddr, err = address.FromString(candidate.RewardAddress)
			if err != nil {
				return nil, nil, nil, err
			}
		} else {
			log.S().Warnf("Candidate %s doesn't have a reward address", candidate.Address)
		}
		rewardAddrs = append(rewardAddrs, rewardAddr)
		totalWeight = big.NewInt(0).Add(totalWeight, candidate.Votes)
	}
	amounts := make([]*big.Int, 0)
	var amountPerAddr *big.Int
	for _, candidate := range candidates {
		if totalWeight.Cmp(big.NewInt(0)) == 0 {
			amounts = append(amounts, big.NewInt(0))
			continue
		}
		if _, ok := uqd[candidate.Address]; ok {
			// Before Easter, if not qualified, skip the epoch reward
			amounts = append(amounts, big.NewInt(0))
			continue
		}
		amountPerAddr = big.NewInt(0).Div(big.NewInt(0).Mul(totalAmount, candidate.Votes), totalWeight)
		amounts = append(amounts, amountPerAddr)
	}
	return candidates, rewardAddrs, amounts, nil
}

func (p *Protocol) assertNoRewardYet(ctx context.Context, sm protocol.StateManager, prefix []byte, index uint64) error {
	history := rewardHistory{}
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], index)
	_, err := p.state(ctx, sm, append(prefix, indexBytes[:]...), &history)
	if err == nil {
		return errors.Errorf("reward history already exists on index %d", index)
	}
	if errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	return nil
}

func (p *Protocol) assertLastBlockInEpoch(blkHeight uint64, epochNum uint64, rp *rolldpos.Protocol) error {
	lastBlkHeight := rp.GetEpochLastBlockHeight(epochNum)
	if blkHeight != lastBlkHeight {
		return errors.Errorf("current block %d is not the last block of epoch %d", blkHeight, epochNum)
	}
	return nil
}

// UnmarshalRewardLog unmarshals reward log from byte slice
// it keep the compatibility with old reward log
func UnmarshalRewardLog(data []byte) (*rewardingpb.RewardLogs, error) {
	logs := rewardingpb.RewardLogs{}
	if err := proto.Unmarshal(data, &logs); err != nil {
		return nil, err
	}
	if len(logs.Logs) == 0 {
		// compatibility with old reward log
		log := rewardingpb.RewardLog{}
		if err := proto.Unmarshal(data, &log); err != nil {
			return nil, err
		}
		logs = rewardingpb.RewardLogs{
			Logs: []*rewardingpb.RewardLog{&log},
		}
	}
	return &logs, nil
}
