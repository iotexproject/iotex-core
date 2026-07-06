// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/state"
)

// pendingBlockReward is the per-delegate accumulator that holds block-reward
// credits during an epoch. rewardAddr and commissionRate are captured at
// credit time; if the delegate churns out of top-N before the epoch closes,
// the drain still has enough to construct a synthetic state.Candidate for
// distributeVoterReward.
type pendingBlockReward struct {
	amount         *big.Int
	rewardAddr     address.Address
	commissionRate uint64
}

// Serialize encodes to bytes via rewardingpb.PendingBlockReward.
func (r pendingBlockReward) Serialize() ([]byte, error) {
	amt := "0"
	if r.amount != nil {
		amt = r.amount.String()
	}
	var addrBytes []byte
	if r.rewardAddr != nil {
		addrBytes = r.rewardAddr.Bytes()
	}
	return proto.Marshal(&rewardingpb.PendingBlockReward{
		Amount:         amt,
		RewardAddr:     addrBytes,
		CommissionRate: r.commissionRate,
	})
}

// Deserialize decodes bytes into the receiver.
func (r *pendingBlockReward) Deserialize(data []byte) error {
	gen := rewardingpb.PendingBlockReward{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	amount, ok := new(big.Int).SetString(gen.Amount, 10)
	if !ok {
		return errors.Errorf("failed to parse pending block reward amount %q", gen.Amount)
	}
	r.amount = amount
	if len(gen.RewardAddr) > 0 {
		addr, err := address.FromBytes(gen.RewardAddr)
		if err != nil {
			return errors.Wrap(err, "failed to parse pending block reward address")
		}
		r.rewardAddr = addr
	} else {
		r.rewardAddr = nil
	}
	r.commissionRate = gen.CommissionRate
	return nil
}

// pendingBlockRewardIndex is a sorted, deduped list of delegate identity
// addresses that currently hold a pendingBlockReward entry.
type pendingBlockRewardIndex struct {
	identities []address.Address
}

// Serialize encodes to bytes via rewardingpb.PendingBlockRewardIndex.
func (idx pendingBlockRewardIndex) Serialize() ([]byte, error) {
	raw := make([][]byte, len(idx.identities))
	for i, a := range idx.identities {
		raw[i] = a.Bytes()
	}
	return proto.Marshal(&rewardingpb.PendingBlockRewardIndex{Identities: raw})
}

// Deserialize decodes bytes into the receiver.
func (idx *pendingBlockRewardIndex) Deserialize(data []byte) error {
	gen := rewardingpb.PendingBlockRewardIndex{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	out := make([]address.Address, 0, len(gen.Identities))
	for _, raw := range gen.Identities {
		a, err := address.FromBytes(raw)
		if err != nil {
			return errors.Wrap(err, "failed to parse pending block reward index entry")
		}
		out = append(out, a)
	}
	idx.identities = out
	return nil
}

// insert inserts id into the sorted slice if absent. Returns true when a new
// entry was added.
func (idx *pendingBlockRewardIndex) insert(id address.Address) bool {
	target := id.Bytes()
	pos := sort.Search(len(idx.identities), func(i int) bool {
		return bytes.Compare(idx.identities[i].Bytes(), target) >= 0
	})
	if pos < len(idx.identities) && bytes.Equal(idx.identities[pos].Bytes(), target) {
		return false
	}
	idx.identities = append(idx.identities, nil)
	copy(idx.identities[pos+1:], idx.identities[pos:])
	idx.identities[pos] = id
	return true
}

// pendingBlockRewardKey returns the state key for one delegate's pool entry.
func pendingBlockRewardKey(id address.Address) []byte {
	return append(append([]byte{}, _pendingBlockRewardKeyPrefix...), id.Bytes()...)
}

// blockRewardEligibleForVoterSplit mirrors distributeVoterReward's legacy
// fallback triggers exactly. The block-reward routing check must match the
// epoch-reward split check so a delegate can never be pool-credited but then
// snapped back to the legacy path at drain time.
func (p *Protocol) blockRewardEligibleForVoterSplit(fCtx protocol.FeatureCtx, cand *state.Candidate) bool {
	return !fCtx.NoVoterRewardDistribution &&
		cand != nil &&
		cand.Identity != "" &&
		cand.CommissionRate > 0 &&
		cand.CommissionRate <= commissionRateDenominator
}

// creditPendingBlockReward adds amount to the per-delegate pool entry and
// records the delegate in the pool index if absent. rewardAddr and
// commissionRate are refreshed on every credit so orphan drains use the
// latest snapshot values.
func (p *Protocol) creditPendingBlockReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() == 0 {
		return nil
	}
	if amount.Sign() < 0 {
		return errors.Errorf("cannot credit negative amount %s to pending block reward pool", amount.String())
	}
	identityAddr, err := address.FromString(cand.Identity)
	if err != nil {
		return errors.Wrapf(err, "failed to parse candidate identity %q", cand.Identity)
	}
	rewardAddr, err := address.FromString(cand.RewardAddress)
	if err != nil {
		return errors.Wrapf(err, "failed to parse candidate reward address %q", cand.RewardAddress)
	}

	entry := pendingBlockReward{amount: big.NewInt(0)}
	entryKey := pendingBlockRewardKey(identityAddr)
	if _, err := p.state(ctx, sm, entryKey, &entry); err != nil {
		if errors.Cause(err) != state.ErrStateNotExist {
			return errors.Wrap(err, "failed to read pending block reward entry")
		}
		entry = pendingBlockReward{amount: big.NewInt(0)}
	}
	if entry.amount == nil {
		entry.amount = big.NewInt(0)
	}
	entry.amount = new(big.Int).Add(entry.amount, amount)
	entry.rewardAddr = rewardAddr
	entry.commissionRate = cand.CommissionRate
	if err := p.putState(ctx, sm, entryKey, &entry); err != nil {
		return errors.Wrap(err, "failed to write pending block reward entry")
	}

	idx := pendingBlockRewardIndex{}
	if _, err := p.state(ctx, sm, _pendingBlockRewardIndexKey, &idx); err != nil {
		if errors.Cause(err) != state.ErrStateNotExist {
			return errors.Wrap(err, "failed to read pending block reward index")
		}
		idx = pendingBlockRewardIndex{}
	}
	if idx.insert(identityAddr) {
		if err := p.putState(ctx, sm, _pendingBlockRewardIndexKey, &idx); err != nil {
			return errors.Wrap(err, "failed to write pending block reward index")
		}
	}
	return nil
}

// drainPendingBlockRewards iterates the pool index at epoch close, hands each
// entry to distributeVoterReward, and clears the pool. candidates is the
// current-epoch poll snapshot — entries whose identity is present take the
// snapshot's fresher commissionRate/rewardAddr; entries whose identity is not
// present (orphans — delegate rotated out of top-N) fall back to the values
// frozen on the pool entry at credit time.
//
// Fund accounting: the fund was already debited at block-credit time via
// updateAvailableBalance(totalReward). The drain only moves money from pool
// entries to per-address unclaimed balances via grantToAccount inside
// distributeVoterReward; no additional fund mutation occurs here.
func (p *Protocol) drainPendingBlockRewards(
	ctx context.Context,
	sm protocol.StateManager,
	candidates state.CandidateList,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, error) {
	idx := pendingBlockRewardIndex{}
	if _, err := p.state(ctx, sm, _pendingBlockRewardIndexKey, &idx); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read pending block reward index")
	}
	if len(idx.identities) == 0 {
		return nil, nil
	}

	byIdentity := make(map[string]*state.Candidate, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Identity == "" {
			continue
		}
		byIdentity[c.Identity] = c
	}

	logs := make([]*action.Log, 0)
	for _, id := range idx.identities {
		entryKey := pendingBlockRewardKey(id)
		entry := pendingBlockReward{}
		if _, err := p.state(ctx, sm, entryKey, &entry); err != nil {
			if errors.Cause(err) == state.ErrStateNotExist {
				continue
			}
			return nil, errors.Wrapf(err,
				"failed to read pending block reward entry for %s", id.String())
		}
		if entry.amount == nil || entry.amount.Sign() == 0 {
			if err := p.deleteState(ctx, sm, entryKey, &pendingBlockReward{}); err != nil {
				return nil, errors.Wrapf(err,
					"failed to delete empty pending block reward entry for %s", id.String())
			}
			continue
		}

		var cand *state.Candidate
		if fresh, ok := byIdentity[id.String()]; ok {
			cand = fresh
		} else {
			cand = &state.Candidate{
				Identity:       id.String(),
				CommissionRate: entry.commissionRate,
			}
			if entry.rewardAddr != nil {
				cand.RewardAddress = entry.rewardAddr.String()
			}
		}
		payoutAddr := entry.rewardAddr
		if cand.RewardAddress != "" {
			if pa, err := address.FromString(cand.RewardAddress); err == nil {
				payoutAddr = pa
			}
		}
		if payoutAddr == nil {
			return nil, errors.Errorf(
				"pending block reward entry for %s has no reward address", id.String())
		}

		voterLogs, handled, err := p.distributeVoterReward(
			ctx, sm, cand, payoutAddr, entry.amount, blkHeight, actionHash)
		if err != nil {
			return nil, err
		}
		if handled {
			logs = append(logs, voterLogs...)
		} else {
			// Belt-and-suspenders: pre-flag or CommissionRate=0 wouldn't have
			// credited this pool entry in the first place, but if we somehow
			// arrive here (e.g. flag re-flipped mid-epoch), pay the delegate
			// directly and emit an EPOCH_REWARD log so no funds are stranded.
			if err := p.grantToAccount(ctx, sm, payoutAddr, entry.amount); err != nil {
				return nil, err
			}
			data, err := p.encodeRewardLog(
				rewardingpb.RewardLog_EPOCH_REWARD, payoutAddr.String(), entry.amount)
			if err != nil {
				return nil, err
			}
			logs = append(logs, &action.Log{
				Address:     p.addr.String(),
				Topics:      nil,
				Data:        data,
				BlockHeight: blkHeight,
				ActionHash:  actionHash,
			})
		}
		if err := p.deleteState(ctx, sm, entryKey, &pendingBlockReward{}); err != nil {
			return nil, errors.Wrapf(err,
				"failed to delete pending block reward entry for %s", id.String())
		}
	}

	if err := p.deleteState(ctx, sm, _pendingBlockRewardIndexKey, &pendingBlockRewardIndex{}); err != nil {
		return nil, errors.Wrap(err, "failed to delete pending block reward index")
	}
	return logs, nil
}
