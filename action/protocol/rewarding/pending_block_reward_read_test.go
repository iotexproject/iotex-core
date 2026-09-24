// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// rangeScanRejectingReader stands in for the erigon-backed reader an archive
// node uses at a historical height: point reads and full-namespace reads work,
// ordered range scans are refused because the backing store cannot honour
// [min, max) ordering.
type rangeScanRejectingReader struct {
	protocol.StateManager
	refused int
}

func (r *rangeScanRejectingReader) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, nil, err
	}
	if cfg.RangeMin != nil || cfg.RangeMax != nil || cfg.Limit > 0 {
		r.refused++
		return 0, nil, errors.Wrap(db.ErrNotSupported, "erigon store does not support ordered range scan")
	}
	return r.StateManager.States(opts...)
}

// seedPendingPoolDelegates registers a candidate record for every id and credits
// a pool to all but the last one, which is the negative control.
func seedPendingPoolDelegates(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	withPool []address.Address,
	withoutPool address.Address,
) {
	t.Helper()
	r := require.New(t)
	for _, id := range withPool {
		r.NoError(staking.TestOnlyPutCandidateRewardAddress(ctx, sm, id, id, id, false, true))
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, id.Bytes(), big.NewInt(10)))
	}
	r.NoError(staking.TestOnlyPutCandidateRewardAddress(
		ctx, sm, withoutPool, withoutPool, withoutPool, false, true))
}

// TestPendingVoterRewardDelegates_ReadWithoutRangeScan — a reader that cannot
// serve an ordered range scan (an archive node at a historical height) must
// still answer PendingVoterRewardDelegates, with the same bytes the scan-capable
// reader produces at the same state.
func TestPendingVoterRewardDelegates_ReadWithoutRangeScan(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	// Deliberately not in ascending byte order, so an implementation that just
	// echoed the candidate enumeration order would fail the equality check.
	withPool := []address.Address{
		identityset.Address(9), identityset.Address(2), identityset.Address(7),
	}
	seedPendingPoolDelegates(t, ctx, sm, p, withPool, identityset.Address(11))

	want, wantHeight, err := p.ReadState(ctx, sm, []byte("PendingVoterRewardDelegates"))
	r.NoError(err)

	restricted := &rangeScanRejectingReader{StateManager: sm}
	got, gotHeight, err := p.ReadState(ctx, restricted, []byte("PendingVoterRewardDelegates"))
	r.NoError(err)
	r.Positive(restricted.refused, "fallback must engage only after the range scan is refused")
	r.Equal(want, got)
	r.Equal(wantHeight, gotHeight)

	decoded := &rewardingpb.PendingVoterRewardDelegates{}
	r.NoError(proto.Unmarshal(got, decoded))
	ids := decoded.GetDelegateIdentifiers()
	r.Len(ids, len(withPool), "a candidate without a pool must not be listed")
	for i := 1; i < len(ids); i++ {
		r.Less(bytes.Compare(ids[i-1], ids[i]), 0,
			"fallback enumeration not ascending at position %d: %x vs %x", i, ids[i-1], ids[i])
	}
}

// TestPendingVoterRewardDelegates_ReadWithoutRangeScanEmpty — no pools is an
// empty list, not an error, on the fallback path as well.
func TestPendingVoterRewardDelegates_ReadWithoutRangeScanEmpty(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	restricted := &rangeScanRejectingReader{StateManager: sm}
	data, _, err := p.ReadState(ctx, restricted, []byte("PendingVoterRewardDelegates"))
	r.NoError(err)
	decoded := &rewardingpb.PendingVoterRewardDelegates{}
	r.NoError(proto.Unmarshal(data, decoded))
	r.Empty(decoded.GetDelegateIdentifiers())
}

// TestPendingVoterRewardDelegates_ReadDoesNotMaskOtherErrors — only an
// unsupported scan degrades to point reads. A scan that runs and finds corrupt
// state must still surface, or the read API would paper over real damage.
func TestPendingVoterRewardDelegates_ReadDoesNotMaskOtherErrors(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	malformedKey := append(append([]byte(nil), _pendingBlockRewardPoolKeyPrefix...), 0x01)
	r.NoError(p.putState(ctx, sm, malformedKey, &pendingBlockRewardPool{amount: big.NewInt(1)}))

	_, _, err := p.ReadState(ctx, sm, []byte("PendingVoterRewardDelegates"))
	r.ErrorContains(err, "malformed pending block reward pool key")
}

// TestFreezePendingPoolDrainWork_RangeScanFailureStillHalts — the era freeze is
// consensus, and must keep treating a refused range scan as a hard failure. It
// must never pick up the read-only point-read fallback: enumeration source would
// then depend on a node's storage backend.
func TestFreezePendingPoolDrainWork_RangeScanFailureStillHalts(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	seedPendingPoolDelegates(t, ctx, sm, p,
		[]address.Address{identityset.Address(9), identityset.Address(2)}, identityset.Address(11))

	restricted := &rangeScanRejectingReader{StateManager: sm}

	_, err := p.listPendingBlockRewardPoolIDs(ctx, restricted)
	r.ErrorIs(err, db.ErrNotSupported)

	_, err = p.freezePendingPoolDrainWork(ctx, restricted, iip59FixtureFreezeHeight)
	r.ErrorIs(err, db.ErrNotSupported)
}
