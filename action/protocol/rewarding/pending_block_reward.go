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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

const _candidateIdentifierSize = 20

// pendingBlockRewardPool is a single delegate's accumulated block reward
// balance under IIP-59 §3.2. Created lazily on first credit and decremented as
// voters are paid; rounding residual may carry into a later era. Value is
// stored as raw big-endian bytes so the entry is compact
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
// otherwise the reduced balance is persisted. Used by voter reward distribution
// chunks so any voter-side
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

// listPendingBlockRewardPoolIDsForRead answers the same question as
// listPendingBlockRewardPoolIDs for the read-only ReadState path, on state
// readers that cannot serve an ordered range scan.
//
// An archive node serves a historical height out of the erigon store, whose
// objects are addressed by contract slot instead of by an ordered iotex key
// space; it rejects RangeOption rather than hand back a differently-ordered
// answer. Point reads are unaffected, so the same set is recoverable by probing
// one key per candidate: a pool is only ever credited under a candidate
// identifier and candidate records are not deleted, so the candidate set at that
// height contains every delegate that can hold a pool.
//
// Read paths only. freezePendingPoolDrainWork must keep failing on a rejected
// scan: an era freeze that quietly switched enumeration source would make block
// validity depend on which storage backend a node happens to run.
func (p *Protocol) listPendingBlockRewardPoolIDsForRead(
	ctx context.Context,
	sr protocol.StateReader,
) ([][]byte, error) {
	// Only the scan itself is retried differently. Anything else the scan
	// reports -- a malformed key, a decode failure -- is real damage in state
	// and must reach the caller instead of being papered over by a second,
	// weaker enumeration.
	ids, err := p.listPendingBlockRewardPoolIDs(ctx, sr)
	if err == nil || !errors.Is(err, db.ErrNotSupported) {
		return ids, err
	}
	return p.probePendingBlockRewardPoolIDs(sr)
}

// probePendingBlockRewardPoolIDs rebuilds the pool set with one point read per
// candidate. O(candidates) round-trips where the range scan costs one, which is
// why it stays behind the ErrNotSupported fallback instead of becoming the
// default.
func (p *Protocol) probePendingBlockRewardPoolIDs(sr protocol.StateReader) ([][]byte, error) {
	candidates, err := staking.CandidateIdentifiersFor(sr)
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: enumerate candidates for pending block reward pool probe")
	}
	ids := make([][]byte, 0, len(candidates))
	for _, cand := range candidates {
		if cand == nil {
			continue
		}
		candID := cand.Bytes()
		// stateV2 rather than state: the scan being stood in for covers the V2
		// namespace alone, and the legacy fallback inside p.state would double the
		// round-trips looking for keys that cannot be there.
		var entry pendingBlockRewardPool
		switch _, err := p.stateV2(sr, pendingBlockRewardPoolKey(candID), &entry); {
		case err == nil:
			ids = append(ids, candID)
		case errors.Is(err, state.ErrStateNotExist):
		default:
			return nil, err
		}
	}
	// The scan contract is ascending by state key; the shared key prefix makes
	// that the same order as ascending by candidate identifier.
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i], ids[j]) < 0 })
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
