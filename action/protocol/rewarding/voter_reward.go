// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const _basisPointsDenom uint64 = 10_000

type delegateRewardRouting struct {
	owner                address.Address
	legacyRewardAddress  address.Address
	onchainRewardEnabled bool
	rewardAddressUpdated bool
	blockCommissionBPs   uint64
	epochCommissionBPs   uint64
	snapshot             *staking.CandidatePollSnapshot
}

func resolveDelegateRewardRouting(
	ctx context.Context,
	sr protocol.StateReader,
	candID address.Address,
) (*delegateRewardRouting, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	live, err := staking.ReadCandidateRewardRouting(sr, candID, g.HermesRewardVaultAddresses)
	if err != nil {
		return nil, err
	}
	routing := &delegateRewardRouting{
		owner:                live.Owner,
		legacyRewardAddress:  live.LegacyRewardAddress,
		onchainRewardEnabled: live.OnchainRewardEnabled,
		rewardAddressUpdated: live.RewardAddressUpdated,
		blockCommissionBPs:   _basisPointsDenom,
		epochCommissionBPs:   _basisPointsDenom,
	}
	snap, err := staking.PollSnapshotFor(sr, candID)
	switch {
	case err == nil:
		routing.snapshot = snap
		routing.onchainRewardEnabled = snap.OnchainRewardEnabled
		if snap.OnchainRewardEnabled {
			routing.blockCommissionBPs = snap.BlockCommissionBasisPoints
			routing.epochCommissionBPs = snap.EpochCommissionBasisPoints
		}
	case errors.Is(err, state.ErrStateNotExist):
	default:
		return nil, err
	}
	return routing, nil
}

// splitDelegateEpochReward computes the epoch-reward commission / voter
// split for a delegate. Before the fork it returns (amount, 0). After the
// fork, only an enabled delegate is split. Missing profile data defaults to
// 100% delegate commission; disabled delegates stay on the legacy claim path.
//
//   - fork off (NoVoterRewardDistribution)
//   - nil candidate or zero amount
//
// Otherwise it returns splitCommission(amount, snap.EpochCommissionBasisPoints).
// Empty voter metadata still routes the amount to the pending pool; the cursor
// defers that pool until a later era has an eligible snapshot.
// The caller uses the voter portion as the epoch contribution to the
// per-delegate voter drain and the commission portion as the immediate
// per-delegate payout at Phase A.
func (p *Protocol) splitDelegateEpochReward(
	ctx context.Context,
	sm protocol.StateReader,
	cand *state.Candidate,
	amount *big.Int,
) (*big.Int, *big.Int, error) {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return safeBig(amount), new(big.Int), nil
	}
	if cand == nil || isNilOrZero(amount) {
		return safeBig(amount), new(big.Int), nil
	}
	if err := assertNonNegativeReward(amount); err != nil {
		return nil, nil, err
	}
	candidateID := candidateIdentifier(cand)
	candID, err := address.FromString(candidateID)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "rewarding: invalid candidate identity %q", candidateID)
	}
	routing, err := resolveDelegateRewardRouting(ctx, sm, candID)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "rewarding: resolve reward routing for %s", candID.String())
	}
	if !routing.onchainRewardEnabled {
		return safeBig(amount), new(big.Int), nil
	}
	commission, voterShare := splitCommission(amount, routing.epochCommissionBPs)
	return commission, voterShare, nil
}

// voterChunkRequest is everything one Phase B call needs about a single
// delegate. The fields split into three groups: who is being paid (cand,
// rewardAddr), what was frozen for them at Phase A (voterAmount through
// hasWeightedEntries — these must not be recomputed from live state, or a
// snapshot change mid-drain would silently repay voters at new weights), and
// where this particular block's window sits in the drain (distributedBefore,
// startVoter, voterBudget).
type voterChunkRequest struct {
	cand       *state.Candidate
	rewardAddr address.Address

	// Frozen at Phase A; identical for every chunk of this delegate's era.
	voterAmount        *big.Int
	totalWeightFrozen  *big.Int
	snapshotHash       hash.Hash256
	voterStartIndex    uint32
	lastWeightedIndex  uint32
	hasWeightedEntries bool
	epochCommission    *big.Int

	// Per-chunk position.
	distributedBefore *big.Int
	startVoter        uint32
	voterBudget       uint32

	blkHeight  uint64
	actionHash hash.Hash256
}

// voterChunkResult is what one Phase B call produced. The zero value is the
// "not routed" answer: no logs, nothing paid, and a caller that should advance
// past this delegate rather than treat it as an error.
type voterChunkResult struct {
	logs            []*action.Log
	transactionLogs []*action.TransactionLog
	routed          bool
	paid            *big.Int
	compounded      *big.Int
	consumedVoters  uint32
	totalVoters     uint32
}

// voterRouting holds the collaborators the payout loop needs to decide
// compound-vs-credit for each voter. Resolved once per chunk because every one
// of them is a state read that would otherwise repeat per voter.
type voterRouting struct {
	stakingProto *staking.Protocol
	bucketReader autodeposit.BucketReader
	csr          staking.CandidateStateReader
	candID       address.Address
}

// voterWindowPayout is the parallel-slice form the DelegateDistributed log
// wants: index j of each slice describes the same voter, the j-th in the
// window. paid and compounded are the sums over the window only.
type voterWindowPayout struct {
	voters            []address.Address
	recipients        []address.Address
	amounts           []*big.Int
	compoundBucketIDs []uint64
	paid              *big.Int
	compounded        *big.Int
}

// distributeVoterOnly is IIP-59 §3.2's Phase B: allocate the delegate's frozen
// voter share across the whole snapshot, then pay only the
// [startVoter, startVoter+voterBudget) window. voterBudget == 0 means no cap.
//
// Allocating over the whole snapshot and paying a window is what makes the
// chunking invisible: every voter's amount is the same no matter where the
// block boundaries fall (TestVoterAllocationIsChunkInvariant).
//
// A zero result with a nil error means the snapshot went missing between
// Phase A and this chunk; the caller advances past the delegate and the orphan
// sweep drains the residual pool. Malformed on-chain data (bridge RPC failure,
// unreadable or ineligible bucket) degrades that one voter to a direct payout
// rather than halting the block. Only wiring errors — nil staking protocol,
// log-encoder failure — hard-fail.
//
// The emitted log's TotalVoterPool covers this chunk only, and TotalCommission
// omits block-side commission (visible in the per-block BLOCK_REWARD logs), so
// off-chain consumers must aggregate by (SnapshotHash, delegate, epoch) to
// recover era-wide totals.
func (p *Protocol) distributeVoterOnly(
	ctx context.Context,
	sm protocol.StateManager,
	req voterChunkRequest,
) (voterChunkResult, error) {
	var none voterChunkResult
	if req.cand == nil || req.rewardAddr == nil {
		return none, nil
	}
	if err := assertNonNegativeReward(req.voterAmount); err != nil {
		return none, err
	}
	distributed := safeBig(req.distributedBefore)
	if err := assertNonNegativeReward(distributed); err != nil {
		return none, errors.Wrap(err, "rewarding: invalid distributed voter amount")
	}
	if distributed.Cmp(safeBig(req.voterAmount)) > 0 {
		return none, errors.New("rewarding: distributed voter amount exceeds frozen pool")
	}
	candidateID := candidateIdentifier(req.cand)
	candID, err := address.FromString(candidateID)
	if err != nil {
		return none, errors.Wrapf(err, "rewarding: invalid candidate identity %q", candidateID)
	}
	snap, err := staking.PollSnapshotFor(sm, candID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return none, nil
		}
		return none, errors.Wrapf(err, "rewarding: read poll snapshot for %s", candID.String())
	}
	if req.snapshotHash != hash.ZeroHash256 && snapshotHashFull(snap) != req.snapshotHash {
		return none, nil
	}
	totalVoters := uint32(len(snap.Entries))
	// endVoter is clamped to the list; a startVoter past the end is a
	// no-op window (0 voters paid), which is a legal "delegate is done"
	// state — the caller advances past this delegate.
	startVoter := req.startVoter
	if startVoter > totalVoters {
		startVoter = totalVoters
	}
	endVoter := totalVoters
	if req.voterBudget > 0 && startVoter+req.voterBudget < endVoter {
		endVoter = startVoter + req.voterBudget
	}

	routing, err := p.resolveVoterRouting(ctx, sm, candID)
	if err != nil {
		return none, err
	}

	alloc := newVoterAllocator(
		snap, safeBig(req.voterAmount), req.totalWeightFrozen,
		req.voterStartIndex, req.lastWeightedIndex, req.hasWeightedEntries,
	)
	payout, err := p.allocateAndRouteVoters(ctx, sm, alloc, routing, distributed, startVoter, endVoter)
	if err != nil {
		return none, err
	}
	transactionLogs := make([]*action.TransactionLog, 0, len(payout.voters))
	for i, voter := range payout.voters {
		if voter == nil || payout.amounts[i] == nil || payout.amounts[i].Sign() == 0 || payout.compoundBucketIDs[i] != 0 {
			continue
		}
		transactionLogs = append(transactionLogs, &action.TransactionLog{
			Type:      iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND,
			Sender:    address.RewardingPoolAddr,
			Recipient: payout.recipients[i].String(),
			Amount:    new(big.Int).Set(payout.amounts[i]),
		})
	}

	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(req.blkHeight)
	// SnapshotHash covers the full frozen weight list, so it's stable
	// across chunks — off-chain consumers assemble partial logs by
	// (SnapshotHash, delegate, epoch).
	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:             epochNum,
		Delegate:          candID,
		RewardAddr:        req.rewardAddr,
		TotalCommission:   safeBig(req.epochCommission),
		TotalVoterPool:    safeBig(payout.paid),
		SnapshotHash:      req.snapshotHash,
		Voters:            payout.voters,
		Recipients:        payout.recipients,
		Amounts:           payout.amounts,
		CompoundBucketIDs: payout.compoundBucketIDs,
	})
	if err != nil {
		return none, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return voterChunkResult{
		logs: []*action.Log{{
			Address:     p.addr.String(),
			Topics:      topics,
			Data:        data,
			BlockHeight: req.blkHeight,
			ActionHash:  req.actionHash,
		}},
		transactionLogs: transactionLogs,
		routed:          true,
		paid:            payout.paid,
		compounded:      payout.compounded,
		consumedVoters:  endVoter - startVoter,
		totalVoters:     totalVoters,
	}, nil
}

// resolveVoterRouting gathers the per-chunk collaborators for compound routing.
// A nil autoDepositBridge (the common case on a node that has not configured
// one) leaves bucketReader and csr nil, which the payout loop reads as
// "everyone gets credited directly".
func (p *Protocol) resolveVoterRouting(
	ctx context.Context,
	sm protocol.StateManager,
	candID address.Address,
) (voterRouting, error) {
	var none voterRouting
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return none, errors.New("rewarding: staking protocol not registered")
	}
	routing := voterRouting{stakingProto: stakingProto, candID: candID}
	if p.autoDepositBridge == nil {
		return routing, nil
	}
	slotReader, err := evm.NewSlotReader(ctx, sm)
	if err != nil {
		return none, errors.Wrap(err, "rewarding: build slot reader for autodeposit")
	}
	routing.bucketReader, err = p.resolveAutoDepositBucketReader(slotReader)
	if err != nil {
		return none, errors.Wrap(err, "rewarding: resolve autodeposit bucket reader")
	}
	if routing.bucketReader != nil {
		routing.csr, err = staking.ConstructBaseView(sm)
		if err != nil {
			return none, errors.Wrap(err, "rewarding: construct base view for compound routing")
		}
	}
	return routing, nil
}

// snapshotHashFull computes the deterministic SnapshotHash over the full
// frozen (voter, weight) list. Unlike a per-chunk hash, this value is
// stable across chunks and lets off-chain consumers assemble partial
// DelegateDistributed logs by (SnapshotHash, delegate, epoch).
func snapshotHashFull(snap *staking.CandidatePollSnapshot) hash.Hash256 {
	if snap == nil {
		return hash.ZeroHash256
	}
	if snap.SnapshotHash != hash.ZeroHash256 {
		return snap.SnapshotHash
	}
	voters := make([]address.Address, len(snap.Entries))
	weights := make([]*big.Int, len(snap.Entries))
	for i, e := range snap.Entries {
		voters[i] = e.Voter
		if e.Weight != nil {
			weights[i] = new(big.Int).Set(e.Weight)
		} else {
			weights[i] = new(big.Int)
		}
	}
	return distributedlog.SnapshotHash(voters, weights)
}

// voterDistributionMetadata computes the allocation metadata once when Phase A
// initializes the cursor. Continuation chunks consume the stored values and do
// not rescan all preceding voter weights.
func voterDistributionMetadata(
	snap *staking.CandidatePollSnapshot,
	voterStartIndex uint32,
) (*big.Int, hash.Hash256, uint32, bool) {
	totalWeight := new(big.Int)
	var lastWeightedIndex uint32
	var hasWeightedEntries bool
	if snap == nil || len(snap.Entries) == 0 {
		return totalWeight, hash.ZeroHash256, lastWeightedIndex, hasWeightedEntries
	}
	totalVoters := uint32(len(snap.Entries))
	voterStartIndex %= totalVoters
	for logicalIndex := uint32(0); logicalIndex < totalVoters; logicalIndex++ {
		entry := snap.Entries[rotatedIndex(voterStartIndex, logicalIndex, totalVoters)]
		if entry.Weight == nil || entry.Weight.Sign() <= 0 {
			continue
		}
		totalWeight.Add(totalWeight, entry.Weight)
		lastWeightedIndex = logicalIndex
		hasWeightedEntries = true
	}
	return totalWeight, snapshotHashFull(snap), lastWeightedIndex, hasWeightedEntries
}

// allocateAndRouteVoters pays the [startVoter, endVoter) window of the payout
// order and applies compound-vs-credit routing to each share. The amounts come
// from alloc, which owns the share rule for the delegate's whole frozen list;
// distributedBefore is the running total from earlier chunks, which is what
// makes the amounts identical no matter where the window boundaries fall
// (TestVoterAllocationIsChunkInvariant).
//
// All fallback branches (nil bridge, bridge RPC error, bucket ineligible)
// degrade the affected voter to credit rather than halting the block.
func (p *Protocol) allocateAndRouteVoters(
	ctx context.Context,
	sm protocol.StateManager,
	alloc *voterAllocator,
	routing voterRouting,
	distributedBefore *big.Int,
	startVoter uint32,
	endVoter uint32,
) (voterWindowPayout, error) {
	stop := startIIP59Duration("allocate_and_route")
	routeDurations := iip59RouteDurations{}
	defer routeDurations.observe()
	defer stop()

	var none voterWindowPayout

	// Clamp the payout window to the frozen list.
	total := alloc.count()
	if startVoter > total {
		startVoter = total
	}
	if endVoter > total {
		endVoter = total
	}
	if endVoter < startVoter {
		endVoter = startVoter
	}
	winLen := int(endVoter - startVoter)

	voters := make([]address.Address, winLen)
	recipients := make([]address.Address, winLen)
	amounts := make([]*big.Int, winLen)
	compoundBucketIDs := make([]uint64, winLen)
	paid := new(big.Int)
	compounded := new(big.Int)
	distributed := new(big.Int).Set(distributedBefore)
	directVoters := 0
	compoundVoters := 0
	autoDepositLookups := 0
	nativeBucketReads := 0
	destinationLookups := 0

	for j := 0; j < winLen; j++ {
		logicalIndex := startVoter + uint32(j)
		e := alloc.entryAt(logicalIndex)
		voters[j] = e.Voter
		recipients[j] = e.Voter
		share, err := alloc.shareAt(logicalIndex, distributed)
		if err != nil {
			return none, err
		}
		amounts[j] = share
		distributed.Add(distributed, share)
		if share.Sign() == 0 {
			continue
		}
		if e.Voter == nil {
			// Malformed snapshot entry — should not happen. There is no
			// address to credit, so refuse rather than silently drop
			// the share.
			return none, errors.Errorf(
				"rewarding: nil voter address at logical index %d (snapshot index %d)",
				logicalIndex, alloc.physicalIndex(logicalIndex),
			)
		}
		compoundBucketID := uint64(0)
		if routing.bucketReader != nil {
			stop := startIIP59Accumulation(&routeDurations.autoDepositLookup)
			bucketID, present, lookupErr := routing.bucketReader.LookupBucket(e.Voter)
			stop()
			autoDepositLookups++
			if lookupErr != nil {
				log.L().Warn("autodeposit bucket lookup failed; routing voter share to credit",
					zap.String("delegate", routing.candID.String()),
					zap.String("voter", e.Voter.String()),
					zap.Error(lookupErr))
			} else if present {
				stop := startIIP59Accumulation(&routeDurations.nativeBucketRead)
				bucket, bErr := routing.csr.NativeBucket(bucketID)
				stop()
				nativeBucketReads++
				if bErr != nil {
					log.L().Warn("bucket read for compound routing failed; routing voter share to credit",
						zap.String("delegate", routing.candID.String()),
						zap.String("voter", e.Voter.String()),
						zap.Uint64("bucket", bucketID),
						zap.Error(bErr))
				} else if autodeposit.IsBucketEligibleForCompound(bucket, e.Voter) {
					stop := startIIP59Accumulation(&routeDurations.compoundDeposit)
					if err := routing.stakingProto.AddDepositForCompound(ctx, sm, e.Voter, bucketID, share); err != nil {
						return none, errors.Wrapf(err,
							"rewarding: compound deposit failed for voter %s bucket %d",
							e.Voter.String(), bucketID)
					}
					stop()
					compoundBucketID = bucketID
					compounded.Add(compounded, share)
					compoundVoters++
				}
			}
		}
		if compoundBucketID == 0 {
			stopDestinationLookup := startIIP59Accumulation(&routeDurations.destinationLookup)
			recipient, _, _, err := p.resolveVoterRewardDestination(ctx, sm, e.Voter)
			stopDestinationLookup()
			destinationLookups++
			if err != nil {
				return none, errors.Wrapf(err,
					"rewarding: resolve reward destination for voter %s", e.Voter.String())
			}
			recipients[j] = recipient
			stopDirectCredit := startIIP59Accumulation(&routeDurations.directCredit)
			if err := creditPrimaryAccount(ctx, sm, recipient, share); err != nil {
				return none, errors.Wrapf(err,
					"rewarding: credit voter %s recipient %s failed", e.Voter.String(), recipient.String())
			}
			stopDirectCredit()
			directVoters++
		}
		compoundBucketIDs[j] = compoundBucketID
		paid.Add(paid, share)
	}
	addIIP59Items("chunk_voter", winLen)
	addIIP59Items("direct_voter", directVoters)
	addIIP59Items("compound_voter", compoundVoters)
	addIIP59Items("auto_deposit_lookup", autoDepositLookups)
	addIIP59Items("native_bucket_read", nativeBucketReads)
	addIIP59Items("reward_destination_lookup", destinationLookups)
	return voterWindowPayout{
		voters:            voters,
		recipients:        recipients,
		amounts:           amounts,
		compoundBucketIDs: compoundBucketIDs,
		paid:              paid,
		compounded:        compounded,
	}, nil
}

// splitCommission returns (commission, voterPool) for a totalReward given
// a basis-points rate. Integer division truncates in favour of the voter
// pool: commission = totalReward * bps / 10_000. Rates above 100% are
// clamped to 100% so a malformed on-chain rate cannot over-pay commission.
func splitCommission(totalReward *big.Int, bps uint64) (*big.Int, *big.Int) {
	if totalReward == nil || totalReward.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}
	if bps == 0 {
		return new(big.Int), new(big.Int).Set(totalReward)
	}
	if bps >= _basisPointsDenom {
		return new(big.Int).Set(totalReward), new(big.Int)
	}
	commission := new(big.Int).Mul(totalReward, new(big.Int).SetUint64(bps))
	commission.Div(commission, new(big.Int).SetUint64(_basisPointsDenom))
	voterPool := new(big.Int).Sub(totalReward, commission)
	return commission, voterPool
}

// assertNonNegativeReward rejects a negative reward amount. Nil is treated
// as zero so callers can opt one of the two streams out of a Phase B call.
func assertNonNegativeReward(v *big.Int) error {
	if v == nil {
		return nil
	}
	if v.Sign() < 0 {
		return errors.New("rewarding: invalid total reward for voter distribution")
	}
	return nil
}

// safeBig returns v when non-nil, otherwise a fresh zero big.Int. Keeps
// nil-tolerance out of the arithmetic sites in the main body.
func safeBig(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return v
}

// isNilOrZero returns true when v is nil or zero.
func isNilOrZero(v *big.Int) bool {
	return v == nil || v.Sign() == 0
}

// resolveAutoDepositBucketReader returns the per-drain BucketReader used
// against the AutoDeposit contract. Tests may inject a factory via
// WithAutoDepositBucketReader; production falls through to
// Bridge.NewSlotBucketReader which reads (registrants, buckets) directly
// from contract storage (see autodeposit/slot_reader.go).
func (p *Protocol) resolveAutoDepositBucketReader(slotReader autodeposit.SlotReader) (autodeposit.BucketReader, error) {
	if p.autoDepositBucketReaderFactory != nil {
		return p.autoDepositBucketReaderFactory(slotReader), nil
	}
	return p.autoDepositBridge.NewSlotBucketReader(slotReader)
}

func creditPrimaryAccount(
	ctx context.Context,
	sm protocol.StateManager,
	addr address.Address,
	amount *big.Int,
) error {
	opts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		opts = append(opts, state.LegacyNonceAccountTypeOption())
	}
	account, err := accountutil.LoadOrCreateAccount(sm, addr, opts...)
	if err != nil {
		return err
	}
	if err := account.AddBalance(amount); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, addr, account)
}
