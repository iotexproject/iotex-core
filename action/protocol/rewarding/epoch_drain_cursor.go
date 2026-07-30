// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"encoding/binary"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

const _settlementSeedDomain = "iip59.settlement-start.v1"

// epochDrainDelegateWork is one frozen per-delegate work item captured
// at an era boundary: the delegate identifier, voter share, reward
// address, and epoch commission. Continuation chunks read this rather
// than re-deriving the target-era distribution from live state or the live
// PendingBlockRewardPool entry, which may keep accruing behind the
// drain if a new epoch closes mid-drain. The frozen amount is the
// voter portion only — delegate commission is granted immediately at
// block time (block-side) or at Phase A (epoch-side), then retained here
// for continuation log consistency.
type epochDrainDelegateWork struct {
	CandidateIdentifier    []byte
	VoterAmountFrozen      *big.Int
	VoterAmountDistributed *big.Int
	RewardAddress          []byte
	EpochCommission        *big.Int
	TotalWeight            *big.Int
	SnapshotHash           []byte
	LastWeightedIndex      uint32
	HasWeightedEntries     bool
	VoterStartIndex        uint32
}

// epochDrainCursor checkpoints an IIP-59 era-boundary drain of
// PendingBlockRewardPool balances into voter accounts. TargetEra is the
// boundary epoch being drained; DelegateIndex is the resume position in
// the frozen Delegates slice; VoterIndex is the resume position inside
// the delegate at DelegateIndex when a per-block voter cap stops
// payout mid-delegate — 0 whenever the entry at DelegateIndex is
// fresh. Delegates is the frozen work list. Completed cursors remain available
// for voter queries until the next era boundary; only incomplete cursors emit
// continuation actions.
type epochDrainCursor struct {
	TargetEra          uint64
	StartEpoch         uint64
	EndEpoch           uint64
	DelegateIndex      uint32
	VoterIndex         uint32
	SettlementSeed     []byte
	DelegateStartIndex uint32
	Completed          bool
	CompletedHeight    uint64
	Delegates          []epochDrainDelegateWork
	SkippedDelegates   []byte
}

// epochDrainPlan is immutable for the lifetime of a settlement. Keeping it
// separate prevents every continuation block from re-versioning the complete
// delegate work list in archive storage.
type epochDrainPlan struct {
	TargetEra          uint64
	StartEpoch         uint64
	EndEpoch           uint64
	SettlementSeed     []byte
	DelegateStartIndex uint32
	Delegates          []epochDrainDelegateWork
}

// epochDrainProgress is the compact checkpoint rewritten by continuation
// blocks. VoterAmountDistributed belongs to DelegateIndex only.
type epochDrainProgress struct {
	TargetEra              uint64
	DelegateIndex          uint32
	VoterIndex             uint32
	VoterAmountDistributed *big.Int
	Completed              bool
	CompletedHeight        uint64
	SkippedDelegateBitmap  []byte
}

// Serialize marshals the cursor to its proto wire form.
func (c epochDrainCursor) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainCursor{
		TargetEra:          c.TargetEra,
		DelegateIndex:      c.DelegateIndex,
		VoterIndex:         c.VoterIndex,
		SettlementSeed:     c.SettlementSeed,
		DelegateStartIndex: c.DelegateStartIndex,
		Completed:          c.Completed,
		CompletedHeight:    c.CompletedHeight,
		StartEpoch:         c.StartEpoch,
		EndEpoch:           c.EndEpoch,
	}
	if len(c.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(c.Delegates))
		for i, d := range c.Delegates {
			m.Delegates[i] = epochDrainDelegateWorkToProto(d, true)
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
	c.DelegateIndex = m.GetDelegateIndex()
	c.VoterIndex = m.GetVoterIndex()
	c.SettlementSeed = append(c.SettlementSeed[:0], m.GetSettlementSeed()...)
	c.DelegateStartIndex = m.GetDelegateStartIndex()
	c.Completed = m.GetCompleted()
	c.CompletedHeight = m.GetCompletedHeight()
	c.StartEpoch = m.GetStartEpoch()
	c.EndEpoch = m.GetEndEpoch()
	c.Delegates = nil
	if ds := m.GetDelegates(); len(ds) > 0 {
		c.Delegates = make([]epochDrainDelegateWork, len(ds))
		for i, d := range ds {
			c.Delegates[i] = epochDrainDelegateWorkFromProto(d)
		}
	}
	return nil
}

func epochDrainDelegateWorkToProto(d epochDrainDelegateWork, includeProgress bool) *rewardingpb.EpochDrainDelegateWork {
	m := &rewardingpb.EpochDrainDelegateWork{
		CandidateIdentifier: d.CandidateIdentifier,
		VoterAmountFrozen:   epochDrainBigIntBytes(d.VoterAmountFrozen),
		RewardAddress:       d.RewardAddress,
		EpochCommission:     epochDrainBigIntBytes(d.EpochCommission),
		TotalWeight:         epochDrainBigIntBytes(d.TotalWeight),
		SnapshotHash:        d.SnapshotHash,
		LastWeightedIndex:   d.LastWeightedIndex,
		HasWeightedEntries:  d.HasWeightedEntries,
		VoterStartIndex:     d.VoterStartIndex,
	}
	if includeProgress {
		m.VoterAmountDistributed = epochDrainBigIntBytes(d.VoterAmountDistributed)
	}
	return m
}

func epochDrainDelegateWorkFromProto(d *rewardingpb.EpochDrainDelegateWork) epochDrainDelegateWork {
	return epochDrainDelegateWork{
		CandidateIdentifier:    d.GetCandidateIdentifier(),
		VoterAmountFrozen:      epochDrainBytesBigInt(d.GetVoterAmountFrozen()),
		VoterAmountDistributed: epochDrainBytesBigInt(d.GetVoterAmountDistributed()),
		RewardAddress:          d.GetRewardAddress(),
		EpochCommission:        epochDrainBytesBigInt(d.GetEpochCommission()),
		TotalWeight:            epochDrainBytesBigInt(d.GetTotalWeight()),
		SnapshotHash:           d.GetSnapshotHash(),
		LastWeightedIndex:      d.GetLastWeightedIndex(),
		HasWeightedEntries:     d.GetHasWeightedEntries(),
		VoterStartIndex:        d.GetVoterStartIndex(),
	}
}

func (p epochDrainPlan) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainPlan{
		TargetEra:          p.TargetEra,
		SettlementSeed:     p.SettlementSeed,
		DelegateStartIndex: p.DelegateStartIndex,
		StartEpoch:         p.StartEpoch,
		EndEpoch:           p.EndEpoch,
	}
	if len(p.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(p.Delegates))
		for i, d := range p.Delegates {
			m.Delegates[i] = epochDrainDelegateWorkToProto(d, false)
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
	p.DelegateStartIndex = m.GetDelegateStartIndex()
	p.StartEpoch = m.GetStartEpoch()
	p.EndEpoch = m.GetEndEpoch()
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
	return proto.Marshal(&rewardingpb.EpochDrainProgress{
		TargetEra:              p.TargetEra,
		DelegateIndex:          p.DelegateIndex,
		VoterIndex:             p.VoterIndex,
		VoterAmountDistributed: epochDrainBigIntBytes(p.VoterAmountDistributed),
		Completed:              p.Completed,
		CompletedHeight:        p.CompletedHeight,
		SkippedDelegateBitmap:  p.SkippedDelegateBitmap,
	})
}

func (p *epochDrainProgress) Deserialize(data []byte) error {
	m := &rewardingpb.EpochDrainProgress{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	p.TargetEra = m.GetTargetEra()
	p.DelegateIndex = m.GetDelegateIndex()
	p.VoterIndex = m.GetVoterIndex()
	p.VoterAmountDistributed = epochDrainBytesBigInt(m.GetVoterAmountDistributed())
	p.Completed = m.GetCompleted()
	p.CompletedHeight = m.GetCompletedHeight()
	p.SkippedDelegateBitmap = append(p.SkippedDelegateBitmap[:0], m.GetSkippedDelegateBitmap()...)
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
		TargetEra:          c.TargetEra,
		StartEpoch:         c.StartEpoch,
		EndEpoch:           c.EndEpoch,
		SettlementSeed:     c.SettlementSeed,
		DelegateStartIndex: c.DelegateStartIndex,
		Delegates:          c.Delegates,
	}
}

func epochDrainProgressFromCursor(c *epochDrainCursor) *epochDrainProgress {
	distributed := new(big.Int)
	if int(c.DelegateIndex) < len(c.Delegates) {
		distributed.Set(safeBig(c.Delegates[c.DelegateIndex].VoterAmountDistributed))
	}
	return &epochDrainProgress{
		TargetEra:              c.TargetEra,
		DelegateIndex:          c.DelegateIndex,
		VoterIndex:             c.VoterIndex,
		VoterAmountDistributed: distributed,
		Completed:              c.Completed,
		CompletedHeight:        c.CompletedHeight,
		SkippedDelegateBitmap:  c.SkippedDelegates,
	}
}

func epochDrainCursorFromState(plan *epochDrainPlan, progress *epochDrainProgress) (*epochDrainCursor, error) {
	if plan.TargetEra != progress.TargetEra {
		return nil, errors.Errorf(
			"rewarding: epoch drain plan era %d does not match progress era %d",
			plan.TargetEra, progress.TargetEra,
		)
	}
	c := &epochDrainCursor{
		TargetEra:          plan.TargetEra,
		StartEpoch:         plan.StartEpoch,
		EndEpoch:           plan.EndEpoch,
		DelegateIndex:      progress.DelegateIndex,
		VoterIndex:         progress.VoterIndex,
		SettlementSeed:     plan.SettlementSeed,
		DelegateStartIndex: plan.DelegateStartIndex,
		Completed:          progress.Completed,
		CompletedHeight:    progress.CompletedHeight,
		Delegates:          plan.Delegates,
		SkippedDelegates:   progress.SkippedDelegateBitmap,
	}
	for i := range c.Delegates {
		distributed := new(big.Int)
		switch {
		case delegateSkipped(c, uint32(i)):
		case c.Completed || uint32(i) < c.DelegateIndex:
			distributed.Set(safeBig(c.Delegates[i].VoterAmountFrozen))
		case uint32(i) == c.DelegateIndex:
			distributed.Set(safeBig(progress.VoterAmountDistributed))
		}
		c.Delegates[i].VoterAmountDistributed = distributed
	}
	return c, nil
}

func markDelegateSkipped(c *epochDrainCursor, index uint32) {
	byteIndex := int(index / 8)
	if len(c.SkippedDelegates) <= byteIndex {
		c.SkippedDelegates = append(c.SkippedDelegates, make([]byte, byteIndex+1-len(c.SkippedDelegates))...)
	}
	c.SkippedDelegates[byteIndex] |= byte(1 << (index % 8))
}

func delegateSkipped(c *epochDrainCursor, index uint32) bool {
	byteIndex := int(index / 8)
	return c != nil && byteIndex < len(c.SkippedDelegates) &&
		c.SkippedDelegates[byteIndex]&(byte(1<<(index%8))) != 0
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

func rotateDelegateWork(delegates []epochDrainDelegateWork, start uint32) []epochDrainDelegateWork {
	if len(delegates) == 0 {
		return delegates
	}
	start %= uint32(len(delegates))
	if start == 0 {
		return delegates
	}
	rotated := make([]epochDrainDelegateWork, 0, len(delegates))
	rotated = append(rotated, delegates[start:]...)
	rotated = append(rotated, delegates[:start]...)
	return rotated
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

// TestOnlyEpochDrainSnapshot returns the live cursor's delegateIndex,
// voterIndex (mid-delegate resume position), total delegate count, and
// target era, or zero values with present=false when no drain is in
// progress. Used by the e2e perf bench to watch the drain advance chunk
// by chunk. Production callers must not depend on this.
func (p *Protocol) TestOnlyEpochDrainSnapshot(
	ctx context.Context,
	sm protocol.StateReader,
) (delegateIndex uint32, voterIndex uint32, totalDelegates uint32, targetEra uint64, present bool, err error) {
	c, err := p.readEpochDrainCursor(ctx, sm)
	if err != nil || c == nil || c.Completed {
		return 0, 0, 0, 0, false, err
	}
	return c.DelegateIndex, c.VoterIndex, uint32(len(c.Delegates)), c.TargetEra, true, nil
}
