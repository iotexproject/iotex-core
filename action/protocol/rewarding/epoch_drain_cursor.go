// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

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
	CandidateIdentifier []byte
	VoterAmountFrozen   *big.Int
	RewardAddress       []byte
	EpochCommission     *big.Int
}

// epochDrainCursor checkpoints an in-progress IIP-59 era-boundary drain
// of PendingBlockRewardPool balances into voter accounts. TargetEra is
// the era ID being drained; DelegateIndex is the resume position in
// the frozen Delegates slice; VoterIndex is the resume position inside
// the delegate at DelegateIndex when a per-block voter cap stops
// payout mid-delegate — 0 whenever the entry at DelegateIndex is
// fresh. Delegates is the frozen work list. Cursor presence in the
// RewardingNamespace signals a drain is live and the system-action
// layer must emit a continuation grant on the next block. Absence =
// no drain in progress.
type epochDrainCursor struct {
	TargetEra     uint64
	DelegateIndex uint32
	VoterIndex    uint32
	Delegates     []epochDrainDelegateWork
}

// Serialize marshals the cursor to its proto wire form.
func (c epochDrainCursor) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainCursor{
		TargetEra:     c.TargetEra,
		DelegateIndex: c.DelegateIndex,
		VoterIndex:    c.VoterIndex,
	}
	if len(c.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(c.Delegates))
		for i, d := range c.Delegates {
			m.Delegates[i] = &rewardingpb.EpochDrainDelegateWork{
				CandidateIdentifier: d.CandidateIdentifier,
				VoterAmountFrozen:   epochDrainBigIntBytes(d.VoterAmountFrozen),
				RewardAddress:       d.RewardAddress,
				EpochCommission:     epochDrainBigIntBytes(d.EpochCommission),
			}
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
	c.Delegates = nil
	if ds := m.GetDelegates(); len(ds) > 0 {
		c.Delegates = make([]epochDrainDelegateWork, len(ds))
		for i, d := range ds {
			c.Delegates[i] = epochDrainDelegateWork{
				CandidateIdentifier: d.GetCandidateIdentifier(),
				VoterAmountFrozen:   epochDrainBytesBigInt(d.GetVoterAmountFrozen()),
				RewardAddress:       d.GetRewardAddress(),
				EpochCommission:     epochDrainBytesBigInt(d.GetEpochCommission()),
			}
		}
	}
	return nil
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

// readEpochDrainCursor returns the live cursor, or (nil, nil) when no
// drain is in progress.
func (p *Protocol) readEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateReader,
) (*epochDrainCursor, error) {
	c := &epochDrainCursor{}
	if _, err := p.state(ctx, sm, state.EpochDrainCursorKey, c); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// writeEpochDrainCursor persists a cursor. Overwrites any prior value.
func (p *Protocol) writeEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
	c *epochDrainCursor,
) error {
	return p.putState(ctx, sm, state.EpochDrainCursorKey, c)
}

// deleteEpochDrainCursor removes the cursor entry. Idempotent — the
// underlying deleteState swallows ErrStateNotExist.
func (p *Protocol) deleteEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	return p.deleteState(ctx, sm, state.EpochDrainCursorKey, &epochDrainCursor{})
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
	if err != nil || c == nil {
		return 0, 0, 0, 0, false, err
	}
	return c.DelegateIndex, c.VoterIndex, uint32(len(c.Delegates)), c.TargetEra, true, nil
}
