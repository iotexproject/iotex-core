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
)

// epochDrainDelegateWork is a Phase A snapshot of one per-delegate
// reward split. Continuation chunks read this rather than re-invoking
// splitEpochReward against a working set that has already advanced to
// epoch N+1's candidate/pool view.
type epochDrainDelegateWork struct {
	CandidateAddress string
	RewardAddress    []byte
	HasRewardAddress bool
	EpochAmount      *big.Int
	PoolAmountFrozen *big.Int
}

// epochDrainFoundationBonusWork is a Phase A snapshot of one foundation
// bonus grant. Reward address string is retained separately for the
// reward log payload.
type epochDrainFoundationBonusWork struct {
	RewardAddressStr string
	RewardAddress    []byte
	Amount           *big.Int
}

// epochDrainOrphanWork is a Phase A snapshot of one orphan pool entry
// (a delegate not present in the current epoch reward split). Target
// address is empty when Phase A couldn't resolve a live reward
// routing — the amount then falls back to unclaimed refund.
type epochDrainOrphanWork struct {
	CandidateIdentifier []byte
	PoolAmountFrozen    *big.Int
	TargetAddress       []byte
	TargetAddressStr    string
}

// epochDrainCursor checkpoints a multi-block IIP-59 epoch reward drain.
// TargetEpoch identifies the epoch whose delegates are being paid;
// DelegateIndex is the resume position in the frozen Delegates slice.
// Delegates, FoundationBonus, and Orphans are frozen at Phase A so
// chunks and Coda in later blocks read stable inputs even after the
// working set has advanced to epoch N+1.
type epochDrainCursor struct {
	TargetEpoch     uint64
	DelegateIndex   uint32
	Delegates       []epochDrainDelegateWork
	FoundationBonus []epochDrainFoundationBonusWork
	Orphans         []epochDrainOrphanWork
}

// Serialize marshals the cursor to its proto wire form.
func (c epochDrainCursor) Serialize() ([]byte, error) {
	m := &rewardingpb.EpochDrainCursor{
		TargetEpoch:   c.TargetEpoch,
		DelegateIndex: c.DelegateIndex,
	}
	if len(c.Delegates) > 0 {
		m.Delegates = make([]*rewardingpb.EpochDrainDelegateWork, len(c.Delegates))
		for i, d := range c.Delegates {
			m.Delegates[i] = &rewardingpb.EpochDrainDelegateWork{
				CandidateAddress: d.CandidateAddress,
				RewardAddress:    d.RewardAddress,
				HasRewardAddress: d.HasRewardAddress,
				EpochAmount:      bigIntBytes(d.EpochAmount),
				PoolAmountFrozen: bigIntBytes(d.PoolAmountFrozen),
			}
		}
	}
	if len(c.FoundationBonus) > 0 {
		m.FoundationBonus = make([]*rewardingpb.EpochDrainFoundationBonusWork, len(c.FoundationBonus))
		for i, f := range c.FoundationBonus {
			m.FoundationBonus[i] = &rewardingpb.EpochDrainFoundationBonusWork{
				RewardAddressStr: f.RewardAddressStr,
				RewardAddress:    f.RewardAddress,
				Amount:           bigIntBytes(f.Amount),
			}
		}
	}
	if len(c.Orphans) > 0 {
		m.Orphans = make([]*rewardingpb.EpochDrainOrphanWork, len(c.Orphans))
		for i, o := range c.Orphans {
			m.Orphans[i] = &rewardingpb.EpochDrainOrphanWork{
				CandidateIdentifier: o.CandidateIdentifier,
				PoolAmountFrozen:    bigIntBytes(o.PoolAmountFrozen),
				TargetAddress:       o.TargetAddress,
				TargetAddressStr:    o.TargetAddressStr,
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
	c.TargetEpoch = m.GetTargetEpoch()
	c.DelegateIndex = m.GetDelegateIndex()
	if ds := m.GetDelegates(); len(ds) > 0 {
		c.Delegates = make([]epochDrainDelegateWork, len(ds))
		for i, d := range ds {
			c.Delegates[i] = epochDrainDelegateWork{
				CandidateAddress: d.GetCandidateAddress(),
				RewardAddress:    d.GetRewardAddress(),
				HasRewardAddress: d.GetHasRewardAddress(),
				EpochAmount:      bytesBigInt(d.GetEpochAmount()),
				PoolAmountFrozen: bytesBigInt(d.GetPoolAmountFrozen()),
			}
		}
	}
	if fs := m.GetFoundationBonus(); len(fs) > 0 {
		c.FoundationBonus = make([]epochDrainFoundationBonusWork, len(fs))
		for i, f := range fs {
			c.FoundationBonus[i] = epochDrainFoundationBonusWork{
				RewardAddressStr: f.GetRewardAddressStr(),
				RewardAddress:    f.GetRewardAddress(),
				Amount:           bytesBigInt(f.GetAmount()),
			}
		}
	}
	if os := m.GetOrphans(); len(os) > 0 {
		c.Orphans = make([]epochDrainOrphanWork, len(os))
		for i, o := range os {
			c.Orphans[i] = epochDrainOrphanWork{
				CandidateIdentifier: o.GetCandidateIdentifier(),
				PoolAmountFrozen:    bytesBigInt(o.GetPoolAmountFrozen()),
				TargetAddress:       o.GetTargetAddress(),
				TargetAddressStr:    o.GetTargetAddressStr(),
			}
		}
	}
	return nil
}

// bigIntBytes returns big-endian bytes, or nil for nil.
func bigIntBytes(v *big.Int) []byte {
	if v == nil {
		return nil
	}
	return v.Bytes()
}

// bytesBigInt returns a big.Int from big-endian bytes. Zero-length
// input yields a zero-valued big.Int (not nil), so callers can compare
// with .Sign() without a nil check.
func bytesBigInt(b []byte) *big.Int {
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

// deleteEpochDrainCursor removes the cursor entry. Idempotent —
// swallows the underlying not-exist error.
func (p *Protocol) deleteEpochDrainCursor(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	return p.deleteState(ctx, sm, state.EpochDrainCursorKey, &epochDrainCursor{})
}
