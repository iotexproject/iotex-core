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
	// DelegateProfile registration, split the base block reward now
	// using the snapshot's BlockCommissionBasisPoints — commission to
	// the reward address immediately, voter portion into the per-delegate
	// pending pool. The pool is drained at era boundary by the
	// VoterRewardChunk system action. Priority tip is fee income and
	// stays with the producer directly.
	optInPool := false
	var producerCandAddr address.Address
	var blockCommissionBPs uint64
	if !fCtx.NoVoterRewardDistribution {
		producerCandAddr, err = address.FromString(producerAddrStr)
		if err != nil {
			return nil, err
		}
		snap, snapErr := staking.PollSnapshotFor(sm, producerCandAddr)
		switch {
		case snapErr == nil:
			optInPool = snap.VoterRewardOnchainOptIn && snap.Registered
			if optInPool {
				blockCommissionBPs = snap.BlockCommissionBasisPoints
			}
		case errors.Is(snapErr, state.ErrStateNotExist):
			// Pre-fork block or delegate registered after the last freeze;
			// legacy grantToAccount path.
		default:
			return nil, errors.Wrapf(snapErr, "rewarding: read poll snapshot for %s", producerAddrStr)
		}
	}

	// blockCommission is the amount named in the emitted BLOCK_REWARD log
	// for both legacy and opt-in paths. Legacy path: full blockReward.
	// Opt-in path: split by snapshot BPs — commission paid to rewardAddr
	// immediately; voter share credited to the pending pool. Empty-voter
	// or BPs=10000 fallback collapses naturally (voter share = 0).
	blockCommission := blockReward
	if optInPool {
		commission, voterShare := splitCommission(blockReward, blockCommissionBPs)
		blockCommission = commission
		if commission.Sign() > 0 {
			if err := p.grantToAccount(ctx, sm, rewardAddr, commission); err != nil {
				return nil, err
			}
		}
		if voterShare.Sign() > 0 {
			if err := p.creditPendingBlockRewardPool(ctx, sm, producerCandAddr.Bytes(), voterShare); err != nil {
				return nil, err
			}
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

	// Legacy pre-dynamic-fee format: a bare RewardLog. On the opt-in path
	// the amount is the commission (voter share is attested by the
	// batched DelegateDistributed log at era close). When the commission
	// is zero (BPs=0 with non-empty voter list), emit nothing.
	if !fCtx.EnableDynamicFeeTx {
		if optInPool && blockCommission.Sign() == 0 {
			return nil, nil
		}
		data, err := proto.Marshal(&rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   rewardAddrStr,
			Amount: blockCommission.String(),
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

	// Post-dynamic-fee format: RewardLogs wrapper. BLOCK_REWARD entry
	// names rewardAddr with the immediate payout (commission on opt-in,
	// full reward otherwise). Suppress the entry when the amount is zero.
	var rewardLogs []*rewardingpb.RewardLog
	if blockCommission.Sign() > 0 {
		rewardLogs = append(rewardLogs, &rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   rewardAddrStr,
			Amount: blockCommission.String(),
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

// GrantEpochReward runs the epoch-last-block work as a single body for
// both pre- and post-fork chains:
//
//  1. Pre-A checks: epoch-last, no prior sentinel, no live cursor (post-fork).
//  2. Load inputs: admin config, exempt set, uqd map, candidates, split partition.
//  3. Slashing (its own feature flag; independent of IIP-59).
//  4. Per-delegate epoch split loop — for each rewarded candidate:
//     - splitDelegateEpochReward returns (commission, voterShare) — the
//       IIP-59 path when the frozen snapshot is opted in / registered
//       with non-empty voters; (amount, 0) otherwise (fork off, opt out,
//       no snapshot, unregistered, empty voter list).
//     - grant commission to the reward address (EPOCH_REWARD log).
//     - if voterShare > 0, credit into the delegate's pending pool.
//     - if pool + voterShare > 0, append a cursor entry for Phase B to
//       drain. Zero-voter delegates never enter the cursor.
//  5. Foundation bonus.
//  6. Persist cursor iff any entries (post-fork only).
//  7. Sentinel.
//  8. Apply net debit.
//
// Pre-fork: splitDelegateEpochReward returns (amount, 0) for every
// delegate; no snapshot lookup, no pool touches, no cursor written; the
// coda (foundation bonus + sentinel) runs here and the function returns
// without a continuation handoff.
func (p *Protocol) GrantEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.TransactionLog, []*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)

	// Pre-A checks.
	if err := p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	if err := p.assertLastBlockInEpoch(blkCtx.BlockHeight, epochNum, rp); err != nil {
		return nil, nil, err
	}
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
	// debit accumulates this block's net delta against fund.unclaimedBalance:
	// slashing returns value (negative), grants + pool credits pay out
	// (positive). Block-time voter credits were already debited at
	// GrantBlockReward time.
	debit := big.NewInt(0)

	// Slashing (gated by NotSlashUnproductiveDelegates, independent of IIP-59).
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
		debit = new(big.Int).Sub(debit, slashAmount)
	}

	// Per-delegate epoch split.
	var cursorEntries []epochDrainDelegateWork
	for i, cand := range rewardedCandidates {
		if cand == nil || addrs[i] == nil {
			continue
		}
		epochAmt := amounts[i]
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		commission, voterShare, err := p.splitDelegateEpochReward(ctx, sm, cand, epochAmt)
		if err != nil {
			return nil, nil, err
		}
		if commission.Sign() > 0 {
			if err := p.grantToAccount(ctx, sm, addrs[i], commission); err != nil {
				return nil, nil, errors.Wrapf(err,
					"rewarding: credit commission to %s failed", addrs[i].String())
			}
			data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, addrs[i].String(), commission)
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
			debit = new(big.Int).Add(debit, commission)
		}
		// Cursor only materializes on the post-fork IIP-59 path when the
		// frozen snapshot yields voter share, or when the pool already
		// carries block-side voter accrual for this delegate.
		if featureCtx.NoVoterRewardDistribution {
			continue
		}
		candID, err := candidateIdentifierBytes(cand.Address)
		if err != nil {
			return nil, nil, err
		}
		poolAccrued, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		if err != nil {
			return nil, nil, err
		}
		if voterShare.Sign() > 0 {
			if err := p.creditPendingBlockRewardPool(ctx, sm, candID, voterShare); err != nil {
				return nil, nil, err
			}
			debit = new(big.Int).Add(debit, voterShare)
		}
		totalVoter := new(big.Int).Add(poolAccrued, voterShare)
		if totalVoter.Sign() > 0 {
			cursorEntries = append(cursorEntries, epochDrainDelegateWork{
				CandidateIdentifier: candID,
				VoterAmountFrozen:   totalVoter,
			})
		}
	}

	// Foundation bonus.
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
			debit = new(big.Int).Add(debit, a.foundationBonus)
		}
	}

	// Persist cursor iff any voter drain is queued.
	if len(cursorEntries) > 0 {
		cursor := &epochDrainCursor{
			TargetEra: epochNum,
			Delegates: cursorEntries,
		}
		if err := p.writeEpochDrainCursor(ctx, sm, cursor); err != nil {
			return nil, nil, err
		}
	}

	// Sentinel.
	if err := p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	if err := p.updateAvailableBalance(ctx, sm, debit); err != nil {
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

	_, _, _, _, rewardedCandidates, addrs, amounts, err :=
		p.loadEpochDistributionInputs(ctx, sm, cursor.TargetEra)
	if err != nil {
		return nil, nil, err
	}

	return p.runVoterDistributionChunk(
		ctx, sm, cursor,
		rewardedCandidates, addrs, amounts,
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
// +chunkSize) slice of the frozen work list. Post-C3 the coda shrinks
// to orphan sweep + cursor delete: foundation bonus and epoch sentinel
// already ran in GrantEpochReward.
//
// Post-fork only — GrantVoterRewardChunk is the sole caller. Cursor
// entries are joined against the epoch-scoped rewardedCandidates /
// addrs / amounts via a candidate-address lookup: cursor.Delegates is a
// compacted (opted-in only) subset of the rewarded candidate list, so
// indices are not parallel.
func (p *Protocol) runVoterDistributionChunk(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
	rewardedCandidates []*state.Candidate,
	addrs []address.Address,
	amounts []*big.Int,
	transactionLogs []*action.TransactionLog,
	rewardLogs []*action.Log,
	debit *big.Int,
) ([]*action.TransactionLog, []*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	candByAddr := make(map[string]int, len(rewardedCandidates))
	for i, c := range rewardedCandidates {
		if c != nil {
			candByAddr[c.Address] = i
		}
	}

	chunkSize := p.epochDrainChunkSize(ctx)
	startIdx := cursor.DelegateIndex
	endIdx := uint32(len(cursor.Delegates))
	if chunkSize > 0 && startIdx+chunkSize < endIdx {
		endIdx = startIdx + chunkSize
	}
	for i := startIdx; i < endIdx; i++ {
		work := cursor.Delegates[i]
		voterAmt := safeBig(work.VoterAmountFrozen)
		if voterAmt.Sign() == 0 {
			continue
		}
		candAddr, err := address.FromBytes(work.CandidateIdentifier)
		if err != nil {
			return nil, nil, errors.Wrap(err, "rewarding: decode cursor candidate identifier")
		}
		idx, ok := candByAddr[candAddr.String()]
		if !ok {
			// Delegate fell off the reward split between Phase A and now.
			// Leave the pool balance alone — the coda's orphan sweep
			// routes it to the delegate's live reward address or refunds.
			continue
		}
		cand := rewardedCandidates[idx]
		rewardAddr := addrs[idx]
		if cand == nil || rewardAddr == nil {
			continue
		}
		epochCommission, _, err := p.splitDelegateEpochReward(ctx, sm, cand, amounts[idx])
		if err != nil {
			return nil, nil, err
		}
		iip59Logs, _, err := p.distributeVoterOnly(
			ctx, sm, cand, rewardAddr, voterAmt, epochCommission,
			blkCtx.BlockHeight, actionCtx.ActionHash,
		)
		if err != nil {
			return nil, nil, err
		}
		rewardLogs = append(rewardLogs, iip59Logs...)
		if err := p.decrementPendingBlockRewardPool(ctx, sm, work.CandidateIdentifier, voterAmt); err != nil {
			return nil, nil, err
		}
	}

	if endIdx < uint32(len(cursor.Delegates)) {
		// Drain still in progress. Persist the cursor and let
		// CreatePostSystemActions emit the continuation on the next block.
		cursor.DelegateIndex = endIdx
		if err := p.writeEpochDrainCursor(ctx, sm, cursor); err != nil {
			return nil, nil, err
		}
		if err := p.updateAvailableBalance(ctx, sm, debit); err != nil {
			return nil, nil, err
		}
		return transactionLogs, rewardLogs, nil
	}

	// Coda: orphan drain + cursor delete. Sentinel and foundation bonus
	// ran in Phase A. Delegates that were in the cursor got drained above;
	// any residual pool entries belong to delegates that opted out
	// mid-epoch or fell off the poll list — orphan sweep resolves them.
	visitedPoolIDs := make(map[string]bool, len(cursor.Delegates))
	for _, d := range cursor.Delegates {
		visitedPoolIDs[string(d.CandidateIdentifier)] = true
	}
	orphanLogs, err := p.drainPendingBlockRewardOrphans(
		ctx, sm, visitedPoolIDs, blkCtx.BlockHeight, actionCtx.ActionHash,
	)
	if err != nil {
		return nil, nil, err
	}
	rewardLogs = append(rewardLogs, orphanLogs...)

	if err := p.updateAvailableBalance(ctx, sm, debit); err != nil {
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

// voterBudgetPerBlock returns the maximum number of voters to pay out per
// block during the IIP-59 era-boundary drain. Zero means unbounded — a
// single delegate's full voter list executes in one block regardless of
// size. This is the behavior before the fork gate opens and whenever
// VoterBudgetPerBlock is left at 0. When non-zero, one delegate's voter
// list may span multiple continuation blocks; the cursor's VoterIndex
// records the resume position mid-delegate.
func (p *Protocol) voterBudgetPerBlock(ctx context.Context) uint32 {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return 0
	}
	return uint32(p.cfg.VoterBudgetPerBlock)
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
