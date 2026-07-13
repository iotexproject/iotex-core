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

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// _pendingBlockRewardPoolIndexKey stores the sorted list of candidate
// identifier byte-slices that currently hold a non-zero pool balance. It
// makes end-of-epoch enumeration deterministic without a namespace scan;
// the individual balances live at
// _pendingBlockRewardPoolKeyPrefix || candidateIdentifier.
var _pendingBlockRewardPoolIndexKey = []byte("pbrpx")

// pendingBlockRewardPool is a single delegate's accumulated block reward
// balance under IIP-59 §3.2. Created lazily on first credit, deleted on
// drain. Value is stored as raw big-endian bytes so the entry is compact
// and independent of the big.Int text-radix representation.
type pendingBlockRewardPool struct {
	amount *big.Int
}

// Serialize marshals the pool balance for storage.
func (b pendingBlockRewardPool) Serialize() ([]byte, error) {
	m := &rewardingpb.PendingBlockRewardPool{}
	if b.amount != nil {
		m.Amount = b.amount.Bytes()
	}
	return proto.Marshal(m)
}

// Deserialize unmarshals a stored pool balance.
func (b *pendingBlockRewardPool) Deserialize(data []byte) error {
	m := &rewardingpb.PendingBlockRewardPool{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	if len(m.Amount) == 0 {
		b.amount = new(big.Int)
	} else {
		b.amount = new(big.Int).SetBytes(m.Amount)
	}
	return nil
}

// pendingBlockRewardPoolIndex is the sorted list of candidate identifier
// byte-slices with a non-zero pool balance. Sorted at every write so
// enumeration order is deterministic across replay.
type pendingBlockRewardPoolIndex struct {
	ids [][]byte
}

// Serialize marshals the index. The proto wire format preserves the
// caller-supplied byte-slice ordering, which is already sorted.
func (i pendingBlockRewardPoolIndex) Serialize() ([]byte, error) {
	m := &rewardingpb.Exempt{Addrs: i.ids}
	return proto.Marshal(m)
}

// Deserialize decodes the index list. Reuses the rewardingpb.Exempt shape
// (repeated bytes addrs) to avoid a second proto message for a plain
// list-of-bytes payload.
func (i *pendingBlockRewardPoolIndex) Deserialize(data []byte) error {
	m := &rewardingpb.Exempt{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	i.ids = m.Addrs
	return nil
}

// pendingBlockRewardPoolKey returns the per-delegate key layout used by
// credit/read/delete. The prefix bytes are copied so the returned slice is
// independent of the prefix constant.
func pendingBlockRewardPoolKey(candID []byte) []byte {
	k := make([]byte, 0, len(_pendingBlockRewardPoolKeyPrefix)+len(candID))
	k = append(k, _pendingBlockRewardPoolKeyPrefix...)
	k = append(k, candID...)
	return k
}

// readPendingBlockRewardPool returns the accumulated pool amount for a
// delegate. A missing entry returns zero — not an error — so callers can
// treat "no pool entry" and "zero pool entry" identically.
func (p *Protocol) readPendingBlockRewardPool(
	ctx context.Context,
	sm protocol.StateReader,
	candID []byte,
) (*big.Int, error) {
	entry := pendingBlockRewardPool{}
	if _, err := p.state(ctx, sm, pendingBlockRewardPoolKey(candID), &entry); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return new(big.Int), nil
		}
		return nil, err
	}
	if entry.amount == nil {
		return new(big.Int), nil
	}
	return entry.amount, nil
}

// creditPendingBlockRewardPool adds amount to the delegate's pool balance
// and inserts the candidate into the enumeration index if absent. Zero or
// nil amount is a no-op. The caller has already debited amount from
// unclaimedBalance via updateAvailableBalance; the pool holds the balance
// until drain.
func (p *Protocol) creditPendingBlockRewardPool(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() <= 0 {
		return nil
	}
	entry := pendingBlockRewardPool{}
	key := pendingBlockRewardPoolKey(candID)
	if _, err := p.state(ctx, sm, key, &entry); err != nil {
		if !errors.Is(err, state.ErrStateNotExist) {
			return err
		}
		entry.amount = new(big.Int)
	}
	if entry.amount == nil {
		entry.amount = new(big.Int)
	}
	entry.amount = new(big.Int).Add(entry.amount, amount)
	if err := p.putState(ctx, sm, key, &entry); err != nil {
		return err
	}
	return p.addPendingBlockRewardPoolIndex(ctx, sm, candID)
}

// deletePendingBlockRewardPool removes a delegate's pool entry and its
// index entry. Idempotent — a missing key is silently swallowed by
// deleteState.
func (p *Protocol) deletePendingBlockRewardPool(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
) error {
	if err := p.deleteState(ctx, sm, pendingBlockRewardPoolKey(candID), &pendingBlockRewardPool{}); err != nil {
		return err
	}
	return p.removePendingBlockRewardPoolIndex(ctx, sm, candID)
}

// readPendingBlockRewardPoolIndex returns the deterministic list of
// candidate identifier byte-slices that currently hold a pool balance.
// Returns an empty slice for a missing index (no entries exist yet).
func (p *Protocol) readPendingBlockRewardPoolIndex(
	ctx context.Context,
	sm protocol.StateReader,
) ([][]byte, error) {
	idx := pendingBlockRewardPoolIndex{}
	if _, err := p.state(ctx, sm, _pendingBlockRewardPoolIndexKey, &idx); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, err
	}
	// Return a deep copy so callers can safely mutate the returned slice
	// without accidentally reaching back into the deserialised state.
	out := make([][]byte, len(idx.ids))
	for i, id := range idx.ids {
		cp := make([]byte, len(id))
		copy(cp, id)
		out[i] = cp
	}
	return out, nil
}

func (p *Protocol) writePendingBlockRewardPoolIndex(
	ctx context.Context,
	sm protocol.StateManager,
	ids [][]byte,
) error {
	if len(ids) == 0 {
		return p.deleteState(ctx, sm, _pendingBlockRewardPoolIndexKey, &pendingBlockRewardPoolIndex{})
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i], ids[j]) < 0 })
	return p.putState(ctx, sm, _pendingBlockRewardPoolIndexKey, &pendingBlockRewardPoolIndex{ids: ids})
}

func (p *Protocol) addPendingBlockRewardPoolIndex(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
) error {
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	if err != nil {
		return err
	}
	// Binary-search insert to keep the index sorted at all times.
	pos := sort.Search(len(ids), func(i int) bool { return bytes.Compare(ids[i], candID) >= 0 })
	if pos < len(ids) && bytes.Equal(ids[pos], candID) {
		return nil
	}
	cp := make([]byte, len(candID))
	copy(cp, candID)
	ids = append(ids, nil)
	copy(ids[pos+1:], ids[pos:])
	ids[pos] = cp
	return p.writePendingBlockRewardPoolIndex(ctx, sm, ids)
}

func (p *Protocol) removePendingBlockRewardPoolIndex(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
) error {
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	if err != nil {
		return err
	}
	pos := sort.Search(len(ids), func(i int) bool { return bytes.Compare(ids[i], candID) >= 0 })
	if pos >= len(ids) || !bytes.Equal(ids[pos], candID) {
		return nil
	}
	ids = append(ids[:pos], ids[pos+1:]...)
	return p.writePendingBlockRewardPoolIndex(ctx, sm, ids)
}

// candidateIdentifierBytes resolves the byte form of a candidate identifier
// suitable for pool-key storage. Uses the same address-as-identifier
// convention distributeVoterReward reads via staking.PollSnapshotFor.
func candidateIdentifierBytes(candAddress string) ([]byte, error) {
	if candAddress == "" {
		return nil, errors.New("rewarding: empty candidate address for pool key")
	}
	addr, err := address.FromString(candAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "rewarding: invalid candidate address %q", candAddress)
	}
	return addr.Bytes(), nil
}

// refundPendingBlockRewardPool returns amount to the rewarding fund's
// unclaimed balance. Used by the orphan-drain fallback when a pool entry
// has no reachable destination (candidate fully unregistered, no reward
// address). Never touches totalBalance — the deposit that funded this
// amount is still on the books.
func (p *Protocol) refundPendingBlockRewardPool(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() <= 0 {
		return nil
	}
	f := fund{}
	if _, err := p.state(ctx, sm, _fundKey, &f); err != nil {
		return err
	}
	f.unclaimedBalance = new(big.Int).Add(f.unclaimedBalance, amount)
	return p.putState(ctx, sm, _fundKey, &f)
}

// drainPendingBlockRewardOrphans handles pool entries left over after the
// per-candidate epoch loop. Any pool ID not in visited is a delegate that
// dropped out of the current epoch's reward split entirely (deactivated,
// unregistered, or otherwise fell off the poll list) after having
// accumulated block reward inside the epoch.
//
// Resolution order per orphan:
//  1. Look up the live staking.Candidate by owner address. If present and
//     .Reward is set, credit the pool balance to that reward address and
//     emit a BLOCK_REWARD log naming it.
//  2. Otherwise (candidate fully gone, or no reward address), refund the
//     balance to fund.unclaimedBalance and emit a BLOCK_REWARD log with
//     an empty addr for observability. Never burn — that would violate
//     the unclaimedBalance ≤ totalBalance invariant.
//
// Regardless of destination, the pool entry and its index membership are
// deleted so a replay is a no-op.
func (p *Protocol) drainPendingBlockRewardOrphans(
	ctx context.Context,
	sm protocol.StateManager,
	visited map[string]bool,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, error) {
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var csr staking.CandidateStateReader
	logs := make([]*action.Log, 0)
	for _, candID := range ids {
		if visited[string(candID)] {
			continue
		}
		poolAmt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		if err != nil {
			return nil, err
		}
		if poolAmt.Sign() == 0 {
			if err := p.deletePendingBlockRewardPool(ctx, sm, candID); err != nil {
				return nil, err
			}
			continue
		}

		var target address.Address
		var targetStr string
		candAddr, addrErr := address.FromBytes(candID)
		if addrErr == nil {
			if csr == nil {
				csr, err = staking.ConstructBaseView(sm)
				if err != nil {
					return nil, errors.Wrap(err, "rewarding: construct base view for orphan drain")
				}
			}
			if cand := csr.GetCandidateByOwner(candAddr); cand != nil && cand.Reward != nil {
				target = cand.Reward
				targetStr = cand.Reward.String()
			}
		} else {
			log.L().Warn("rewarding: orphan pool ID does not decode to an address; refunding",
				zap.Binary("candID", candID),
				zap.Error(addrErr))
		}

		if target != nil {
			if err := p.grantToAccount(ctx, sm, target, poolAmt); err != nil {
				return nil, err
			}
		} else {
			if err := p.refundPendingBlockRewardPool(ctx, sm, poolAmt); err != nil {
				return nil, err
			}
		}
		data, err := p.encodeRewardLog(rewardingpb.RewardLog_BLOCK_REWARD, targetStr, poolAmt)
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
		if err := p.deletePendingBlockRewardPool(ctx, sm, candID); err != nil {
			return nil, err
		}
	}
	return logs, nil
}
