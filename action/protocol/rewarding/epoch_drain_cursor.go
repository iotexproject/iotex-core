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

const _settlementSeedDomain = "iip59.settlement-start.v1"

// epochDrainDelegateWork is one immutable per-delegate allocation input.
type epochDrainDelegateWork struct {
	CandidateIdentifier []byte
	VoterAmountFrozen   *big.Int
	// TotalWeight is the frozen value of the candidate's Votes accumulator.
	// It is the denominator every per-voter share divides by, and it is not
	// recomputable: see the payout clamp in computeVoterShares for why a
	// stateless recompute can disagree with it and what the drain does about it.
	//
	// TotalWeight == 0 is what "this delegate has no payable voter set this
	// era" means. There is no longer a separate HasWeightedEntries flag: it
	// used to mean "the frozen entry list held at least one positive weight",
	// and with the list gone that is exactly this field being positive.
	TotalWeight *big.Int
	// SelfStakeBucketIdx is the candidate's self-stake bucket index as of
	// FreezeHeight. It is the only candidate field the per-voter weight
	// recompute reads, and candidate records are mutable during the drain
	// window, so it is frozen here as a scalar instead of being copied on
	// write. candidateNoSelfStakeBucketIndex (math.MaxUint64) means the
	// candidate had no self-stake bucket at the boundary.
	SelfStakeBucketIdx uint64
}

// noSelfStakeBucketIndex is staking's "candidate has no self-stake bucket"
// sentinel, aliased rather than redeclared so the two cannot drift.
const noSelfStakeBucketIndex = staking.NoSelfStakeBucketIndex

// epochDrainCursor composes the immutable plan and mutable progress used by
// execution and ReadState. Only its two embedded parts are persisted.
//
// The drain walks the voter key space rather than the delegate list. The space
// is split into 256 shards by the first byte of the voter address; the start
// shard is derived from SettlementSeed, ShardsDone counts finished shards
// (256 means done), and ResumeVoter is the last address visited inside the
// shard currently in progress. Distributed is the per-delegate running payout
// total, positionally aligned with Delegates, and is what the payout clamp
// measures against. Completed cursors remain available for voter queries until
// the next era boundary; only incomplete cursors emit continuation actions.
type epochDrainCursor struct {
	epochDrainPlan
	epochDrainProgress
}

// totalShards is the number of key-space shards a drain visits. Aliased from
// staking so the two cannot drift: the shard id is the first byte of an
// address, and it is staking that owns the key layout being sharded.
const totalShards = uint16(staking.AddressShards)

// currentShard is the shard the next voter will come from. Meaningless once the
// drain is over, which is what shardsRemaining is for.
func (c *epochDrainCursor) currentShard() byte {
	return byte((uint16(settlementStartShard(c.SettlementSeed)) + c.ShardsDone) % totalShards)
}

// drainFinished reports whether every shard has been walked.
func (c *epochDrainCursor) drainFinished() bool { return c.ShardsDone >= totalShards }

// distributedAt returns the running payout total for one delegate.
func (c *epochDrainCursor) distributedAt(i int) *big.Int {
	if i < 0 || i >= len(c.Distributed) || c.Distributed[i] == nil {
		return new(big.Int)
	}
	return c.Distributed[i]
}

// epochDrainPlan is immutable for the lifetime of a settlement. Keeping it
// separate prevents every continuation block from re-versioning the complete
// delegate work list in archive storage.
type epochDrainPlan struct {
	TargetEra      uint64
	FreezeHeight   uint64
	SettlementSeed []byte
	Delegates      []epochDrainDelegateWork
}

// epochDrainProgress is the compact checkpoint rewritten by continuation
// blocks.
type epochDrainProgress struct {
	ShardsDone      uint16
	ResumeVoter     []byte
	Distributed     []*big.Int
	CompletedHeight uint64
}

// Serialize marshals the cursor to its proto wire form.
func (c epochDrainCursor) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainCursor{
		TargetEra:       c.TargetEra,
		FreezeHeight:    c.FreezeHeight,
		SettlementSeed:  c.SettlementSeed,
		Completed:       c.drainFinished(),
		CompletedHeight: c.CompletedHeight,
		StartShard:      uint32(settlementStartShard(c.SettlementSeed)),
		ShardsDone:      uint32(c.ShardsDone),
		ResumeVoter:     c.ResumeVoter,
	}
	if len(c.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(c.Delegates))
		for i, d := range c.Delegates {
			m.Delegates[i] = epochDrainDelegateWorkToProto(d)
			// The running total lives in the progress record, but the cursor is
			// also the ReadState view, and callers there expect to read a
			// delegate's paid-so-far next to its frozen amount.
			m.Delegates[i].VoterAmountDistributed = epochDrainBigIntBytes(c.distributedAt(i))
		}
	}
	return proto.Marshal(m)
}

// Deserialize populates the cursor from its proto wire form.
func (c *epochDrainCursor) Deserialize(data []byte) error {
	m := &rewardingpb.EpochDrainCursor{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	c.TargetEra = m.GetTargetEra()
	c.FreezeHeight = m.GetFreezeHeight()
	c.SettlementSeed = append(c.SettlementSeed[:0], m.GetSettlementSeed()...)
	c.CompletedHeight = m.GetCompletedHeight()
	var err error
	if c.ShardsDone, err = decodeShardsDone(m.GetShardsDone()); err != nil {
		return err
	}
	c.ResumeVoter = append(c.ResumeVoter[:0], m.GetResumeVoter()...)
	c.Delegates = nil
	c.Distributed = nil
	if ds := m.GetDelegates(); len(ds) > 0 {
		c.Delegates = make([]epochDrainDelegateWork, len(ds))
		c.Distributed = make([]*big.Int, len(ds))
		for i, d := range ds {
			c.Delegates[i] = epochDrainDelegateWorkFromProto(d)
			c.Distributed[i] = epochDrainBytesBigInt(d.GetVoterAmountDistributed())
		}
	}
	return nil
}

// decodeShardsDone allows the full count, which is the "drain finished" value.
func decodeShardsDone(v uint32) (uint16, error) {
	if v > uint32(totalShards) {
		return 0, errors.Errorf("rewarding: shards done %d out of range", v)
	}
	return uint16(v), nil
}

func epochDrainDelegateWorkToProto(d epochDrainDelegateWork) *rewardingpb.EpochDrainDelegateWork {
	return &rewardingpb.EpochDrainDelegateWork{
		CandidateIdentifier: d.CandidateIdentifier,
		VoterAmountFrozen:   epochDrainBigIntBytes(d.VoterAmountFrozen),
		TotalWeight:         epochDrainBigIntBytes(d.TotalWeight),
		SelfStakeBucketIdx:  d.SelfStakeBucketIdx,
	}
}

func epochDrainDelegateWorkFromProto(d *rewardingpb.EpochDrainDelegateWork) epochDrainDelegateWork {
	return epochDrainDelegateWork{
		SelfStakeBucketIdx:  d.GetSelfStakeBucketIdx(),
		CandidateIdentifier: d.GetCandidateIdentifier(),
		VoterAmountFrozen:   epochDrainBytesBigInt(d.GetVoterAmountFrozen()),
		TotalWeight:         epochDrainBytesBigInt(d.GetTotalWeight()),
	}
}

func (p epochDrainPlan) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainPlan{
		TargetEra:      p.TargetEra,
		FreezeHeight:   p.FreezeHeight,
		SettlementSeed: p.SettlementSeed,
	}
	if len(p.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(p.Delegates))
		for i, d := range p.Delegates {
			m.Delegates[i] = epochDrainDelegateWorkToProto(d)
		}
	}
	return proto.Marshal(m)
}

func (p *epochDrainPlan) Deserialize(data []byte) error {
	m := &rewardingpb.EpochDrainPlan{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	p.TargetEra = m.GetTargetEra()
	p.FreezeHeight = m.GetFreezeHeight()
	p.SettlementSeed = append(p.SettlementSeed[:0], m.GetSettlementSeed()...)
	p.Delegates = nil
	if ds := m.GetDelegates(); len(ds) > 0 {
		p.Delegates = make([]epochDrainDelegateWork, len(ds))
		for i, d := range ds {
			p.Delegates[i] = epochDrainDelegateWorkFromProto(d)
		}
	}
	return nil
}

func (p *epochDrainPlan) Encode() (systemcontracts.GenericValue, error) {
	data, err := p.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (p *epochDrainPlan) Decode(v systemcontracts.GenericValue) error {
	return p.Deserialize(v.PrimaryData)
}

func (p epochDrainProgress) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainProgress{
		ShardsDone:      uint32(p.ShardsDone),
		ResumeVoter:     p.ResumeVoter,
		CompletedHeight: p.CompletedHeight,
	}
	if len(p.Distributed) > 0 {
		m.VoterDistributed = make([][]byte, len(p.Distributed))
		for i, v := range p.Distributed {
			m.VoterDistributed[i] = epochDrainBigIntBytes(safeBig(v))
		}
	}
	return proto.Marshal(m)
}

func (p *epochDrainProgress) Deserialize(data []byte) error {
	m := &rewardingpb.EpochDrainProgress{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	var err error
	if p.ShardsDone, err = decodeShardsDone(m.GetShardsDone()); err != nil {
		return err
	}
	p.ResumeVoter = append(p.ResumeVoter[:0], m.GetResumeVoter()...)
	p.CompletedHeight = m.GetCompletedHeight()
	p.Distributed = nil
	if vs := m.GetVoterDistributed(); len(vs) > 0 {
		p.Distributed = make([]*big.Int, len(vs))
		for i, v := range vs {
			p.Distributed[i] = epochDrainBytesBigInt(v)
		}
	}
	return nil
}

func (p *epochDrainProgress) Encode() (systemcontracts.GenericValue, error) {
	data, err := p.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (p *epochDrainProgress) Decode(v systemcontracts.GenericValue) error {
	return p.Deserialize(v.PrimaryData)
}

func epochDrainPlanFromCursor(c *epochDrainCursor) *epochDrainPlan {
	plan := c.epochDrainPlan
	return &plan
}

func epochDrainProgressFromCursor(c *epochDrainCursor) *epochDrainProgress {
	distributed := make([]*big.Int, len(c.Delegates))
	for i := range distributed {
		distributed[i] = new(big.Int).Set(c.distributedAt(i))
	}
	return &epochDrainProgress{
		ShardsDone:      c.ShardsDone,
		ResumeVoter:     c.ResumeVoter,
		Distributed:     distributed,
		CompletedHeight: c.CompletedHeight,
	}
}

func epochDrainCursorFromState(plan *epochDrainPlan, progress *epochDrainProgress) (*epochDrainCursor, error) {
	if len(progress.Distributed) > len(plan.Delegates) {
		return nil, errors.Errorf(
			"rewarding: epoch drain progress has %d distributed totals for %d delegates",
			len(progress.Distributed), len(plan.Delegates),
		)
	}
	c := &epochDrainCursor{
		epochDrainPlan:     *plan,
		epochDrainProgress: *progress,
	}
	// The vector is authoritative and padded, never reconstructed from a
	// position: a voter-major drain leaves most delegates partially paid for
	// most of the settlement, so there is no index from which a per-delegate
	// total could be inferred.
	c.Distributed = make([]*big.Int, len(plan.Delegates))
	for i := range c.Distributed {
		if i < len(progress.Distributed) && progress.Distributed[i] != nil {
			c.Distributed[i] = progress.Distributed[i]
			continue
		}
		c.Distributed[i] = new(big.Int)
	}
	return c, nil
}

func rewardEraStartEpoch(endEpoch, epochsPerEra uint64) uint64 {
	if endEpoch == 0 {
		return 0
	}
	if epochsPerEra == 0 || endEpoch < epochsPerEra {
		return 1
	}
	return endEpoch - epochsPerEra + 1
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

// settlementListOffset treats seed as an unsigned big-endian integer and
// maps it into [0, length). Empty lists have no valid offset and return zero.
func settlementListOffset(seed []byte, length int) uint32 {
	if length <= 0 {
		return 0
	}
	n := new(big.Int).SetBytes(seed)
	n.Mod(n, new(big.Int).SetUint64(uint64(length)))
	return uint32(n.Uint64())
}

// settlementStartShard maps the settlement seed onto the 256-shard key space.
// The rotation exists so the shard visited first is not always shard 0, which
// would let an address prefix buy a permanent position at the head or tail of
// every drain.
func settlementStartShard(seed []byte) uint8 {
	return uint8(settlementListOffset(seed, int(totalShards)))
}

// epochDrainBigIntBytes returns big-endian bytes, or nil for nil.
func epochDrainBigIntBytes(v *big.Int) []byte {
	if v == nil {
		return nil
	}
	return v.Bytes()
}

// epochDrainBytesBigInt returns a big.Int from big-endian bytes. Zero-
// length input yields a zero-valued big.Int (not nil), so callers can
// compare with .Sign() without a nil check.
func epochDrainBytesBigInt(b []byte) *big.Int {
	out := new(big.Int)
	if len(b) > 0 {
		out.SetBytes(b)
	}
	return out
}

// readEpochDrainCursor composes the immutable plan and mutable progress into
// the public cursor shape used by execution and ReadState.
func (p *Protocol) readEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateReader,
) (*epochDrainCursor, error) {
	plan := &epochDrainPlan{}
	if _, err := p.state(ctx, sm, state.EpochDrainPlanKey, plan); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			progress := &epochDrainProgress{}
			if _, progressErr := p.state(ctx, sm, state.EpochDrainCursorKey, progress); progressErr == nil {
				return nil, errors.New("rewarding: epoch drain progress exists without a plan")
			} else if !errors.Is(progressErr, state.ErrStateNotExist) {
				return nil, progressErr
			}
			return nil, nil
		}
		return nil, err
	}
	progress := &epochDrainProgress{}
	if _, err := p.state(ctx, sm, state.EpochDrainCursorKey, progress); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, errors.New("rewarding: epoch drain plan exists without progress")
		}
		return nil, err
	}
	return epochDrainCursorFromState(plan, progress)
}

// writeEpochDrainCursor creates or replaces both parts of a settlement. It is
// used at Phase A; continuation blocks must call writeEpochDrainProgress.
func (p *Protocol) writeEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
	c *epochDrainCursor,
) error {
	if err := p.putState(ctx, sm, state.EpochDrainPlanKey, epochDrainPlanFromCursor(c)); err != nil {
		return err
	}
	return p.writeEpochDrainProgress(ctx, sm, c)
}

func (p *Protocol) writeEpochDrainProgress(
	ctx context.Context,
	sm protocol.StateManager,
	c *epochDrainCursor,
) error {
	return p.putState(ctx, sm, state.EpochDrainCursorKey, epochDrainProgressFromCursor(c))
}

// deleteEpochDrainCursor removes both state entries. It is idempotent because
// deleteState swallows ErrStateNotExist.
func (p *Protocol) deleteEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	if err := p.deleteState(ctx, sm, state.EpochDrainCursorKey, &epochDrainProgress{}); err != nil {
		return err
	}
	return p.deleteState(ctx, sm, state.EpochDrainPlanKey, &epochDrainPlan{})
}

// reportVoterRewardChunkFailure logs a failed drain chunk at Error and counts
// it. The chunk's own control flow is unchanged: the block still commits with a
// Failure receipt. What this adds is visibility, because nothing else supplies
// it -- a chunk that fails leaves the cursor untouched while the chain keeps
// advancing, and the next era boundary calls writeEpochDrainCursor, which
// replaces the plan and the progress record together. By then the stalled era's
// payouts are unrecoverable from state. Error level plus a counter is what gives
// an operator the window between the first failure and that overwrite.
//
// The cursor read is best-effort and only for the log: it runs on the failure
// path, before Handle reverts to its entry snapshot, so it may observe writes
// the failed chunk made and will be rolled back. That is fine for a diagnostic
// and is deliberately not allowed to produce a second error -- an unreadable
// cursor just drops the position fields.
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
		shardsDone uint16
		hasCursor  bool
	)
	if c, err := p.readEpochDrainCursor(ctx, sm); err == nil && c != nil {
		shardsDone, hasCursor = c.ShardsDone, true
		fields = append(fields,
			zap.Uint64("targetEra", c.TargetEra),
			zap.Uint64("freezeHeight", c.FreezeHeight),
			zap.Uint16("shardsDone", c.ShardsDone),
			zap.Uint8("currentShard", c.currentShard()),
			// One address, the shard walk's resume point -- not the voter set.
			// The voter set is unbounded and never belongs in a log line.
			zap.String("resumeVoter", hex.EncodeToString(c.ResumeVoter)),
			zap.Int("delegates", len(c.Delegates)),
			zap.Bool("completed", c.drainFinished()),
		)
	}
	log.L().Error("IIP-59 voter reward chunk failed; drain cursor did not advance", fields...)
	noteIIP59DrainChunkFailure(shardsDone, hasCursor)
}

// TestOnlyEpochDrainSnapshot returns the live cursor's shards-done count, the
// length of its resume-voter checkpoint, the total delegate count, and the
// target era, or zero values with present=false when no drain is in progress.
// Used by the e2e perf bench to watch the drain advance chunk by chunk.
// Production callers must not depend on this.
func (p *Protocol) TestOnlyEpochDrainSnapshot(
	ctx context.Context,
	sm protocol.StateReader,
) (shardsDone uint32, resumeVoterLen uint32, totalDelegates uint32, targetEra uint64, present bool, err error) {
	c, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil || c == nil || c.drainFinished() {
		return 0, 0, 0, 0, false, err
	}
	return uint32(c.ShardsDone), uint32(len(c.ResumeVoter)), uint32(len(c.Delegates)), c.TargetEra, true, nil
}
