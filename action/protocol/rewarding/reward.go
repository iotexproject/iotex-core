// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"fmt"
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
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
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
) (*action.Log, []*action.TransactionLog, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)

	restoreV1History, err := p.suspendV1BlockRewardHistory(ctx, sm, blkCtx.BlockHeight)
	if err != nil {
		return nil, nil, err
	}
	defer restoreV1History()

	if err := p.assertNoRewardYet(ctx, sm, _blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, nil, err
	}

	payout, err := p.resolveBlockProducerPayout(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	if payout.addr == nil {
		// No reward address — nothing to grant, and no history sentinel
		// either, matching the pre-IIP-59 behaviour.
		return nil, nil, nil
	}

	totalReward, blockReward, effectiveTip, err := p.calculateTotalRewardAndTip(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	if err := p.updateAvailableBalance(ctx, sm, totalReward); err != nil {
		return nil, nil, err
	}

	blockCommission, transactionLogs, err := p.creditBlockProducer(
		ctx, sm, payout, totalReward, blockReward, effectiveTip,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := p.updateRewardHistory(ctx, sm, _blockRewardHistoryKeyPrefix, blkCtx.BlockHeight); err != nil {
		return nil, nil, err
	}

	rewardLog, err := p.encodeBlockRewardLog(ctx, payout, blockCommission, effectiveTip)
	if err != nil {
		return nil, nil, err
	}
	return rewardLog, transactionLogs, nil
}

// suspendV1BlockRewardHistory clears this height's v1 reward-history key from
// the Erigon store for the duration of the grant, and returns the restore to
// defer. During the v1→v2 transition the v1 key must read as "not granted" so
// the v2 path can grant, but it has to be back in place before the block is
// committed. Outside the transition the returned func is a no-op.
func (p *Protocol) suspendV1BlockRewardHistory(
	ctx context.Context,
	sm protocol.StateManager,
	height uint64,
) (func(), error) {
	if !protocol.MustGetFeatureCtx(ctx).UseV2Storage {
		return func() {}, nil
	}
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], height)
	key := append(_blockRewardHistoryKeyPrefix, indexBytes[:]...)
	if err := p.deleteStateV1(
		sm, key, &rewardHistory{}, protocol.ErigonStoreOnlyOption(),
	); err != nil && !errors.Is(err, state.ErrErigonStoreNotSupported) {
		return nil, err
	}
	return func() {
		err := p.putStateV1(sm, key, &rewardHistory{}, protocol.ErigonStoreOnlyOption())
		if err != nil && !errors.Is(err, state.ErrErigonStoreNotSupported) {
			log.L().Panic("failed to put block reward history in Erigon store", zap.Error(err))
		}
	}, nil
}

// blockProducerPayout says where this block's producer reward goes. A nil addr
// means the producer has no reward address and the grant is skipped entirely.
type blockProducerPayout struct {
	addr          address.Address
	candAddr      address.Address // candidate identity; post-fork only
	onchainPool   bool
	commissionBPs uint64
}

// resolveBlockProducerPayout looks up the block producer among the poll
// candidates and decides where its block reward is paid. Pre-fork that is the
// candidate's declared RewardAddress; post-fork it comes from the frozen
// routing, along with the commission rate that splits the reward between the
// producer and its voters.
func (p *Protocol) resolveBlockProducerPayout(
	ctx context.Context,
	sm protocol.StateManager,
) (blockProducerPayout, error) {
	var (
		none            blockProducerPayout
		payout          = blockProducerPayout{commissionBPs: _basisPointsDenom}
		producerAddrStr = protocol.MustGetBlockCtx(ctx).Producer.String()
		legacyReward    string
	)

	var producerCandidate *state.Candidate
	if pp := poll.FindProtocol(protocol.MustGetRegistry(ctx)); pp != nil {
		candidates, err := pp.Candidates(ctx, sm)
		if err != nil {
			return none, err
		}
		for _, candidate := range candidates {
			if candidate.Address == producerAddrStr {
				legacyReward = candidate.RewardAddress
				producerCandidate = candidate
				break
			}
		}
	}

	if !protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		if producerCandidate == nil {
			return none, errors.Errorf("rewarding: producer candidate %s not found", producerAddrStr)
		}
		producerIdentity := candidateIdentifier(producerCandidate)
		candAddr, err := address.FromString(producerIdentity)
		if err != nil {
			return none, errors.Wrapf(err, "rewarding: invalid producer candidate identity %q", producerIdentity)
		}
		routing, err := resolveDelegateRewardRouting(sm, candAddr)
		if err != nil {
			return none, errors.Wrapf(err, "rewarding: resolve reward routing for producer %s", producerIdentity)
		}
		payout.candAddr = candAddr
		payout.addr = routing.PayoutAddress()
		payout.onchainPool = routing.onchainRewardEnabled
		if payout.onchainPool {
			payout.commissionBPs = routing.blockCommissionBPs
		}
	}

	if payout.addr == nil && legacyReward == "" {
		log.S().Debugf("Producer %s doesn't have a reward address", producerAddrStr)
		return payout, nil
	}
	if payout.addr == nil {
		addr, err := address.FromString(legacyReward)
		if err != nil {
			return none, err
		}
		payout.addr = addr
	}
	return payout, nil
}

// creditBlockProducer pays out the block reward and returns the amount to name
// in the BLOCK_REWARD log. Pre-fork the producer takes the whole thing as a
// claimable balance. Post-fork the reward splits by the frozen commission rate:
// the commission and the full priority tip transfer immediately, the voter
// share accrues into the producer's pending pool for the era drain to settle.
func (p *Protocol) creditBlockProducer(
	ctx context.Context,
	sm protocol.StateManager,
	payout blockProducerPayout,
	totalReward *big.Int,
	blockReward *big.Int,
	effectiveTip *big.Int,
) (*big.Int, []*action.TransactionLog, error) {
	transactionLogs := make([]*action.TransactionLog, 0, 2)
	if !payout.onchainPool {
		if err := p.grantToAccount(ctx, sm, payout.addr, totalReward); err != nil {
			return nil, nil, err
		}
		return blockReward, transactionLogs, nil
	}
	commission, voterShare := splitCommission(blockReward, payout.commissionBPs)
	if commission.Sign() > 0 {
		tLog, err := p.creditRewardDirect(ctx, sm, payout.addr, commission)
		if err != nil {
			return nil, nil, err
		}
		transactionLogs = append(transactionLogs, tLog)
	}
	if voterShare.Sign() > 0 {
		if err := p.creditPendingBlockRewardPool(ctx, sm, payout.candAddr.Bytes(), voterShare); err != nil {
			return nil, nil, err
		}
	}
	if effectiveTip.Sign() > 0 {
		tLog, err := p.creditRewardDirect(ctx, sm, payout.addr, effectiveTip)
		if err != nil {
			return nil, nil, err
		}
		transactionLogs = append(transactionLogs, tLog)
	}
	return commission, transactionLogs, nil
}

// encodeBlockRewardLog builds the receipt log for a block grant, in whichever
// of the two wire formats the chain is on. blockCommission is what the producer
// was actually paid now; post-fork the voter share is attested separately by
// the batched DelegateDistributed log at era close. A zero payout emits no log
// at all rather than a log naming zero.
func (p *Protocol) encodeBlockRewardLog(
	ctx context.Context,
	payout blockProducerPayout,
	blockCommission *big.Int,
	effectiveTip *big.Int,
) (*action.Log, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	wrap := func(data []byte) *action.Log {
		return &action.Log{
			Address:     p.addr.String(),
			Topics:      nil,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  protocol.MustGetActionCtx(ctx).ActionHash,
		}
	}

	// Legacy pre-dynamic-fee format: a bare RewardLog.
	if !protocol.MustGetFeatureCtx(ctx).EnableDynamicFeeTx {
		if payout.onchainPool && blockCommission.Sign() == 0 {
			return nil, nil
		}
		data, err := proto.Marshal(&rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   payout.addr.String(),
			Amount: blockCommission.String(),
		})
		if err != nil {
			return nil, err
		}
		return wrap(data), nil
	}

	// Post-dynamic-fee format: RewardLogs wrapper, one entry per stream.
	var rewardLogs []*rewardingpb.RewardLog
	if blockCommission.Sign() > 0 {
		rewardLogs = append(rewardLogs, &rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_BLOCK_REWARD,
			Addr:   payout.addr.String(),
			Amount: blockCommission.String(),
		})
	}
	if !isZero(effectiveTip) {
		rewardLogs = append(rewardLogs, &rewardingpb.RewardLog{
			Type:   rewardingpb.RewardLog_PRIORITY_BONUS,
			Addr:   payout.addr.String(),
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
	return wrap(data), nil
}

// epochGrantResult accumulates what GrantEpochReward's phases produce. The
// phases append into it rather than each returning their own slices, so a
// delegate that pays a commission, credits a voter pool, and then earns a
// foundation bonus lands its three effects in emission order.
type epochGrantResult struct {
	transactionLogs []*action.TransactionLog
	rewardLogs      []*action.Log
	// debit is this block's net delta against fund.unclaimedBalance: slashing
	// returns value (reclaim), grants and pool credits pay out. Block-time
	// voter credits were already debited at GrantBlockReward time.
	debit *big.Int
}

func (r *epochGrantResult) pay(amount *big.Int)     { r.debit = new(big.Int).Add(r.debit, amount) }
func (r *epochGrantResult) reclaim(amount *big.Int) { r.debit = new(big.Int).Sub(r.debit, amount) }

// appendRewardLog records one RewardLog entry against the current block.
func (p *Protocol) appendRewardLog(
	ctx context.Context,
	out *epochGrantResult,
	rewardType rewardingpb.RewardLog_RewardType,
	addr string,
	amount *big.Int,
) error {
	data, err := p.encodeRewardLog(rewardType, addr, amount)
	if err != nil {
		return err
	}
	out.rewardLogs = append(out.rewardLogs, &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: protocol.MustGetBlockCtx(ctx).BlockHeight,
		ActionHash:  protocol.MustGetActionCtx(ctx).ActionHash,
	})
	return nil
}

// payDelegateShare credits a delegate's own share — an epoch commission or a
// foundation bonus — and records the matching reward log. An on-chain delegate
// is paid immediately (the amount leaves the rewarding pool and shows up as a
// transaction log); a legacy delegate accrues a claimable balance instead.
func (p *Protocol) payDelegateShare(
	ctx context.Context,
	sm protocol.StateManager,
	rewardAddr address.Address,
	amount *big.Int,
	onchainReward bool,
	rewardType rewardingpb.RewardLog_RewardType,
	out *epochGrantResult,
) error {
	if onchainReward {
		tLog, err := p.creditRewardDirect(ctx, sm, rewardAddr, amount)
		if err != nil {
			return errors.Wrapf(err, "rewarding: credit %s to owner %s failed",
				rewardType.String(), rewardAddr.String())
		}
		out.transactionLogs = append(out.transactionLogs, tLog)
	} else if err := p.grantToAccount(ctx, sm, rewardAddr, amount); err != nil {
		return errors.Wrapf(err, "rewarding: credit legacy %s to %s failed",
			rewardType.String(), rewardAddr.String())
	}
	if err := p.appendRewardLog(ctx, out, rewardType, rewardAddr.String(), amount); err != nil {
		return err
	}
	out.pay(amount)
	return nil
}

// resolveStaleDrainCursor deals with a previous era's cursor still being live at
// this Phase A boundary. A completed cursor is simply retired; an incomplete
// one is an overrun handed off to handlePhaseAEntryOverrun (IIP-59 §10.2).
func (p *Protocol) resolveStaleDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
) (*action.Log, error) {
	existing, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	if existing.drainFinished() {
		return nil, p.deleteEpochDrainCursor(ctx, sm)
	}
	return p.handlePhaseAEntryOverrun(ctx, sm, existing)
}

// slashUnproductiveDelegates takes back self-stake from delegates that missed
// too many blocks this epoch. The slashed amount flows from the staking bucket
// pool into the rewarding fund, so it offsets — rather than adds to — this
// block's payout.
func (p *Protocol) slashUnproductiveDelegates(
	ctx context.Context,
	sm protocol.StateManager,
	candidates []*state.Candidate,
	uqdMap map[string]uint64,
	blockReward *big.Int,
	out *epochGrantResult,
) error {
	slashAmount, slashLogs, err := p.slashUqd(
		ctx, sm,
		protocol.MustGetBlockCtx(ctx).BlockHeight,
		protocol.MustGetActionCtx(ctx).ActionHash,
		candidates, blockReward, uqdMap,
	)
	if err != nil {
		return err
	}
	if slashAmount.Sign() > 0 {
		out.transactionLogs = append(out.transactionLogs, &action.TransactionLog{
			Type:      iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND,
			Amount:    slashAmount,
			Sender:    address.StakingBucketPoolAddr,
			Recipient: address.RewardingPoolAddr,
		})
	}
	out.rewardLogs = append(out.rewardLogs, slashLogs...)
	out.reclaim(slashAmount)
	return nil
}

// epochCommissionInputs is the per-epoch candidate partition the commission
// phase walks: rewardedCandidates[i] earns amounts[i], payable at addrs[i] on
// the pre-fork path.
type epochCommissionInputs struct {
	rewardedCandidates []*state.Candidate
	addrs              []address.Address
	amounts            []*big.Int
}

// distributeEpochCommissions splits each rewarded delegate's epoch amount into
// commission (paid now) and voter share (accrued into the delegate's pending
// pool). Era-boundary planning is a separate scan over all pending pools so a
// candidate does not disappear from settlement merely because it left the
// current poll list.
func (p *Protocol) distributeEpochCommissions(
	ctx context.Context,
	sm protocol.StateManager,
	in epochCommissionInputs,
	out *epochGrantResult,
) error {
	postFork := !protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution
	for i, cand := range in.rewardedCandidates {
		if cand == nil {
			continue
		}
		rewardAddr := in.addrs[i]
		onchainReward := false
		if postFork {
			candAddr, err := address.FromString(candidateIdentifier(cand))
			if err != nil {
				return err
			}
			routing, err := resolveDelegateRewardRouting(sm, candAddr)
			if err != nil {
				return err
			}
			rewardAddr = routing.PayoutAddress()
			onchainReward = routing.onchainRewardEnabled
		}
		if rewardAddr == nil {
			continue
		}
		epochAmt := in.amounts[i]
		if epochAmt == nil {
			epochAmt = new(big.Int)
		}
		commission, voterShare, err := p.splitDelegateEpochReward(ctx, sm, cand, epochAmt)
		if err != nil {
			return err
		}
		if commission.Sign() > 0 {
			if err := p.payDelegateShare(
				ctx, sm, rewardAddr, commission, onchainReward,
				rewardingpb.RewardLog_EPOCH_REWARD, out,
			); err != nil {
				return err
			}
		}
		if !onchainReward {
			continue
		}
		if voterShare.Sign() == 0 {
			continue
		}
		candID, err := candidateIdentifierBytes(candidateIdentifier(cand))
		if err != nil {
			return err
		}
		if err := p.creditPendingBlockRewardPool(ctx, sm, candID, voterShare); err != nil {
			return err
		}
		out.pay(voterShare)
	}
	return nil
}

// freezePendingPoolDrainWork builds the canonical work list from every nonzero
// pending pool. Pools without a fresh positive-weight snapshot stay untouched
// and are retried at a later boundary.
func (p *Protocol) freezePendingPoolDrainWork(
	ctx context.Context,
	sm protocol.StateManager,
	eraFreezeHeight uint64,
) ([]epochDrainDelegateWork, error) {
	ids, err := p.listPendingBlockRewardPoolIDs(ctx, sm)
	if err != nil {
		return nil, err
	}
	work := make([]epochDrainDelegateWork, 0, len(ids))
	for _, candID := range ids {
		amount, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		if err != nil {
			return nil, err
		}
		if amount.Sign() <= 0 {
			continue
		}
		item, payable, err := p.freezeDelegateDrainWork(sm, candID, amount, eraFreezeHeight)
		if err != nil {
			return nil, err
		}
		if payable {
			work = append(work, item)
		}
	}
	return work, nil
}

func (p *Protocol) freezeDelegateDrainWork(
	sm protocol.StateManager,
	candID []byte,
	totalVoter *big.Int,
	eraFreezeHeight uint64,
) (epochDrainDelegateWork, bool, error) {
	var none epochDrainDelegateWork
	candAddr, err := address.FromBytes(candID)
	if err != nil {
		return none, false, err
	}
	stop := startIIP59Duration("cursor_snapshot")
	snapshot, err := staking.PollSnapshotFor(sm, candAddr)
	stop()
	if err != nil && !errors.Is(err, state.ErrStateNotExist) {
		return none, false, err
	}
	// STALENESS GUARD.
	//
	// The snapshot lives under a single per-candidate key with no era qualifier,
	// so a candidate frozen in an earlier era and skipped at this era's freeze
	// still reads back — carrying the PREVIOUS era's FreezeHeight H'. That
	// height then travels into the work item and reaches
	// staking.FrozenVoterWeight, while the buckets it weighs come from THIS
	// era's copy-on-write window (opened at H). The numerator's bucket
	// membership is as of H and the denominator, TotalWeight, is the
	// candidate's Votes as of H'. Nothing fails; the drain just pays on a mixed
	// basis, bounded only by the payout clamp.
	//
	// Degrading to "absent" collapses that into the path the freezer already
	// produces for a candidate it never froze: TotalWeight 0, FreezeHeight 0,
	// which the allocator reads as "no payable voter set this era" and skips,
	// leaving the pending pool intact for a later era to settle.
	//
	// Not an error, deliberately. Which era last froze a candidate is committed
	// chain state — every validator reads the same answer and degrades the same
	// item in the same block — so this is the class that must never halt a
	// block. (The converse rule, that node-local capability faults must never
	// degrade, is why the window read above returns its error instead.)
	//
	// The eraFreezeHeight != 0 condition is not a fallback for a missing
	// oracle: a closed window at an era boundary means this era never froze
	// anything, and the drain refuses to run at all in that state
	// (runVoterDistributionChunk rejects a closed window). Testing freshness
	// against 0 would only mislabel every snapshot as stale.
	if snapshot == nil || eraFreezeHeight == 0 || snapshot.FreezeHeight != eraFreezeHeight ||
		safeBig(snapshot.TotalWeight).Sign() <= 0 {
		if snapshot != nil && snapshot.FreezeHeight != eraFreezeHeight {
			log.L().Warn("stale poll snapshot for delegate; deferring voter pool to a later era",
				zap.String("delegate", candAddr.String()),
				zap.Uint64("snapshotFreezeHeight", snapshot.FreezeHeight),
				zap.Uint64("eraFreezeHeight", eraFreezeHeight))
		}
		return none, false, nil
	}
	return epochDrainDelegateWork{
		CandidateIdentifier: candID,
		VoterAmountFrozen:   new(big.Int).Set(totalVoter),
		TotalWeight:         new(big.Int).Set(snapshot.TotalWeight),
		SelfStakeBucketIdx:  snapshot.SelfStakeBucketIdx,
	}, true, nil
}

// distributeFoundationBonus pays the flat per-delegate bonus to the first
// numDelegatesForFoundationBonus eligible candidates in poll order. Exempt
// candidates and zero-vote candidates (hard probation) are skipped without
// consuming a slot.
func (p *Protocol) distributeFoundationBonus(
	ctx context.Context,
	sm protocol.StateManager,
	a *admin,
	candidates []*state.Candidate,
	exemptAddrs map[string]interface{},
	epochNum uint64,
	out *epochGrantResult,
) error {
	if !a.grantFoundationBonus(epochNum) &&
		!(epochNum >= p.cfg.FoundationBonusP2StartEpoch && epochNum <= p.cfg.FoundationBonusP2EndEpoch) {
		return nil
	}
	postFork := !protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution
	for i, count := 0, uint64(0); i < len(candidates) && count < a.numDelegatesForFoundationBonus; i++ {
		if _, ok := exemptAddrs[candidates[i].Address]; ok {
			continue
		}
		if candidates[i].Votes.Sign() == 0 {
			// hard probation
			continue
		}
		count++
		var (
			rewardAddr    address.Address
			onchainReward bool
		)
		if postFork {
			candAddr, err := address.FromString(candidateIdentifier(candidates[i]))
			if err != nil {
				return err
			}
			routing, err := resolveDelegateRewardRouting(sm, candAddr)
			if err != nil {
				return err
			}
			rewardAddr = routing.PayoutAddress()
			onchainReward = routing.onchainRewardEnabled
		} else {
			if candidates[i].RewardAddress == "" {
				log.S().Warnf("Candidate %s doesn't have a reward address", candidates[i].Address)
				continue
			}
			var err error
			if rewardAddr, err = address.FromString(candidates[i].RewardAddress); err != nil {
				return err
			}
		}
		if err := p.payDelegateShare(
			ctx, sm, rewardAddr, a.foundationBonus, onchainReward,
			rewardingpb.RewardLog_FOUNDATION_BONUS, out,
		); err != nil {
			return err
		}
	}
	return nil
}

// persistDrainCursor writes the era's frozen work list together with the shard
// the voter walk starts at, so a settlement that repeatedly runs long does not
// always serve the same corner of the address space first. No entries means no
// voter drain is queued and no cursor is written — the absence of a cursor is
// what tells later blocks there is nothing to continue.
//
// A zero-work era boundary still has to seal the era copy-on-write window.
// PutPollResult opened that window roughly 1.5 epochs ago, unconditionally, and
// the only other seal on the normal path is completeEpochDrain — which never
// runs when no cursor was written. An era with no opted-in delegate, with every
// delegate on 100% commission, or with an empty pool would therefore leave the
// window open until the *next* boundary's Begin, and every bucket write in
// between would pay the copy-on-write cost for a snapshot nobody will read.
//
// The same hole opens on a boundary DECLINED because the LSD owner-index
// backfill has not finished. The freeze already ran ~1.5 epochs earlier — it
// keys on the epoch arithmetic alone and knows nothing about the backfill — so
// a window exists for an era that will produce no cursor and no drain. Hence
// the caller passes isEraBoundaryEpoch here rather than the narrower
// isEraBoundary: this parameter answers "was a window opened for this era",
// which is a question about the freeze, not about whether the drain runs.
//
// The seal is placed here, and gated on the boundary epoch, because this runs
// inside GrantEpochReward — a system action in the epoch's last block, executed
// identically by every node. It is consensus-visible state, not a local
// heuristic: a seal that fired on the proposer and not on a validator would
// fork.
//
// It is deliberately NOT hoisted out of the len(entries)==0 branch: when a
// cursor was written the drain still needs the window, and completeEpochDrain
// seals it at the end. staking.SealEraCOWWindow is idempotent and a no-op when
// no window is open, so a boundary that never opened one costs nothing.
func (p *Protocol) persistDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
	epochNum uint64,
	settlementSeed hash.Hash256,
	freezeHeight uint64,
	entries []epochDrainDelegateWork,
) error {
	if len(entries) == 0 {
		if err := staking.SealEraCOWWindow(ctx, sm); err != nil {
			return errors.Wrap(err, "rewarding: seal era copy-on-write window for a zero-work era")
		}
		return nil
	}
	stop := startIIP59Duration("cursor_write_phase_a")
	defer stop()
	distributed := make([]*big.Int, len(entries))
	for i := range distributed {
		distributed[i] = new(big.Int)
	}
	if err := p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra:      epochNum,
		FreezeHeight:   freezeHeight,
		SettlementSeed: append([]byte(nil), settlementSeed[:]...),
		Delegates:      entries,
		Distributed:    distributed,
	}); err != nil {
		return err
	}
	addIIP59Items("cursor_delegate", len(entries))
	return nil
}

// GrantEpochReward runs the epoch-last-block work as a single body for
// both pre- and post-fork chains:
//
//  1. Pre-A checks: epoch-last and no prior sentinel; hand off any overrun cursor.
//  2. Load inputs: admin config, exempt set, uqd map, candidates, split partition.
//  3. Slashing (its own feature flag; independent of IIP-59).
//  4. Per-delegate epoch split loop — for each rewarded candidate:
//     - splitDelegateEpochReward returns (commission, voterShare) — the
//     post-fork path using the frozen profile rate, or full owner commission
//     when the profile/snapshot is absent; pre-fork returns (amount, 0).
//     - pay on-chain commission directly to owner, or accrue a legacy claim.
//     - if voterShare > 0, credit into the delegate's pending pool.
//  5. Foundation bonus.
//  6. At an era boundary, scan every pending pool and build work from pools
//     with a fresh positive-weight snapshot. Persist a cursor iff work exists;
//     otherwise seal the era copy-on-write window.
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
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	g := genesis.MustExtractGenesisContext(ctx)
	// isEraBoundary is the epoch-arithmetic gate, and it is deliberately
	// the *identical* expression to the one in freezeIIP59PollSnapshot
	// (poll/util.go). That matters: the freeze is what opens this era's
	// copy-on-write window, ~1.5 epochs before this block runs, so this
	// predicate is exactly "a window was opened for this era" and is what the
	// window's lifecycle has to be keyed on.
	isEraBoundary := !featureCtx.NoVoterRewardDistribution &&
		protocol.IsEraBoundary(epochNum, g.EpochsPerRewardEra)
	// isEraBoundary gates the IIP-59 voter cursor lifecycle: cursor
	// materialization, overrun handoff, and cursor write only fire on
	// era-boundary epochs (see IIP-59 §10.2). Commission payment,
	// slashing, and foundation bonus run every epoch regardless.
	//
	// There is no second condition. The LSD owner index this drains is built
	// in full in the single block at g.ToBeEnabledBlockHeight, from staking's
	// CreatePreStates (staking.backfillOwnerIndex) -- i.e. before any action of
	// that block runs, and so before the earliest block at which a freeze could
	// open an era window. There is no window in which the index is partial and
	// a boundary would have to be declined.
	var eraSettlementSeed hash.Hash256
	if isEraBoundary {
		eraSettlementSeed = settlementSeed(ctx, epochNum)
	}
	if !featureCtx.NoVoterRewardDistribution {
		stop := startIIP59Duration("epoch_reward_total")
		defer stop()
	}
	if isEraBoundary {
		stop := startIIP59Duration("cursor_init")
		defer stop()
	}

	// Pre-A checks.
	if err := p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	if err := p.assertLastBlockInEpoch(blkCtx.BlockHeight, epochNum, rp); err != nil {
		return nil, nil, err
	}
	// The EPOCH_DRAIN_OVERRUN entry is emitted ahead of any per-delegate
	// EPOCH_REWARD entry so external verifiers see the handoff first.
	var overrunLog *action.Log
	if isEraBoundary {
		var err error
		overrunLog, err = p.resolveStaleDrainCursor(ctx, sm)
		if err != nil {
			return nil, nil, err
		}
	}

	a, exemptAddrs, uqdMap, candidates, rewardedCandidates, addrs, amounts, err :=
		p.loadEpochDistributionInputs(ctx, sm, epochNum)
	if err != nil {
		return nil, nil, err
	}

	out := &epochGrantResult{
		transactionLogs: make([]*action.TransactionLog, 0),
		rewardLogs:      make([]*action.Log, 0),
		debit:           big.NewInt(0),
	}
	if overrunLog != nil {
		out.rewardLogs = append(out.rewardLogs, overrunLog)
	}

	// Slashing (gated by NotSlashUnproductiveDelegates, independent of IIP-59).
	if !featureCtx.NotSlashUnproductiveDelegates {
		if err := p.slashUnproductiveDelegates(ctx, sm, candidates, uqdMap, a.blockReward, out); err != nil {
			return nil, nil, err
		}
	}

	if err := p.distributeEpochCommissions(ctx, sm, epochCommissionInputs{
		rewardedCandidates: rewardedCandidates,
		addrs:              addrs,
		amounts:            amounts,
	}, out); err != nil {
		return nil, nil, err
	}
	if err := p.distributeFoundationBonus(ctx, sm, a, candidates, exemptAddrs, epochNum, out); err != nil {
		return nil, nil, err
	}
	if isEraBoundary {
		window, err := staking.EraCOWWindow(sm)
		if err != nil {
			return nil, nil, errors.Wrap(err, "rewarding: read era copy-on-write window for drain plan")
		}
		if !window.Open() {
			return nil, nil, errors.New("rewarding: era copy-on-write window is closed at era boundary")
		}
		entries, err := p.freezePendingPoolDrainWork(ctx, sm, window.FreezeHeight)
		if err != nil {
			return nil, nil, err
		}
		if err := p.persistDrainCursor(
			ctx, sm, epochNum, eraSettlementSeed, window.FreezeHeight, entries,
		); err != nil {
			return nil, nil, err
		}
	}

	// Order matters, and not for any reason local to this function: the two
	// writes below go to different keys with nothing reading between them, but
	// the delta state digest hashes the write queue *in order*. Emitting the
	// sentinel first changes the digest of every epoch-boundary block, at every
	// height, including ones long predating the IIP-59 fork -- so a node
	// replaying history rejects the first epoch boundary it reaches, and a live
	// node forks off at the next one. Keep the balance update ahead of the
	// sentinel, matching what mainnet committed.
	if err := p.updateAvailableBalance(ctx, sm, out.debit); err != nil {
		return nil, nil, err
	}
	// Sentinel.
	if err := p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	return out.transactionLogs, out.rewardLogs, nil
}

// GrantVoterRewardChunk advances one chunk of an in-progress IIP-59
// era-boundary drain. Emitted by CreatePostSystemActions on every
// non-epoch-boundary block while a cursor is incomplete; the final chunk
// seals the COW window and records cursor completion inline. Foundation bonus
// and the epoch sentinel are committed by GrantEpochReward in Phase A.
//
// Epoch-scoped allocation inputs are frozen in the plan by Phase A. A
// continuation does not re-run candidate selection or slashing against its
// current epoch, which may only contain a few blocks.
func (p *Protocol) GrantVoterRewardChunk(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.TransactionLog, []*action.Log, error) {
	stop := startIIP59Duration("voter_reward_chunk_total")
	defer stop()

	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		// Defense in depth: CreatePostSystemActions never emits this
		// action pre-fork, and Validate rejects manually crafted ones.
		return nil, nil, settleableVoterChunkError(
			"rewarding: voter reward chunk action requires IIP-59 fork")
	}

	cursor, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	if cursor == nil {
		// Dispatcher invariant: cursor must be live when this handler
		// runs. Reaching here means CreatePostSystemActions or a state
		// migration got out of sync. Settleable: cursor presence is committed
		// state, so every node reaches this verdict together.
		return nil, nil, settleableVoterChunkError(
			"rewarding: voter reward chunk dispatched without a live cursor")
	}
	if cursor.drainFinished() {
		return nil, nil, settleableVoterChunkError(
			"rewarding: voter reward chunk dispatched for a completed cursor")
	}

	// IIP-59 §10.3: emit a CURSOR_PROGRESS snapshot of the pre-drain
	// cursor. Off-chain monitors read this stream to detect pile-up
	// without inspecting protocol state. Purely informational — never
	// affects payout.
	progressLog, err := p.encodeCursorProgressLog(ctx, cursor)
	if err != nil {
		return nil, nil, err
	}

	transactionLogs, rewardLogs, err := p.runVoterDistributionChunk(ctx, sm, cursor)
	return transactionLogs, append([]*action.Log{progressLog}, rewardLogs...), err
}

// loadEpochDistributionInputs derives the deterministic epoch-scoped state
// Phase A needs: admin config, exempt set, uqdMap, poll candidates, and the
// splitEpochReward partition (rewardedCandidates, addrs, amounts).
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
		featureCtx.NoVoterRewardDistribution,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	return a, exemptAddrs, uqdMap, candidates, rewardedCandidates, addrs, amounts, nil
}

// runVoterDistributionChunk advances the voter-major drain by one block.
//
// The drain walks the voter key space rather than the frozen delegate list. The
// space is split into 256 shards by the first byte of the voter address; each
// block resumes at cursor's current shard and, within it, just past
// ResumeVoter, and pays at most VoterBudgetPerBlock voters (0 disables the
// cap). A voter is paid once for everything they are owed across every delegate
// they staked with, native and liquid-staking alike.
//
// Why voter-major: the candidate-major walk paid a voter once per delegate,
// which meant one destination lookup and one balance write per (voter,
// delegate) pair, and it drove the walk from a frozen per-candidate entry list
// that had to be materialized and stored at the era boundary. The shard walk
// needs neither -- it recomputes each voter's weight on demand from the era's
// frozen buckets.
//
// Post-fork only -- GrantVoterRewardChunk is the sole caller.
func (p *Protocol) runVoterDistributionChunk(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
) ([]*action.TransactionLog, []*action.Log, error) {
	stop := startIIP59Duration("chunk_distribution")
	defer stop()

	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	transactionLogs := make([]*action.TransactionLog, 0)
	rewardLogs := make([]*action.Log, 0)

	// The liquid-staking owner index is one of the two streams the shard walk
	// merges. Without it the walk still completes and still reports success --
	// it just pays every contract staker nothing. A silent underpayment of real
	// money is worse than a halted block, so refuse to run at all.
	//
	// The backfill that populates the index has no persisted completion state to
	// assert against, so this checks availability only. If a completion marker
	// is added later it belongs here.
	if !contractstaking.OwnerIndexEnabled(ctx) {
		return nil, nil, errors.New(
			"rewarding: contract-staking owner index unavailable; refusing to drain native-only")
	}
	window, err := staking.EraCOWWindow(sm)
	if err != nil {
		return nil, nil, errors.Wrap(err, "rewarding: read era copy-on-write window")
	}
	if !window.Open() {
		// Every bucket the recompute reads must be the era's frozen copy. A
		// closed window means the drain would read live buckets and pay weights
		// the era never froze.
		return nil, nil, errors.New("rewarding: era copy-on-write window is closed during drain")
	}
	// Open is not enough: it must be *this* era's window. The next era's freeze
	// rides PutPollResult, which fires around the midpoint of the epoch before
	// the boundary epoch -- roughly 1.5 epochs before the boundary block where
	// Phase A would notice the overrun and hand this cursor to
	// handlePhaseAEntryOverrun. eracow.Begin does not refuse to supersede an
	// open window; it queues the old one for collection and installs the new
	// one. So for that 1.5-epoch stretch EraCOWWindow answers at the new freeze
	// height while every work item here still carries the old one, and the two
	// travel together into staking.FrozenVoterWeight.
	//
	// The reads that follow would not fail, they would answer for the wrong
	// era: a bucket that grew since the old H pays at its grown amount, and a
	// bucket minted after the old H becomes payable at all, because the
	// high-water marks moved with the window.
	//
	// Stop rather than pay. Nothing is lost by stopping: the pending pools stay
	// where they are, and Phase A of the incoming era runs
	// handlePhaseAEntryOverrun, which deletes this cursor and rolls every
	// delegate's residue into the era that can freeze it properly. Settleable
	// because both heights are committed state that every node reads
	// identically, so a Failure receipt is a verdict the whole network reaches
	// on the same block.
	if cursor.FreezeHeight == 0 || cursor.FreezeHeight != window.FreezeHeight {
		return nil, nil, settleableVoterChunkError(
			"rewarding: era %d drain frozen at height %d outlived its copy-on-write window, which is now open at height %d",
			cursor.TargetEra, cursor.FreezeHeight, window.FreezeHeight)
	}
	routing, err := p.resolveVoterRouting(ctx, sm)
	if err != nil {
		return nil, nil, err
	}

	ensureDistributed(cursor)
	in := voterShareInputs{
		window:       window,
		staking:      routing.stakingProto,
		delegates:    cursor.Delegates,
		byCandidate:  delegateWorkIndex(cursor.Delegates),
		freezeHeight: cursor.FreezeHeight,
		// Aliased, not copied: the clamp has to see payouts made earlier in this
		// same block, not only those persisted by earlier blocks.
		distributed: cursor.Distributed,
	}

	chunkLogs := make([]delegateChunkLog, len(cursor.Delegates))
	routeDurations := iip59RouteDurations{}
	defer routeDurations.observe()

	budget := p.voterBudgetPerBlock(ctx)
	remaining := budget
	// keyBudget bounds the scan half of the per-block budget. `remaining`
	// alone bounds only the voters this block *pays*; without keyBudget a
	// single shard stuffed by an attacker (the first address byte is
	// grindable) would be read in full before the first voter is paid. See
	// voter_shard_scan_bound.go for why a coverage bound, and not a result
	// count, is what makes the truncated scan safe to resume from.
	keyBudget := 0
	if budget > 0 {
		keyBudget = int(budget) * _voterScanKeyBudgetPerVoter
	}
	compoundedTotal := new(big.Int)
	visited := 0

	for !cursor.drainFinished() {
		if budget > 0 && (remaining == 0 || keyBudget <= 0) {
			break
		}
		shard := cursor.currentShard()
		resumeBefore := cursor.ResumeVoter
		reader := newBoundedShardReader(sm, voterScanLimit(remaining, keyBudget))
		voters, err := staking.FrozenShardVoters(reader, window, shard, cursor.ResumeVoter)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "rewarding: scan voter shard %d", shard)
		}
		keyBudget -= reader.keysScanned()
		coverage, coverageComplete, err := reader.coverage()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "rewarding: bound voter shard %d", shard)
		}
		// A shard is finished only when the scans covered it end to end. A
		// truncated scan leaves the tail unread, so the shard stays current and
		// the resume point moves to the coverage bound instead.
		budgetExhausted := false
		for _, voter := range voters {
			if budget > 0 && remaining == 0 {
				budgetExhausted = true
				break
			}
			if !coverageComplete && bytes.Compare(voter.Bytes(), coverage) > 0 {
				// Beyond the coverage bound: some stream was truncated before
				// this address, so paying it now could skip a voter that
				// stream would have produced below it. `voters` is ascending,
				// so everything after is beyond the bound too.
				break
			}
			shares, err := computeVoterShares(sm, in, voter)
			if err != nil {
				return nil, nil, err
			}
			payout, err := p.payVoterCombined(ctx, sm, routing, in, voter, shares, &routeDurations)
			if err != nil {
				return nil, nil, err
			}
			if payout.amount != nil && payout.amount.Sign() > 0 {
				if err := p.bookVoterPayout(ctx, sm, cursor, payout); err != nil {
					return nil, nil, err
				}
				recordVoterPayout(chunkLogs, payout)
				if tLog := voterTransactionLog(payout); tLog != nil {
					transactionLogs = append(transactionLogs, tLog)
				}
				// payout.compounded, not compoundBucketID != 0: native
				// bucket 0 is a real bucket, and missing it here would
				// under-report the block's rewarding-pool -> bucket-pool
				// outflow while the money had already moved.
				if payout.compounded {
					compoundedTotal.Add(compoundedTotal, payout.amount)
				}
			}
			// ResumeVoter is the last address visited, not the last one paid: a
			// voter whose recomputed weight is zero still has to advance the
			// cursor or every later block rediscovers and re-skips them.
			cursor.ResumeVoter = append([]byte(nil), voter.Bytes()...)
			visited++
			if budget > 0 {
				remaining--
			}
		}
		if budgetExhausted {
			// The voter budget ran out mid-shard. ResumeVoter already points at
			// the last voter paid, and must not be advanced to the coverage
			// bound -- the voters between the two have not been paid.
			break
		}
		if !coverageComplete {
			// The scan, not the payout loop, is what stopped short. Advance past
			// everything proven scanned so the next round -- this block if key
			// budget remains, otherwise the next block -- starts after it.
			// Without this a shard denser than one round would be rescanned from
			// the same offset forever.
			//
			// The coverage bound need not be a real voter address; ResumeVoter is
			// only ever used as an exclusive lower bound within this shard.
			//
			// Assert that it strictly advances. A bounded scan that came back
			// covering no more than the point it resumed from would leave the
			// cursor exactly where it started, and the drain would rescan the
			// same keys on every block for the rest of the era without ever
			// finishing. That is a silent stall, so make it a loud failure.
			if len(resumeBefore) > 0 && bytes.Compare(coverage, resumeBefore) <= 0 {
				return nil, nil, errors.Errorf(
					"rewarding: bounded scan of shard %d made no progress (coverage %x <= resume %x)",
					shard, coverage, resumeBefore)
			}
			cursor.ResumeVoter = append([]byte(nil), coverage...)
			continue
		}
		cursor.ShardsDone++
		cursor.ResumeVoter = nil
	}
	addIIP59Items("chunk_voter", visited)

	if compoundedTotal.Sign() > 0 {
		compoundLog, err := p.settleCompoundOutflow(compoundedTotal)
		if err != nil {
			return nil, nil, err
		}
		transactionLogs = append(transactionLogs, compoundLog)
	}
	for i := range chunkLogs {
		delegateLog, err := p.packDelegateChunkLog(
			cursor.TargetEra, cursor.Delegates[i], chunkLogs[i],
			blkCtx.BlockHeight, actionCtx.ActionHash,
		)
		if err != nil {
			return nil, nil, err
		}
		if delegateLog != nil {
			rewardLogs = append(rewardLogs, delegateLog)
		}
	}

	if !cursor.drainFinished() {
		// Drain still in progress. Persist the cursor and let
		// CreatePostSystemActions emit the continuation on the next block.
		stopWrite := startIIP59Duration("cursor_write_chunk")
		if err := p.writeEpochDrainProgress(ctx, sm, cursor); err != nil {
			return nil, nil, err
		}
		stopWrite()
		return transactionLogs, rewardLogs, nil
	}

	if err := p.completeEpochDrain(ctx, sm, cursor); err != nil {
		return nil, nil, err
	}
	return transactionLogs, rewardLogs, nil
}

// ensureDistributed pads the cursor's per-delegate running totals to the
// delegate count so the clamp can index it without a bounds check.
func ensureDistributed(cursor *epochDrainCursor) {
	if len(cursor.Distributed) == len(cursor.Delegates) {
		for i := range cursor.Distributed {
			if cursor.Distributed[i] == nil {
				cursor.Distributed[i] = new(big.Int)
			}
		}
		return
	}
	padded := make([]*big.Int, len(cursor.Delegates))
	for i := range padded {
		if i < len(cursor.Distributed) && cursor.Distributed[i] != nil {
			padded[i] = cursor.Distributed[i]
			continue
		}
		padded[i] = new(big.Int)
	}
	cursor.Distributed = padded
}

// bookVoterPayout books the funds one voter's combined transfer moved: draw
// each contributing delegate's pending pool down by that delegate's share,
// debit the rewarding fund, and advance the per-delegate running total the
// payout clamp measures against.
func (p *Protocol) bookVoterPayout(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
	payout voterCombinedPayout,
) error {
	for _, share := range payout.shares {
		i := share.delegateIndex
		if i < 0 || i >= len(cursor.Delegates) {
			return errors.Errorf("rewarding: share names delegate %d outside the work list", i)
		}
		if share.share == nil || share.share.Sign() <= 0 {
			continue
		}
		if err := p.decrementPendingBlockRewardPool(
			ctx, sm, cursor.Delegates[i].CandidateIdentifier, share.share,
		); err != nil {
			return err
		}
		if err := p.updateTotalBalance(ctx, sm, share.share); err != nil {
			return errors.Wrap(err, "rewarding: debit voter reward outflow")
		}
		cursor.Distributed[i] = new(big.Int).Add(cursor.Distributed[i], share.share)
	}
	return nil
}

// completeEpochDrain seals the COW window and records completion. Any rounding
// residual remains in its pending pool and is eligible for the next era.
func (p *Protocol) completeEpochDrain(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	// The era's copies exist to serve this drain; once it is done nothing may
	// read them again, so close the window here rather than at the next
	// boundary. Sealing also stops the per-write copy hooks, which is the
	// difference between paying for them for six blocks and paying for them
	// until the next era. The copies themselves are deleted later, in bounded
	// batches — see collectEraCOWGarbage.
	if err := staking.SealEraCOWWindow(ctx, sm); err != nil {
		return err
	}
	cursor.CompletedHeight = blkCtx.BlockHeight
	cursor.ShardsDone = totalShards
	cursor.ResumeVoter = nil
	stop := startIIP59Duration("cursor_write_complete")
	defer stop()
	if err := p.writeEpochDrainProgress(ctx, sm, cursor); err != nil {
		return err
	}
	return nil
}

func (p *Protocol) settleCompoundOutflow(
	amount *big.Int,
) (*action.TransactionLog, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, errors.New("rewarding: compound outflow must be positive")
	}
	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_DEPOSIT_TO_BUCKET,
		Sender:    address.RewardingPoolAddr,
		Recipient: address.StakingBucketPoolAddr,
		Amount:    new(big.Int).Set(amount),
	}, nil
}

// voterBudgetPerBlock returns the maximum number of voters to pay out per
// block during the IIP-59 era-boundary drain. Zero means unbounded — a
// whole era settles in one block regardless of how many voters exist. This is
// the behavior before the fork gate opens and whenever VoterBudgetPerBlock is
// left at 0. When non-zero, a single key-space shard may span multiple
// continuation blocks; the cursor's ResumeVoter records the resume position
// inside that shard.
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

// handlePhaseAEntryOverrun implements the IIP-59 §10.2 graceful degrade for
// the case where a previous era's cursor is still live at Phase A entry.
// It sums the residue (pool balance that would have drained had the era
// completed) across every delegate the stale cursor named, deletes the stale
// cursor, and returns an EPOCH_DRAIN_OVERRUN log describing the handoff.
// The pool entries themselves are left in place — Phase A's own cursor
// materialisation, later in this same call, picks them up as freshly
// frozen work for the new era.
func (p *Protocol) handlePhaseAEntryOverrun(
	ctx context.Context,
	sm protocol.StateManager,
	cursor *epochDrainCursor,
) (*action.Log, error) {
	residue, remaining, err := p.computePhaseAOverrunResidue(ctx, sm, cursor)
	if err != nil {
		return nil, err
	}
	logEntry, err := p.encodeOverrunLog(ctx, cursor.TargetEra, remaining, residue)
	if err != nil {
		return nil, err
	}
	if err := p.deleteEpochDrainCursor(ctx, sm); err != nil {
		return nil, err
	}
	log.L().Warn("IIP-59: prior era drain overran into Phase A; residue rolls into next era",
		zap.Uint64("staleTargetEra", cursor.TargetEra),
		zap.Uint32("staleShardsDone", uint32(cursor.ShardsDone)),
		zap.Uint32("delegatesRemaining", remaining),
		zap.String("residue", residue.String()),
	)
	return logEntry, nil
}

// computePhaseAOverrunResidue sums the live pool balance across the stale
// cursor's delegates and counts how many still hold one.
//
// It sums over every delegate, not over a suffix of the work list: the drain is
// voter-major, so an interrupted settlement leaves most delegates partially
// paid rather than leaving a clean prefix done and a suffix untouched.
// VoterAmountFrozen from the cursor is intentionally NOT used -- the true
// leftover is the live pool balance, which may have accrued additional
// block-time credit between the era-boundary freeze and this Phase A entry.
func (p *Protocol) computePhaseAOverrunResidue(
	ctx context.Context,
	sm protocol.StateReader,
	cursor *epochDrainCursor,
) (*big.Int, uint32, error) {
	residue := new(big.Int)
	remaining := uint32(0)
	for i := range cursor.Delegates {
		bal, err := p.readPendingBlockRewardPool(ctx, sm, cursor.Delegates[i].CandidateIdentifier)
		if err != nil {
			return nil, 0, err
		}
		if bal.Sign() == 0 {
			continue
		}
		remaining++
		residue.Add(residue, bal)
	}
	return residue, remaining, nil
}

// encodeOverrunLog builds an action.Log carrying the EPOCH_DRAIN_OVERRUN
// receipt payload. addr encodes "<target_era>:<delegates_remaining>",
// amount encodes the residue as a decimal string — reusing the existing
// RewardLog wire slots per the enum comment in rewarding.proto.
func (p *Protocol) encodeOverrunLog(
	ctx context.Context,
	targetEra uint64,
	delegatesRemaining uint32,
	residue *big.Int,
) (*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if residue == nil {
		residue = new(big.Int)
	}
	data, err := proto.Marshal(&rewardingpb.RewardLog{
		Type:   rewardingpb.RewardLog_EPOCH_DRAIN_OVERRUN,
		Addr:   fmt.Sprintf("%d:%d", targetEra, delegatesRemaining),
		Amount: residue.String(),
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

// encodeCursorProgressLog builds an action.Log carrying the CURSOR_PROGRESS
// snapshot for the given cursor (pre-drain state). It is a purely
// informational log — off-chain monitors read this stream to detect
// cursor pile-up without inspecting protocol state. See IIP-59 §10.3.
func (p *Protocol) encodeCursorProgressLog(
	ctx context.Context,
	cursor *epochDrainCursor,
) (*action.Log, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	remaining := uint32(0)
	if !cursor.drainFinished() {
		remaining = uint32(totalShards - cursor.ShardsDone)
	}
	data, err := proto.Marshal(&rewardingpb.RewardLog{
		Type: rewardingpb.RewardLog_CURSOR_PROGRESS,
		Addr: fmt.Sprintf("%d:%d:%x:%d",
			cursor.TargetEra, cursor.ShardsDone, cursor.ResumeVoter, remaining),
		Amount: "0",
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

func (p *Protocol) creditRewardDirect(
	ctx context.Context,
	sm protocol.StateManager,
	recipient address.Address,
	amount *big.Int,
) (*action.TransactionLog, error) {
	if err := creditPrimaryAccount(ctx, sm, recipient, amount); err != nil {
		return nil, err
	}
	if err := p.updateTotalBalance(ctx, sm, amount); err != nil {
		return nil, err
	}
	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND,
		Sender:    address.RewardingPoolAddr,
		Recipient: recipient.String(),
		Amount:    new(big.Int).Set(amount),
	}, nil
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
	useLegacyRewardAddress bool,
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
		if useLegacyRewardAddress && candidate.RewardAddress != "" {
			rewardAddr, err = address.FromString(candidate.RewardAddress)
			if err != nil {
				return nil, nil, nil, err
			}
		} else if useLegacyRewardAddress {
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
