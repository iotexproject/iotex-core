// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

const _candidateIdentifierSize = 20

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

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (b *pendingBlockRewardPool) Encode() (systemcontracts.GenericValue, error) {
	data, err := b.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (b *pendingBlockRewardPool) Decode(v systemcontracts.GenericValue) error {
	return b.Deserialize(v.PrimaryData)
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

// creditPendingBlockRewardPool adds amount to the delegate's pool balance.
// Zero or nil amount is a no-op. The caller has already debited amount from
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
	if _, err := p.state(ctx, sm, key, &entry); err != nil && !errors.Is(err, state.ErrStateNotExist) {
		return err
	}
	// Nil on both paths: a first credit for this delegate, and a decoded entry
	// whose amount field was absent.
	if entry.amount == nil {
		entry.amount = new(big.Int)
	}
	entry.amount = new(big.Int).Add(entry.amount, amount)
	return p.putState(ctx, sm, key, &entry)
}

// deletePendingBlockRewardPool removes a delegate's pool entry. Idempotent —
// a missing key is silently swallowed by deleteState.
func (p *Protocol) deletePendingBlockRewardPool(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
) error {
	return p.deleteState(ctx, sm, pendingBlockRewardPoolKey(candID), &pendingBlockRewardPool{})
}

// decrementPendingBlockRewardPool subtracts amount from the delegate's
// pool balance. If the resulting balance is zero, the entry is deleted;
// otherwise the reduced balance is persisted. Used by Phase B chunk drain
// so any voter-side
// balance that accrued after the era-boundary freeze is preserved for
// the next era's cursor. Amount larger than the current balance is
// clamped to the balance (guards against arithmetic slippage).
func (p *Protocol) decrementPendingBlockRewardPool(
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
		if errors.Is(err, state.ErrStateNotExist) {
			return nil
		}
		return err
	}
	if entry.amount == nil {
		entry.amount = new(big.Int)
	}
	if entry.amount.Cmp(amount) <= 0 {
		return p.deletePendingBlockRewardPool(ctx, sm, candID)
	}
	entry.amount = new(big.Int).Sub(entry.amount, amount)
	return p.putState(ctx, sm, key, &entry)
}

// pendingBlockRewardPoolRange is the half-open V2 state-key range containing
// every per-delegate pool entry. IIP-59 is required to activate after the
// Greenland/V2 rewarding-store fork, so these unhashed keys are enumerable.
func (p *Protocol) pendingBlockRewardPoolRange() ([]byte, []byte) {
	min := make([]byte, 0, len(p.keyPrefix)+len(_pendingBlockRewardPoolKeyPrefix))
	min = append(min, p.keyPrefix...)
	min = append(min, _pendingBlockRewardPoolKeyPrefix...)
	max := append([]byte(nil), min...)
	max[len(max)-1]++
	return min, max
}

// listPendingBlockRewardPoolIDs enumerates candidate identifiers with a pool
// entry in deterministic byte order. Before V2 storage is active there can be
// no IIP-59 pool entries, and legacy rewarding keys are hashed and therefore
// not prefix-enumerable.
func (p *Protocol) listPendingBlockRewardPoolIDs(
	ctx context.Context,
	sr protocol.StateReader,
) ([][]byte, error) {
	if !useV2Storage(ctx) {
		return nil, nil
	}
	min, max := p.pendingBlockRewardPoolRange()
	_, iter, err := sr.States(
		protocol.NamespaceOption(_v2RewardingNamespace),
		protocol.RangeOption(min, max),
	)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, err
	}
	ids := make([][]byte, 0, iter.Size())
	var previous []byte
	for i := 0; i < iter.Size(); i++ {
		var pool pendingBlockRewardPool
		key, err := iter.Next(&pool)
		if err != nil {
			return nil, errors.Wrap(err, "rewarding: decode pending block reward pool during range scan")
		}
		if !bytes.HasPrefix(key, min) || len(key) != len(min)+_candidateIdentifierSize {
			return nil, errors.Errorf("rewarding: malformed pending block reward pool key %x", key)
		}
		if previous != nil && bytes.Compare(key, previous) <= 0 {
			return nil, errors.Errorf(
				"rewarding: pending block reward pool scan returned non-ascending key %x after %x",
				key, previous,
			)
		}
		ids = append(ids, append([]byte(nil), key[len(min):]...))
		previous = key
	}
	return ids, nil
}

// candidateIdentifierBytes resolves the stable candidate identity used by
// candidate-scoped IIP-59 state.
func candidateIdentifierBytes(candidateIdentity string) ([]byte, error) {
	if candidateIdentity == "" {
		return nil, errors.New("rewarding: empty candidate identity for pool key")
	}
	addr, err := address.FromString(candidateIdentity)
	if err != nil {
		return nil, errors.Wrapf(err, "rewarding: invalid candidate identity %q", candidateIdentity)
	}
	return addr.Bytes(), nil
}

// candidateIdentifier returns the stable candidate identity. Legacy poll
// records did not populate Identity, where Address was also the identifier.
func candidateIdentifier(candidate *state.Candidate) string {
	if candidate == nil {
		return ""
	}
	if candidate.Identity != "" {
		return candidate.Identity
	}
	return candidate.Address
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
//  1. Look up the live staking.Candidate by identifier. If present, credit
//     the pool balance to its owner (the post-fork default reward address)
//     and emit a BLOCK_REWARD log naming it.
//  2. Otherwise (candidate fully gone), refund the
//     balance to fund.unclaimedBalance and emit a BLOCK_REWARD log with
//     an empty addr for observability. Never burn — that would violate
//     the unclaimedBalance ≤ totalBalance invariant.
//
// Regardless of destination, the pool entry is deleted so a replay is a no-op.
func (p *Protocol) drainPendingBlockRewardOrphans(
	ctx context.Context,
	sm protocol.StateManager,
	visited map[string]bool,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.TransactionLog, []*action.Log, error) {
	ids, err := p.listPendingBlockRewardPoolIDs(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, nil
	}
	transactionLogs := make([]*action.TransactionLog, 0)
	logs := make([]*action.Log, 0)
	for _, candID := range ids {
		if visited[string(candID)] {
			continue
		}
		poolAmt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		if err != nil {
			return nil, nil, err
		}
		if poolAmt.Sign() == 0 {
			if err := p.deletePendingBlockRewardPool(ctx, sm, candID); err != nil {
				return nil, nil, err
			}
			continue
		}

		tLog, rLog, err := p.sweepPendingPoolAmount(ctx, sm, candID, poolAmt, blkHeight, actionHash)
		if err != nil {
			return nil, nil, err
		}
		if tLog != nil {
			transactionLogs = append(transactionLogs, tLog)
		}
		logs = append(logs, rLog)
		if err := p.deletePendingBlockRewardPool(ctx, sm, candID); err != nil {
			return nil, nil, err
		}
	}
	return transactionLogs, logs, nil
}

// sweepPendingPoolAmount moves one amount out of a delegate's pending pool to
// the delegate's owner, or back to the rewarding fund's unclaimed balance when
// no owner can be resolved, and emits the BLOCK_REWARD log that records it.
//
// Both sinks of unpaid pool money go through here: the orphan drain, which
// sweeps a whole pool whose delegate fell off the list, and the drain's
// completion residual, which sweeps the part of a frozen pool that floor
// division and the payout clamp left behind. There is deliberately only one
// sink -- a second one would be a second place for the fund accounting
// invariant to be got wrong.
//
// The pool is decremented rather than deleted, because the pool may have kept
// accruing behind the drain and that accrual belongs to the next era. The
// decrement deletes the entry on its own when it empties.
func (p *Protocol) sweepPendingPoolAmount(
	ctx context.Context,
	sm protocol.StateManager,
	candID []byte,
	amount *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) (*action.TransactionLog, *action.Log, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, nil, nil
	}
	var target address.Address
	var targetStr string
	candAddr, addrErr := address.FromBytes(candID)
	if addrErr == nil {
		routing, rewardErr := staking.ReadCandidateRewardRouting(sm, candAddr,
			genesis.MustExtractGenesisContext(ctx).HermesRewardVaultAddresses)
		if rewardErr == nil {
			target = routing.Owner
			targetStr = routing.Owner.String()
		} else if !errors.Is(rewardErr, state.ErrStateNotExist) {
			return nil, nil, errors.Wrap(rewardErr, "rewarding: read candidate owner for pool sweep")
		}
	} else {
		log.L().Warn("rewarding: pool ID does not decode to an address; refunding",
			zap.Binary("candID", candID),
			zap.Error(addrErr))
	}

	var tLog *action.TransactionLog
	if target != nil {
		var err error
		if tLog, err = p.creditRewardDirect(ctx, sm, target, amount); err != nil {
			return nil, nil, err
		}
	} else if err := p.refundPendingBlockRewardPool(ctx, sm, amount); err != nil {
		return nil, nil, err
	}
	data, err := p.encodeRewardLog(rewardingpb.RewardLog_BLOCK_REWARD, targetStr, amount)
	if err != nil {
		return nil, nil, err
	}
	if err := p.decrementPendingBlockRewardPool(ctx, sm, candID, amount); err != nil {
		return nil, nil, err
	}
	return tLog, &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}, nil
}
