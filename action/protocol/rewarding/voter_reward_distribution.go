// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

const _settlementSeedDomain = "iip59.settlement-start.v2"

// voterRewardDelegateAllocation is one immutable per-delegate allocation input.
type voterRewardDelegateAllocation struct {
	CandidateIdentifier []byte
	VoterAmountFrozen   *big.Int
	// TotalWeight is the frozen value of the candidate's Votes accumulator.
	// It is the denominator every per-voter share divides by, and it is not
	// recomputable: see the payout clamp in computeVoterShares for why a
	// stateless recompute can disagree with it and how distribution handles it.
	//
	// TotalWeight == 0 is what "this delegate has no payable voter set this
	// era" means. There is no longer a separate HasWeightedEntries flag: it
	// used to mean "the frozen entry list held at least one positive weight",
	// and with the list gone that is exactly this field being positive.
	TotalWeight *big.Int
	// SelfStakeBucketIdx is the candidate's self-stake bucket index as of
	// FreezeHeight. It is the only candidate field the per-voter weight
	// recompute reads, and candidate records are mutable during the distribution
	// window, so it is frozen here as a scalar instead of being copied on
	// write. candidateNoSelfStakeBucketIndex (math.MaxUint64) means the
	// candidate had no self-stake bucket at the boundary.
	SelfStakeBucketIdx uint64
}

// noSelfStakeBucketIndex is staking's "candidate has no self-stake bucket"
// sentinel, aliased rather than redeclared so the two cannot drift.
const noSelfStakeBucketIndex = staking.NoSelfStakeBucketIndex

// voterRewardDistributionState composes the immutable allocation plan and
// mutable scan progress used by execution and ReadState. Only its two embedded
// parts are persisted.
//
// Distribution walks the voter address space from a seed-derived start. It first
// scans the tail [start, max], then wraps once and scans the head [min, start).
// ResumeVoter is an exclusive lower bound in the current range.
// DistributedByDelegate is positionally aligned with DelegateAllocations.
// Completed state remains available for voter queries until the next era
// boundary; only incomplete distributions emit continuation actions.
type voterRewardDistributionState struct {
	voterRewardDistributionPlan
	voterRewardDistributionProgress
}

type voterScanPhase uint8

const (
	voterScanTail voterScanPhase = iota
	voterScanHead
	voterScanDone
)

func (c *voterRewardDistributionState) completed() bool { return c.ScanPhase == voterScanDone }

// distributedAt returns the running payout total for one delegate.
func (c *voterRewardDistributionState) distributedAt(i int) *big.Int {
	if i < 0 || i >= len(c.DistributedByDelegate) || c.DistributedByDelegate[i] == nil {
		return new(big.Int)
	}
	return c.DistributedByDelegate[i]
}

// voterRewardDistributionPlan is immutable for the lifetime of a settlement.
// Keeping it separate prevents every continuation block from re-versioning the
// complete allocation list in archive storage.
type voterRewardDistributionPlan struct {
	TargetEra           uint64
	FreezeHeight        uint64
	SettlementSeed      []byte
	DelegateAllocations []voterRewardDelegateAllocation
}

// voterRewardDistributionProgress is the compact checkpoint rewritten by
// continuation blocks.
type voterRewardDistributionProgress struct {
	ScanPhase             voterScanPhase
	ResumeVoter           []byte
	DistributedByDelegate []*big.Int
	CompletedHeight       uint64
}

// Serialize marshals the voter reward distribution state to its proto wire form.
func (c voterRewardDistributionState) Serialize() ([]byte, error) {
	m := &rewardingpb.VoterRewardDistributionState{
		TargetEra:       c.TargetEra,
		FreezeHeight:    c.FreezeHeight,
		SettlementSeed:  c.SettlementSeed,
		Completed:       c.completed(),
		CompletedHeight: c.CompletedHeight,
		StartVoter:      settlementStartVoter(c.SettlementSeed),
		ScanPhase:       uint32(c.ScanPhase),
		ResumeVoter:     c.ResumeVoter,
	}
	if len(c.DelegateAllocations) > 0 {
		m.DelegateAllocations = make([]*rewardingpb.VoterRewardDelegateAllocation, len(c.DelegateAllocations))
		for i, d := range c.DelegateAllocations {
			m.DelegateAllocations[i] = voterRewardDelegateAllocationToProto(d)
			// The running total lives in the progress record, but this combined state is
			// also the ReadState view, and callers there expect to read a
			// delegate's paid-so-far next to its frozen amount.
			m.DelegateAllocations[i].VoterAmountDistributed = voterRewardBigIntBytes(c.distributedAt(i))
		}
	}
	return proto.Marshal(m)
}

// Deserialize populates the voter reward distribution state from its proto wire form.
func (c *voterRewardDistributionState) Deserialize(data []byte) error {
	m := &rewardingpb.VoterRewardDistributionState{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	c.TargetEra = m.GetTargetEra()
	c.FreezeHeight = m.GetFreezeHeight()
	c.SettlementSeed = append(c.SettlementSeed[:0], m.GetSettlementSeed()...)
	c.CompletedHeight = m.GetCompletedHeight()
	var err error
	if c.ScanPhase, err = decodeVoterScanPhase(m.GetScanPhase()); err != nil {
		return err
	}
	c.ResumeVoter = append(c.ResumeVoter[:0], m.GetResumeVoter()...)
	c.DelegateAllocations = nil
	c.DistributedByDelegate = nil
	if ds := m.GetDelegateAllocations(); len(ds) > 0 {
		c.DelegateAllocations = make([]voterRewardDelegateAllocation, len(ds))
		c.DistributedByDelegate = make([]*big.Int, len(ds))
		for i, d := range ds {
			c.DelegateAllocations[i] = voterRewardDelegateAllocationFromProto(d)
			c.DistributedByDelegate[i] = voterRewardBytesBigInt(d.GetVoterAmountDistributed())
		}
	}
	return nil
}

func decodeVoterScanPhase(v uint32) (voterScanPhase, error) {
	if v > uint32(voterScanDone) {
		return 0, errors.Errorf("rewarding: voter scan phase %d out of range", v)
	}
	return voterScanPhase(v), nil
}

func voterRewardDelegateAllocationToProto(d voterRewardDelegateAllocation) *rewardingpb.VoterRewardDelegateAllocation {
	return &rewardingpb.VoterRewardDelegateAllocation{
		CandidateIdentifier: d.CandidateIdentifier,
		VoterAmountFrozen:   voterRewardBigIntBytes(d.VoterAmountFrozen),
		TotalWeight:         voterRewardBigIntBytes(d.TotalWeight),
		SelfStakeBucketIdx:  d.SelfStakeBucketIdx,
	}
}

func voterRewardDelegateAllocationFromProto(d *rewardingpb.VoterRewardDelegateAllocation) voterRewardDelegateAllocation {
	return voterRewardDelegateAllocation{
		SelfStakeBucketIdx:  d.GetSelfStakeBucketIdx(),
		CandidateIdentifier: d.GetCandidateIdentifier(),
		VoterAmountFrozen:   voterRewardBytesBigInt(d.GetVoterAmountFrozen()),
		TotalWeight:         voterRewardBytesBigInt(d.GetTotalWeight()),
	}
}

func (p voterRewardDistributionPlan) Serialize() ([]byte, error) {
	m := &rewardingpb.VoterRewardDistributionPlan{
		TargetEra:      p.TargetEra,
		FreezeHeight:   p.FreezeHeight,
		SettlementSeed: p.SettlementSeed,
	}
	if len(p.DelegateAllocations) > 0 {
		m.DelegateAllocations = make([]*rewardingpb.VoterRewardDelegateAllocation, len(p.DelegateAllocations))
		for i, d := range p.DelegateAllocations {
			m.DelegateAllocations[i] = voterRewardDelegateAllocationToProto(d)
		}
	}
	return proto.Marshal(m)
}

func (p *voterRewardDistributionPlan) Deserialize(data []byte) error {
	m := &rewardingpb.VoterRewardDistributionPlan{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	p.TargetEra = m.GetTargetEra()
	p.FreezeHeight = m.GetFreezeHeight()
	p.SettlementSeed = append(p.SettlementSeed[:0], m.GetSettlementSeed()...)
	p.DelegateAllocations = nil
	if ds := m.GetDelegateAllocations(); len(ds) > 0 {
		p.DelegateAllocations = make([]voterRewardDelegateAllocation, len(ds))
		for i, d := range ds {
			p.DelegateAllocations[i] = voterRewardDelegateAllocationFromProto(d)
		}
	}
	return nil
}

func (p *voterRewardDistributionPlan) Encode() (systemcontracts.GenericValue, error) {
	data, err := p.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (p *voterRewardDistributionPlan) Decode(v systemcontracts.GenericValue) error {
	return p.Deserialize(v.PrimaryData)
}

func (p voterRewardDistributionProgress) Serialize() ([]byte, error) {
	m := &rewardingpb.VoterRewardDistributionProgress{
		ScanPhase:       uint32(p.ScanPhase),
		ResumeVoter:     p.ResumeVoter,
		CompletedHeight: p.CompletedHeight,
	}
	if len(p.DistributedByDelegate) > 0 {
		m.DistributedByDelegate = make([][]byte, len(p.DistributedByDelegate))
		for i, v := range p.DistributedByDelegate {
			m.DistributedByDelegate[i] = voterRewardBigIntBytes(safeBig(v))
		}
	}
	return proto.Marshal(m)
}

func (p *voterRewardDistributionProgress) Deserialize(data []byte) error {
	m := &rewardingpb.VoterRewardDistributionProgress{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	var err error
	if p.ScanPhase, err = decodeVoterScanPhase(m.GetScanPhase()); err != nil {
		return err
	}
	p.ResumeVoter = append(p.ResumeVoter[:0], m.GetResumeVoter()...)
	p.CompletedHeight = m.GetCompletedHeight()
	p.DistributedByDelegate = nil
	if vs := m.GetDistributedByDelegate(); len(vs) > 0 {
		p.DistributedByDelegate = make([]*big.Int, len(vs))
		for i, v := range vs {
			p.DistributedByDelegate[i] = voterRewardBytesBigInt(v)
		}
	}
	return nil
}

func (p *voterRewardDistributionProgress) Encode() (systemcontracts.GenericValue, error) {
	data, err := p.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (p *voterRewardDistributionProgress) Decode(v systemcontracts.GenericValue) error {
	return p.Deserialize(v.PrimaryData)
}

func voterRewardDistributionPlanFromState(c *voterRewardDistributionState) *voterRewardDistributionPlan {
	plan := c.voterRewardDistributionPlan
	return &plan
}

func voterRewardDistributionProgressFromState(c *voterRewardDistributionState) *voterRewardDistributionProgress {
	distributed := make([]*big.Int, len(c.DelegateAllocations))
	for i := range distributed {
		distributed[i] = new(big.Int).Set(c.distributedAt(i))
	}
	return &voterRewardDistributionProgress{
		ScanPhase:             c.ScanPhase,
		ResumeVoter:           c.ResumeVoter,
		DistributedByDelegate: distributed,
		CompletedHeight:       c.CompletedHeight,
	}
}

func composeVoterRewardDistributionState(plan *voterRewardDistributionPlan, progress *voterRewardDistributionProgress) (*voterRewardDistributionState, error) {
	if len(progress.DistributedByDelegate) > len(plan.DelegateAllocations) {
		return nil, errors.Errorf(
			"rewarding: voter reward progress has %d distributed totals for %d delegate allocations",
			len(progress.DistributedByDelegate), len(plan.DelegateAllocations),
		)
	}
	c := &voterRewardDistributionState{
		voterRewardDistributionPlan:     *plan,
		voterRewardDistributionProgress: *progress,
	}
	// The vector is authoritative and padded, never reconstructed from a
	// position: voter-major distribution leaves most delegates partially paid for
	// most of the settlement, so there is no index from which a per-delegate
	// total could be inferred.
	c.DistributedByDelegate = make([]*big.Int, len(plan.DelegateAllocations))
	for i := range c.DistributedByDelegate {
		if i < len(progress.DistributedByDelegate) && progress.DistributedByDelegate[i] != nil {
			c.DistributedByDelegate[i] = progress.DistributedByDelegate[i]
			continue
		}
		c.DistributedByDelegate[i] = new(big.Int)
	}
	return c, nil
}

// settlementSeed freezes one consensus-visible number for every list offset
// in an era settlement. The parent hash is identical for all validators that
// execute the boundary block; targetEra and the domain prevent cross-use.
func settlementSeed(ctx context.Context, targetEra uint64) hash.Hash256 {
	parent := protocol.MustGetBlockchainCtx(ctx).Tip.Hash
	payload := make([]byte, len(_settlementSeedDomain)+len(parent)+8)
	copy(payload, _settlementSeedDomain)
	offset := len(_settlementSeedDomain)
	copy(payload[offset:], parent[:])
	binary.BigEndian.PutUint64(payload[offset+len(parent):], targetEra)
	return hash.Hash256b(payload)
}

// settlementStartVoter maps the uniformly distributed settlement seed onto the
// 160-bit voter address space. Production seeds are 32 bytes; copying the first
// 20 bytes keeps the mapping transparent to off-chain readers.
func settlementStartVoter(seed []byte) []byte {
	start := make([]byte, 20)
	copy(start, seed)
	return start
}

// voterRewardBigIntBytes returns big-endian bytes, or nil for nil.
func voterRewardBigIntBytes(v *big.Int) []byte {
	if v == nil {
		return nil
	}
	return v.Bytes()
}

// voterRewardBytesBigInt returns a big.Int from big-endian bytes. Zero-
// length input yields a zero-valued big.Int (not nil), so callers can
// compare with .Sign() without a nil check.
func voterRewardBytesBigInt(b []byte) *big.Int {
	out := new(big.Int)
	if len(b) > 0 {
		out.SetBytes(b)
	}
	return out
}

// readVoterRewardDistributionState composes the immutable plan and mutable progress into
// the public distribution-state shape used by execution and ReadState.
func (p *Protocol) readVoterRewardDistributionState(
	ctx context.Context,
	sm protocol.StateReader,
) (*voterRewardDistributionState, error) {
	plan := &voterRewardDistributionPlan{}
	if _, err := p.state(ctx, sm, state.VoterRewardDistributionPlanKey, plan); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			progress := &voterRewardDistributionProgress{}
			if _, progressErr := p.state(ctx, sm, state.VoterRewardDistributionProgressKey, progress); progressErr == nil {
				return nil, errors.New("rewarding: voter reward distribution progress exists without a plan")
			} else if !errors.Is(progressErr, state.ErrStateNotExist) {
				return nil, progressErr
			}
			return nil, nil
		}
		return nil, err
	}
	progress := &voterRewardDistributionProgress{}
	if _, err := p.state(ctx, sm, state.VoterRewardDistributionProgressKey, progress); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, errors.New("rewarding: voter reward distribution plan exists without progress")
		}
		return nil, err
	}
	return composeVoterRewardDistributionState(plan, progress)
}

// writeVoterRewardDistributionState creates or replaces both parts of a settlement. It is
// used when the era-boundary distribution is initialized; continuation blocks must call
// writeVoterRewardDistributionProgress.
func (p *Protocol) writeVoterRewardDistributionState(
	ctx context.Context,
	sm protocol.StateManager,
	c *voterRewardDistributionState,
) error {
	if err := p.putState(ctx, sm, state.VoterRewardDistributionPlanKey, voterRewardDistributionPlanFromState(c)); err != nil {
		return err
	}
	return p.writeVoterRewardDistributionProgress(ctx, sm, c)
}

func (p *Protocol) writeVoterRewardDistributionProgress(
	ctx context.Context,
	sm protocol.StateManager,
	c *voterRewardDistributionState,
) error {
	return p.putState(ctx, sm, state.VoterRewardDistributionProgressKey, voterRewardDistributionProgressFromState(c))
}

// deleteVoterRewardDistributionState removes both state entries. It is idempotent because
// deleteState swallows ErrStateNotExist.
func (p *Protocol) deleteVoterRewardDistributionState(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	if err := p.deleteState(ctx, sm, state.VoterRewardDistributionProgressKey, &voterRewardDistributionProgress{}); err != nil {
		return err
	}
	return p.deleteState(ctx, sm, state.VoterRewardDistributionPlanKey, &voterRewardDistributionPlan{})
}

// reportVoterRewardChunkFailure logs a failed distribution chunk at Error and counts
// it. The chunk's own control flow is unchanged: the block still commits with a
// Failure receipt. What this adds is visibility, because nothing else supplies
// it -- a chunk that fails leaves the distribution state untouched while the chain keeps
// advancing, and the next era boundary calls writeVoterRewardDistributionState, which
// replaces the plan and the progress record together. By then the stalled era's
// payouts are unrecoverable from state. Error level plus a counter is what gives
// an operator the window between the first failure and that overwrite.
//
// The state read is best-effort and only for the log: it runs on the failure
// path, before Handle reverts to its entry snapshot, so it may observe writes
// the failed chunk made and will be rolled back. That is fine for a diagnostic
// and is deliberately not allowed to produce a second error -- an unreadable
// an unreadable state just drops the position fields.
func (p *Protocol) reportVoterRewardChunkFailure(
	ctx context.Context,
	sm protocol.StateReader,
	cause error,
) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	fields := []zap.Field{
		zap.Uint64("height", blkCtx.BlockHeight),
		zap.Error(cause),
	}
	var (
		scanPhase voterScanPhase
		hasCursor bool
	)
	if c, err := p.readVoterRewardDistributionState(ctx, sm); err == nil && c != nil {
		scanPhase, hasCursor = c.ScanPhase, true
		fields = append(fields,
			zap.Uint64("targetEra", c.TargetEra),
			zap.Uint64("freezeHeight", c.FreezeHeight),
			zap.Uint8("scanPhase", uint8(c.ScanPhase)),
			// One address, the global walk's resume point -- not the voter set.
			// The voter set is unbounded and never belongs in a log line.
			zap.String("resumeVoter", hex.EncodeToString(c.ResumeVoter)),
			zap.Int("delegates", len(c.DelegateAllocations)),
			zap.Bool("completed", c.completed()),
		)
	}
	log.L().Error("IIP-59 voter reward chunk failed; distribution state did not advance", fields...)
	noteIIP59DrainChunkFailure(scanPhase, hasCursor)
}

// TestOnlyVoterRewardDistributionProgress returns the live distribution's scan phase, the
// length of its resume-voter checkpoint, the total delegate count, and the
// target era, or zero values with present=false when no distribution is in progress.
// Used by the e2e perf bench to watch distribution advance chunk by chunk.
// Production callers must not depend on this.
func (p *Protocol) TestOnlyVoterRewardDistributionProgress(
	ctx context.Context,
	sm protocol.StateReader,
) (scanPhase uint32, resumeVoterLen uint32, totalDelegates uint32, targetEra uint64, present bool, err error) {
	c, err := p.readVoterRewardDistributionState(ctx, sm)
	if err != nil || c == nil || c.completed() {
		return 0, 0, 0, 0, false, err
	}
	return uint32(c.ScanPhase), uint32(len(c.ResumeVoter)), uint32(len(c.DelegateAllocations)), c.TargetEra, true, nil
}
