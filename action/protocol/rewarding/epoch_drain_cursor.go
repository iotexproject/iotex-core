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

// epochDrainDelegateWork is one per-delegate work item frozen at an era
// boundary. Continuation chunks read it rather than re-deriving from live
// state, which would drift: the PendingBlockRewardPool entry keeps accruing
// behind the drain when a new epoch closes mid-drain.
//
// VoterAmountFrozen is the voter portion only. Commission is already paid — at
// block time block-side, at Phase A epoch-side — and is retained here so the
// continuation logs can attest to it.
type epochDrainDelegateWork struct {
	CandidateIdentifier []byte
	VoterAmountFrozen   *big.Int
	RewardAddress       []byte
	EpochCommission     *big.Int
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
	// SnapshotHash is the era snapshot's digest, stamped into every
	// DelegateVoterRewardsDistributed log this settlement emits for the delegate so an
	// off-chain consumer can group the per-block partial logs back together.
	SnapshotHash []byte
	// FreezeHeight is the block height the era's state was frozen at. It is
	// carried per delegate rather than derived at drain time because the
	// weight recompute is height-sensitive in a way the copy-on-write layer
	// cannot paper over: a contract bucket that is not timestamp-based has its
	// duration converted from blocks with blocksToDurationFn at the height the
	// weight is evaluated at, so evaluating the same frozen bucket at the
	// continuation block's height yields a different weight than the boundary
	// block would have. Every recompute must be handed this number explicitly.
	//
	// Zero means "no frozen era" — a record written before this field existed,
	// which can only be a pre-activation artifact. The recompute must refuse to
	// run against such a record rather than default to the current height.
	FreezeHeight uint64
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

// hasFrozenEra reports whether this work item carries the era metadata the
// per-voter weight recompute needs. False for records written before the
// fields existed; there is nothing left to pay such a record from, so the
// drain skips it and its pending pool rolls into a later era.
func (d epochDrainDelegateWork) hasFrozenEra() bool { return d.FreezeHeight > 0 }

// eraFreezeHeightMismatch reports the first work item whose freeze height is not
// h, and whether one exists.
//
// Phase A materialises the whole work list from a single window, so in a healthy
// cursor every non-zero FreezeHeight is the same number. The caller compares
// that number against the window the drain is about to read through; this walks
// the whole list rather than sampling the first entry so a cursor that somehow
// carries two eras is caught as well as one that has outlived its window.
//
// Items with a zero FreezeHeight are ignored: they carry no era at all and are
// already dropped by drainPayablePrefilter.
func (c *epochDrainCursor) eraFreezeHeightMismatch(h uint64) (uint64, bool) {
	for i := range c.Delegates {
		if d := c.Delegates[i]; d.hasFrozenEra() && d.FreezeHeight != h {
			return d.FreezeHeight, true
		}
	}
	return 0, false
}

// epochDrainCursor checkpoints an IIP-59 era-boundary drain of
// PendingBlockRewardPool balances into voter accounts.
//
// The drain walks the voter key space rather than the delegate list. The space
// is split into 256 shards by the first byte of the voter address; StartShard
// is where the rotation begins, ShardsDone counts the shards finished so far
// (256 means done), and ResumeVoter is the last address visited inside the
// shard currently in progress. Distributed is the per-delegate running payout
// total, positionally aligned with Delegates, and is what the payout clamp
// measures against. Completed cursors remain available for voter queries until
// the next era boundary; only incomplete cursors emit continuation actions.
type epochDrainCursor struct {
	TargetEra             uint64
	StartEpoch            uint64
	EndEpoch              uint64
	StartShard            uint8
	ShardsDone            uint16
	ResumeVoter           []byte
	SettlementSeed        []byte
	Completed             bool
	CompletedHeight       uint64
	Delegates             []epochDrainDelegateWork
	Distributed           []*big.Int
	SkippedDelegateBitmap []byte
}

// totalShards is the number of key-space shards a drain visits. Aliased from
// staking so the two cannot drift: the shard id is the first byte of an
// address, and it is staking that owns the key layout being sharded.
const totalShards = uint16(staking.AddressShards)

// currentShard is the shard the next voter will come from. Meaningless once the
// drain is over, which is what shardsRemaining is for.
func (c *epochDrainCursor) currentShard() byte {
	return byte((uint16(c.StartShard) + c.ShardsDone) % totalShards)
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
	StartEpoch     uint64
	EndEpoch       uint64
	SettlementSeed []byte
	StartShard     uint8
	Delegates      []epochDrainDelegateWork
}

// epochDrainProgress is the compact checkpoint rewritten by continuation
// blocks.
type epochDrainProgress struct {
	TargetEra             uint64
	ShardsDone            uint16
	ResumeVoter           []byte
	Distributed           []*big.Int
	Completed             bool
	CompletedHeight       uint64
	SkippedDelegateBitmap []byte
}

// Serialize marshals the cursor to its proto wire form.
func (c epochDrainCursor) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainCursor{
		TargetEra:       c.TargetEra,
		SettlementSeed:  c.SettlementSeed,
		Completed:       c.Completed,
		CompletedHeight: c.CompletedHeight,
		StartEpoch:      c.StartEpoch,
		EndEpoch:        c.EndEpoch,
		StartShard:      uint32(c.StartShard),
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
	c.SettlementSeed = append(c.SettlementSeed[:0], m.GetSettlementSeed()...)
	c.Completed = m.GetCompleted()
	c.CompletedHeight = m.GetCompletedHeight()
	c.StartEpoch = m.GetStartEpoch()
	c.EndEpoch = m.GetEndEpoch()
	shard, err := decodeShardCount(m.GetStartShard(), "start shard")
	if err != nil {
		return err
	}
	c.StartShard = uint8(shard)
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

// decodeShardCount rejects a shard id that cannot address a shard. The wire
// type is uint32 because 256 does not fit a uint8, so an out-of-range value is
// representable and has to be refused rather than truncated.
func decodeShardCount(v uint32, what string) (uint16, error) {
	if v >= uint32(totalShards) {
		return 0, errors.Errorf("rewarding: %s %d out of range", what, v)
	}
	return uint16(v), nil
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
		RewardAddress:       d.RewardAddress,
		EpochCommission:     epochDrainBigIntBytes(d.EpochCommission),
		TotalWeight:         epochDrainBigIntBytes(d.TotalWeight),
		SnapshotHash:        d.SnapshotHash,
		FreezeHeight:        d.FreezeHeight,
		SelfStakeBucketIdx:  d.SelfStakeBucketIdx,
	}
}

func epochDrainDelegateWorkFromProto(d *rewardingpb.EpochDrainDelegateWork) epochDrainDelegateWork {
	// A record with no freeze height predates both new fields, so its zero
	// self-stake index is an absence and not "bucket 0". Normalizing it to the
	// sentinel here keeps the ambiguity from reaching the recompute, which
	// would otherwise silently treat someone else's bucket 0 as self-stake.
	selfStakeIdx := d.GetSelfStakeBucketIdx()
	if d.GetFreezeHeight() == 0 {
		selfStakeIdx = noSelfStakeBucketIndex
	}
	return epochDrainDelegateWork{
		FreezeHeight:        d.GetFreezeHeight(),
		SelfStakeBucketIdx:  selfStakeIdx,
		CandidateIdentifier: d.GetCandidateIdentifier(),
		VoterAmountFrozen:   epochDrainBytesBigInt(d.GetVoterAmountFrozen()),
		RewardAddress:       d.GetRewardAddress(),
		EpochCommission:     epochDrainBytesBigInt(d.GetEpochCommission()),
		TotalWeight:         epochDrainBytesBigInt(d.GetTotalWeight()),
		SnapshotHash:        d.GetSnapshotHash(),
	}
}

func (p epochDrainPlan) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainPlan{
		TargetEra:      p.TargetEra,
		SettlementSeed: p.SettlementSeed,
		StartEpoch:     p.StartEpoch,
		EndEpoch:       p.EndEpoch,
		StartShard:     uint32(p.StartShard),
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
	p.SettlementSeed = append(p.SettlementSeed[:0], m.GetSettlementSeed()...)
	p.StartEpoch = m.GetStartEpoch()
	p.EndEpoch = m.GetEndEpoch()
	shard, err := decodeShardCount(m.GetStartShard(), "start shard")
	if err != nil {
		return err
	}
	p.StartShard = uint8(shard)
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
		TargetEra:             p.TargetEra,
		ShardsDone:            uint32(p.ShardsDone),
		ResumeVoter:           p.ResumeVoter,
		Completed:             p.Completed,
		CompletedHeight:       p.CompletedHeight,
		SkippedDelegateBitmap: p.SkippedDelegateBitmap,
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
	p.TargetEra = m.GetTargetEra()
	var err error
	if p.ShardsDone, err = decodeShardsDone(m.GetShardsDone()); err != nil {
		return err
	}
	p.ResumeVoter = append(p.ResumeVoter[:0], m.GetResumeVoter()...)
	p.Completed = m.GetCompleted()
	p.CompletedHeight = m.GetCompletedHeight()
	p.SkippedDelegateBitmap = append(p.SkippedDelegateBitmap[:0], m.GetSkippedDelegateBitmap()...)
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
	return &epochDrainPlan{
		TargetEra:      c.TargetEra,
		StartEpoch:     c.StartEpoch,
		EndEpoch:       c.EndEpoch,
		SettlementSeed: c.SettlementSeed,
		StartShard:     c.StartShard,
		Delegates:      c.Delegates,
	}
}

func epochDrainProgressFromCursor(c *epochDrainCursor) *epochDrainProgress {
	distributed := make([]*big.Int, len(c.Delegates))
	for i := range distributed {
		distributed[i] = new(big.Int).Set(c.distributedAt(i))
	}
	return &epochDrainProgress{
		TargetEra:             c.TargetEra,
		ShardsDone:            c.ShardsDone,
		ResumeVoter:           c.ResumeVoter,
		Distributed:           distributed,
		Completed:             c.Completed,
		CompletedHeight:       c.CompletedHeight,
		SkippedDelegateBitmap: c.SkippedDelegateBitmap,
	}
}

func epochDrainCursorFromState(plan *epochDrainPlan, progress *epochDrainProgress) (*epochDrainCursor, error) {
	if plan.TargetEra != progress.TargetEra {
		return nil, errors.Errorf(
			"rewarding: epoch drain plan era %d does not match progress era %d",
			plan.TargetEra, progress.TargetEra,
		)
	}
	if len(progress.Distributed) > len(plan.Delegates) {
		return nil, errors.Errorf(
			"rewarding: epoch drain progress has %d distributed totals for %d delegates",
			len(progress.Distributed), len(plan.Delegates),
		)
	}
	c := &epochDrainCursor{
		TargetEra:             plan.TargetEra,
		StartEpoch:            plan.StartEpoch,
		EndEpoch:              plan.EndEpoch,
		StartShard:            plan.StartShard,
		ShardsDone:            progress.ShardsDone,
		ResumeVoter:           progress.ResumeVoter,
		SettlementSeed:        plan.SettlementSeed,
		Completed:             progress.Completed,
		CompletedHeight:       progress.CompletedHeight,
		Delegates:             plan.Delegates,
		SkippedDelegateBitmap: progress.SkippedDelegateBitmap,
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

func markDelegateSkipped(c *epochDrainCursor, index uint32) {
	byteIndex := int(index / 8)
	if len(c.SkippedDelegateBitmap) <= byteIndex {
		c.SkippedDelegateBitmap = append(c.SkippedDelegateBitmap, make([]byte, byteIndex+1-len(c.SkippedDelegateBitmap))...)
	}
	c.SkippedDelegateBitmap[byteIndex] |= byte(1 << (index % 8))
}

func delegateSkipped(c *epochDrainCursor, index uint32) bool {
	byteIndex := int(index / 8)
	return c != nil && byteIndex < len(c.SkippedDelegateBitmap) &&
		c.SkippedDelegateBitmap[byteIndex]&(byte(1<<(index%8))) != 0
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

func (c *epochDrainCursor) epochRange(epochsPerEra uint64) (uint64, uint64) {
	if c == nil {
		return 0, 0
	}
	endEpoch := c.EndEpoch
	if endEpoch == 0 {
		endEpoch = c.TargetEra
	}
	startEpoch := c.StartEpoch
	if startEpoch == 0 {
		startEpoch = rewardEraStartEpoch(endEpoch, epochsPerEra)
	}
	return startEpoch, endEpoch
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

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *epochDrainCursor) Encode() (systemcontracts.GenericValue, error) {
	data, err := c.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *epochDrainCursor) Decode(v systemcontracts.GenericValue) error {
	return c.Deserialize(v.PrimaryData)
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
			zap.Uint64("startEpoch", c.StartEpoch),
			zap.Uint64("endEpoch", c.EndEpoch),
			zap.Uint16("shardsDone", c.ShardsDone),
			zap.Uint8("currentShard", c.currentShard()),
			// One address, the shard walk's resume point -- not the voter set.
			// The voter set is unbounded and never belongs in a log line.
			zap.String("resumeVoter", hex.EncodeToString(c.ResumeVoter)),
			zap.Int("delegates", len(c.Delegates)),
			zap.Bool("completed", c.Completed),
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
	if err != nil || c == nil || c.Completed {
		return 0, 0, 0, 0, false, err
	}
	return uint32(c.ShardsDone), uint32(len(c.ResumeVoter)), uint32(len(c.Delegates)), c.TargetEra, true, nil
}
