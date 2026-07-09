// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"sync"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// TestActPool_popLowestPriorityAcrossWorkers verifies the eviction victim is
// chosen from the union of every worker shard, not just the sender's shard.
// Per-shard PopPeek lets an attacker who fills one shard force honest users in
// every other shard to evict each other (~94% censorship). The fix must pick
// the globally lowest-priority head action regardless of which shard runs
// eviction.
func TestActPool_popLowestPriorityAcrossWorkers(t *testing.T) {
	r := require.New(t)

	// Find two identityset keys that land in different shards so the test
	// genuinely exercises cross-shard selection. _numWorker = 16.
	ap := &actPool{worker: make([]*queueWorker, _numWorker)}
	for i := range ap.worker {
		ap.worker[i] = &queueWorker{ap: ap, accountActs: newAccountPool()}
	}
	var (
		idxA, idxB int = -1, -1
	)
	for i := 0; i < identityset.Size(); i++ {
		for j := i + 1; j < identityset.Size(); j++ {
			if ap.allocatedWorker(identityset.Address(i)) != ap.allocatedWorker(identityset.Address(j)) {
				idxA, idxB = i, j
				break
			}
		}
		if idxA >= 0 {
			break
		}
	}
	r.GreaterOrEqual(idxA, 0, "need two senders in different shards")

	pkA, pkB := identityset.PrivateKey(idxA), identityset.PrivateKey(idxB)
	addrA, addrB := pkA.PublicKey().Address(), pkB.PublicKey().Address()
	wA := ap.worker[ap.allocatedWorker(addrA)]
	wB := ap.worker[ap.allocatedWorker(addrB)]
	r.NotSame(wA, wB)

	// Shard A holds an expensive action; shard B holds a cheap one. A global
	// eviction must pop from shard B regardless of which shard is "the
	// caller". With per-shard PopPeek the attacker (shard A) would never lose
	// its action; with the fix, B's cheap action is the victim.
	expensive, err := action.SignedTransfer(addrB.String(), pkA, 1, big.NewInt(1), nil, uint64(0), big.NewInt(10))
	r.NoError(err)
	cheap, err := action.SignedTransfer(addrA.String(), pkB, 1, big.NewInt(1), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	r.NoError(wA.accountActs.PutAction(addrA.String(), ap, 1, _balance, _expireTime, expensive))
	r.NoError(wB.accountActs.PutAction(addrB.String(), ap, 1, _balance, _expireTime, cheap))

	evicted := ap.popLowestPriorityAcrossWorkers()
	r.Equal(cheap, evicted, "expected globally lowest-priced action to be evicted")

	// Shard A must still hold its expensive action; only shard B's account
	// was drained. This is the property that prevents cross-shard censorship.
	queueA := wA.accountActs.Account(addrA.String())
	r.NotNil(queueA)
	r.False(queueA.Empty())
	queueB := wB.accountActs.Account(addrB.String())
	r.NotNil(queueB)
	r.True(queueB.Empty())
}

// TestActPool_popLowestPriorityAcrossWorkers_empty exercises the no-op path:
// with every shard empty there is nothing to evict and the pop must return
// nil rather than panic on an empty priorityQueue.
func TestActPool_popLowestPriorityAcrossWorkers_empty(t *testing.T) {
	r := require.New(t)
	ap := &actPool{worker: make([]*queueWorker, _numWorker)}
	for i := range ap.worker {
		ap.worker[i] = &queueWorker{ap: ap, accountActs: newAccountPool()}
	}
	r.Nil(ap.popLowestPriorityAcrossWorkers())
}

// TestHeadLess pins the cross-shard eviction ordering against
// accountPriorityQueue.Less so a future "simplification" of either function
// can't silently diverge from the global heap semantics.
func TestHeadLess(t *testing.T) {
	r := require.New(t)
	gpLow := big.NewInt(1)
	gpHigh := big.NewInt(100)
	cases := []struct {
		name                                       string
		aHasNext, aSettled                         bool
		aGP                                        *big.Int
		bHasNext, bSettled                         bool
		bGP                                        *big.Int
		want                                       bool
		description                                string
	}{
		// nil-next handling mirrors Less: jgp==nil → true; igp==nil (b non-nil) → false.
		{"b has no next", true, true, gpLow, false, false, nil, true, "b has no next: a wins eviction"},
		{"a has no next", false, false, nil, true, true, gpHigh, false, "a has no next, b non-nil: b is preferred head"},
		{"both empty (matches Less asymmetry)", false, false, nil, false, false, nil, true, "both empty: first nil-check fires, a wins"},
		// settled/unsettled axis (with non-nil gp on both sides)
		{"a unsettled vs b settled", true, false, gpHigh, true, true, gpLow, true, "unsettled is evicted before settled even at higher gas"},
		{"a settled vs b unsettled", true, true, gpLow, true, false, gpHigh, false, "settled stays even with lower gas vs unsettled"},
		// gas axis (both unsettled)
		{"both unsettled, a cheaper", true, false, gpLow, true, false, gpHigh, true, "lower gas wins eviction among unsettled"},
		{"both unsettled, a pricier", true, false, gpHigh, true, false, gpLow, false, "higher gas survives among unsettled"},
		// gas axis (both settled)
		{"both settled, a cheaper", true, true, gpLow, true, true, gpHigh, true, "lower gas wins eviction among settled"},
		{"both settled, a pricier", true, true, gpHigh, true, true, gpLow, false, "higher gas survives among settled"},
		// equal gas: Cmp == 0 → !(<0) → false (not less)
		{"equal gas same settled state", true, true, gpLow, true, true, big.NewInt(1), false, "ties resolve to false (matches Less strict-less)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.Equal(tc.want,
				headLess(tc.aHasNext, tc.aSettled, tc.aGP, tc.bHasNext, tc.bSettled, tc.bGP),
				tc.description)
		})
	}
}

// TestActPool_crossShardEviction_e2e drives the full Add path: it fills the
// pool with low-fee actions in shard A and then submits a higher-fee action
// from shard B. Under the old per-shard PopPeek the shard-B sender would have
// been forced to evict its own action (cross-shard censorship). With the
// global eviction the low-fee shard-A action must be the victim.
func TestActPool_crossShardEviction_e2e(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)

	// Find two identityset keys in different shards.
	idxA, idxB := -1, -1
	probe := &actPool{}
	for i := 0; i < identityset.Size() && idxA < 0; i++ {
		for j := i + 1; j < identityset.Size(); j++ {
			if probeShard(probe, identityset.Address(i)) != probeShard(probe, identityset.Address(j)) {
				idxA, idxB = i, j
				break
			}
		}
	}
	r.GreaterOrEqual(idxA, 0)
	pkA, pkB := identityset.PrivateKey(idxA), identityset.PrivateKey(idxB)
	addrA, addrB := pkA.PublicKey().Address(), pkB.PublicKey().Address()

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(big.NewInt(1_000_000_000)))
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	cfg.MaxNumActsPerPool = 2 // small so the second add already triggers eviction
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())

	// Two low-fee actions in shard A fill the pool.
	cheap1, err := action.SignedTransfer(addrB.String(), pkA, 1, big.NewInt(1), nil, uint64(100000), big.NewInt(1))
	r.NoError(err)
	cheap2, err := action.SignedTransfer(addrB.String(), pkA, 2, big.NewInt(1), nil, uint64(100000), big.NewInt(1))
	r.NoError(err)
	r.NoError(ap.Add(ctx, cheap1))
	r.NoError(ap.Add(ctx, cheap2))
	r.Equal(uint64(2), ap.GetSize())

	// High-fee shard-B action: the old per-shard eviction would have rejected
	// it because shard B was empty (PopPeek from B returns nil → no slot freed).
	// The fix must evict one of the cheap shard-A actions instead and let the
	// expensive shard-B action in.
	expensive, err := action.SignedTransfer(addrA.String(), pkB, 1, big.NewInt(1), nil, uint64(100000), big.NewInt(100))
	r.NoError(err)
	r.NoError(ap.Add(ctx, expensive))

	// Pool size respected.
	r.LessOrEqual(ap.GetSize(), uint64(cfg.MaxNumActsPerPool))

	// Expensive action must still be in the pool.
	xHash, err := expensive.Hash()
	r.NoError(err)
	got, err := ap.GetActionByHash(xHash)
	r.NoError(err)
	r.Equal(expensive.SenderAddress().String(), got.SenderAddress().String())

	// Exactly one of the cheap actions must have been evicted.
	c1Hash, _ := cheap1.Hash()
	c2Hash, _ := cheap2.Hash()
	_, err1 := ap.GetActionByHash(c1Hash)
	_, err2 := ap.GetActionByHash(c2Hash)
	evicted := 0
	if errors.Is(errors.Cause(err1), action.ErrNotFound) {
		evicted++
	}
	if errors.Is(errors.Cause(err2), action.ErrNotFound) {
		evicted++
	}
	r.Equal(1, evicted, "exactly one shard-A cheap action should have been evicted to make room for the cross-shard expensive one")
}

// TestActPool_crossShardEviction_rejectsLowerFeeNewcomer covers the
// ErrTxPoolOverflow branch: when the incoming action is itself the
// globally-lowest priority, the global pop selects it and the Add must fail
// with ErrTxPoolOverflow rather than silently evicting an honest user.
func TestActPool_crossShardEviction_rejectsLowerFeeNewcomer(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)

	idxA, idxB := -1, -1
	probe := &actPool{}
	for i := 0; i < identityset.Size() && idxA < 0; i++ {
		for j := i + 1; j < identityset.Size(); j++ {
			if probeShard(probe, identityset.Address(i)) != probeShard(probe, identityset.Address(j)) {
				idxA, idxB = i, j
				break
			}
		}
	}
	r.GreaterOrEqual(idxA, 0)
	pkA, pkB := identityset.PrivateKey(idxA), identityset.PrivateKey(idxB)
	addrA, addrB := pkA.PublicKey().Address(), pkB.PublicKey().Address()

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(big.NewInt(1_000_000_000)))
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	cfg.MaxNumActsPerPool = 2
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())

	// Fill with two high-fee actions in shard A.
	hi1, err := action.SignedTransfer(addrB.String(), pkA, 1, big.NewInt(1), nil, uint64(100000), big.NewInt(100))
	r.NoError(err)
	hi2, err := action.SignedTransfer(addrB.String(), pkA, 2, big.NewInt(1), nil, uint64(100000), big.NewInt(100))
	r.NoError(err)
	r.NoError(ap.Add(ctx, hi1))
	r.NoError(ap.Add(ctx, hi2))

	// Newcomer (shard B) tries to enter with a *lower* gas price. It is the
	// globally lowest priority head, so the global pop should select it and
	// the Add must fail. Both incumbent actions must survive.
	lowFee, err := action.SignedTransfer(addrA.String(), pkB, 1, big.NewInt(1), nil, uint64(100000), big.NewInt(1))
	r.NoError(err)
	err = ap.Add(ctx, lowFee)
	r.ErrorIs(errors.Cause(err), action.ErrTxPoolOverflow)

	h1, _ := hi1.Hash()
	h2, _ := hi2.Hash()
	_, err = ap.GetActionByHash(h1)
	r.NoError(err)
	_, err = ap.GetActionByHash(h2)
	r.NoError(err)
}

// TestActPool_crossShardEviction_concurrent stresses the multi-lock global
// pop under load. It is primarily a deadlock smoke test: many goroutines
// concurrently exercise Add across all 16 shards. With -race, any lock-order
// inversion would also surface here.
func TestActPool_crossShardEviction_concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	ctrl := gomock.NewController(t)
	r := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(big.NewInt(1_000_000_000)))
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	cfg.MaxNumActsPerPool = 8 // tight so eviction fires often
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())

	// One sender per identityset entry so the workload spans many shards.
	const goroutines = 16
	const perG = 50
	wg := sync.WaitGroup{}
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for k := 0; k < perG; k++ {
				pk := identityset.PrivateKey((g*perG + k) % identityset.Size())
				tsf, err := action.SignedTransfer(pk.PublicKey().Address().String(), pk, uint64(k%4+1), big.NewInt(1), nil, uint64(100000), big.NewInt(int64(k%5+1)))
				if err != nil {
					continue
				}
				_ = ap.Add(ctx, tsf) // ignore individual outcomes; we only care that the pool doesn't deadlock
			}
		}(g)
	}
	wg.Wait()
	// If the test reached this line at all, the global eviction did not
	// deadlock under concurrent multi-shard load. (Strict capacity equality is
	// not asserted because the pool's global full-check is read-then-act and
	// can temporarily overshoot under concurrency — that is pre-existing
	// behavior unrelated to the cross-shard eviction change.)
	r.Greater(ap.GetSize(), uint64(0), "pool should hold some actions after the workload")
}

// probeShard exposes allocatedWorker for test setup (which needs to know the
// shard of an address before any worker exists).
func probeShard(_ *actPool, addr address.Address) int {
	b := addr.Bytes()
	return int(b[len(b)-1]) % _numWorker
}
