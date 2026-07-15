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

// GrantEpochReward grants the epoch reward (token) to all beneficiaries of a
// epoch. Post-IIP-59 the drain may span multiple consecutive blocks: Phase A
// (prelude) freezes the work list into a cursor, Phase B (per-chunk) grants
// the next chunkSize delegates, Phase C (coda) runs orphan sweep + foundation
// bonus and writes the epoch-reward sentinel. A single-block drain (cursor
// never persisted) is preserved when cfg.EpochDrainChunkSize==0 or when the
// fork gate is closed.
func (p *Protocol) GrantEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) ([]*action.TransactionLog, []*action.Log, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	currentEpoch := rp.GetEpochNum(blkCtx.BlockHeight)

	cursor, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	cursorPersisted := cursor != nil

	// Overrun guard: fire only when this block would start a NEW epoch's
	// Phase A (last block of an epoch that differs from the cursor's
	// target). A cursor whose target is behind `currentEpoch` on a
	// non-boundary block is a legitimate cross-epoch continuation and
	// must be allowed to keep advancing.
	if cursor != nil && cursor.TargetEpoch != currentEpoch &&
		blkCtx.BlockHeight == rp.GetEpochLastBlockHeight(currentEpoch) {
		return nil, nil, errors.Errorf(
			"rewarding: prior epoch %d drain incomplete when %d began",
			cursor.TargetEpoch, currentEpoch,
		)
	}

	// The effective epoch for this call: the cursor's target when we're
	// continuing, otherwise the current block's epoch (Phase A).
	epochNum := currentEpoch
	if cursor != nil {
		epochNum = cursor.TargetEpoch
	}

	var (
		transactionLogs []*action.TransactionLog
		rewardLogs      []*action.Log
		delta           = new(big.Int)
	)
	if cursor == nil {
		// Phase A — first block of the drain. The last-block-in-epoch guard
		// is only enforced on the Phase A block; continuation blocks are
		// scheduled by CreatePostSystemActions off cursor presence and are
		// by construction non-boundary.
		if err := p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
			return nil, nil, err
		}
		if err := p.assertLastBlockInEpoch(blkCtx.BlockHeight, epochNum, rp); err != nil {
			return nil, nil, err
		}
		preludeTxLogs, preludeLogs, freshCursor, preludeDelta, err := p.freezeEpochDrainWork(ctx, sm, epochNum)
		if err != nil {
			return nil, nil, err
		}
		transactionLogs = append(transactionLogs, preludeTxLogs...)
		rewardLogs = append(rewardLogs, preludeLogs...)
		delta.Add(delta, preludeDelta)
		cursor = freshCursor
	}

	// Phase B — grant the next chunk of frozen delegates. chunkSize==0 (fork
	// off OR genesis default) collapses to "consume everything remaining".
	chunkLogs, chunkDelta, done, err := p.runEpochDrainChunk(ctx, sm, cursor, p.epochDrainChunkSize(ctx))
	if err != nil {
		return nil, nil, err
	}
	rewardLogs = append(rewardLogs, chunkLogs...)
	delta.Add(delta, chunkDelta)

	if !done {
		// More delegates to process on a later block. Persist cursor and
		// apply the delta we accumulated so unclaimedBalance stays consistent
		// with the grants that already landed. Sentinel is intentionally not
		// written yet — its absence is the "in progress" signal.
		if err := p.writeEpochDrainCursor(ctx, sm, cursor); err != nil {
			return nil, nil, err
		}
		cursorPersisted = true
		if err := p.updateAvailableBalance(ctx, sm, delta); err != nil {
			return nil, nil, err
		}
		return transactionLogs, rewardLogs, nil
	}

	// Phase C — orphan sweep then foundation bonus, both from frozen lists.
	codaLogs, codaDelta, err := p.runEpochDrainCoda(ctx, sm, cursor)
	if err != nil {
		return nil, nil, err
	}
	rewardLogs = append(rewardLogs, codaLogs...)
	delta.Add(delta, codaDelta)

	if err := p.updateAvailableBalance(ctx, sm, delta); err != nil {
		return nil, nil, err
	}
	if err := p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum); err != nil {
		return nil, nil, err
	}
	// Only clear the cursor slot if we actually persisted one — a
	// same-block drain never wrote to disk, and mock-driven tests trip on
	// the unexpected DelState.
	if cursorPersisted {
		if err := p.deleteEpochDrainCursor(ctx, sm); err != nil {
			return nil, nil, err
		}
	}
	return transactionLogs, rewardLogs, nil
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
