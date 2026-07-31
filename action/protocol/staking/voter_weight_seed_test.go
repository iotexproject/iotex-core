// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// seedHarness drives the activation flush against an in-memory store without a
// full state manager: seedVoterWeights' own plumbing is exercised separately, and
// what matters here is that repeated batches converge on the complete table.
type seedHarness struct {
	view   VoterWeightView
	store  *fakeVoterWeightStore
	cursor voterWeightSeedCursor
}

func newSeedHarness(view VoterWeightView) *seedHarness {
	return &seedHarness{view: view, store: newFakeVoterWeightStore()}
}

// step runs one block: flush a batch, then commit whatever is pending — the same
// order CreatePreStates and viewData.Commit run in.
func (h *seedHarness) step(t *testing.T, batch int) bool {
	t.Helper()
	if !h.cursor.Done {
		pairs := h.view.SeedPairsAfter(h.cursor.position(), batch)
		for _, ref := range pairs {
			h.view.MarkForRewrite(ref.cand, ref.voter)
		}
		if len(pairs) > 0 {
			last := pairs[len(pairs)-1]
			h.cursor.LastCand, h.cursor.LastVoter = last.cand, last.voter
			h.cursor.Started = true
		}
		if batch < 0 || len(pairs) < batch {
			h.cursor.Done = true
		}
	}
	base := h.view.(*voterWeightBase)
	require.NoError(t, base.persist(h.store))
	base.dirty = false
	base.touched = nil
	return h.cursor.Done
}

// TestSeedFlushConvergesRegardlessOfBatchSize is the property the flush rests
// on: the batch size is an operational knob, not part of the answer. Any batch
// size must produce the same committed table as writing everything at once.
func TestSeedFlushConvergesRegardlessOfBatchSize(t *testing.T) {
	r := require.New(t)

	build := func() VoterWeightView {
		v := NewVoterWeightView()
		for c := 0; c < 4; c++ {
			for i := 0; i < 9; i++ {
				v.Apply(candID(20+c), identityset.Address(i), big.NewInt(int64(100+10*c+i)))
			}
		}
		return v
	}
	expected := build().Hash()

	for _, batch := range []int{1, 2, 5, 7, 36, 1000, -1} {
		t.Run(fmt.Sprintf("batch_%d", batch), func(t *testing.T) {
			h := newSeedHarness(build())
			for i := 0; i < 200; i++ {
				if h.step(t, batch) {
					break
				}
			}
			r.True(h.cursor.Done, "flush must terminate for batch %d", batch)

			h.store.markSeedComplete()
			loaded, err := loadVoterWeightView(h.store.reader(t), nil, nil, testVoteWeightConsts())
			r.NoError(err)
			r.Equal(expected, loaded.Hash(), "batch %d produced a different table", batch)
		})
	}
}

// TestSeedFlushWithConcurrentMutation is the case the whole design turns on.
//
// Voters keep staking and unstaking while the flush is running. Nothing here
// tracks which side of the cursor a bucket falls on, and nothing needs to:
// Commit writes each touched pair's *absolute* weight, so a pair the block
// mutates is written correctly whether or not the cursor has reached it, and the
// cursor writing it again later is idempotent.
func TestSeedFlushWithConcurrentMutation(t *testing.T) {
	r := require.New(t)

	prev := voterWeightAnomalyFatal
	voterWeightAnomalyFatal = true
	defer func() { voterWeightAnomalyFatal = prev }()

	for seed := int64(0); seed < 20; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			v := NewVoterWeightView()
			// A starting table big enough that a batch of 3 spans many blocks.
			for c := 0; c < 3; c++ {
				for i := 0; i < 8; i++ {
					v.Apply(candID(20+c), identityset.Address(i), big.NewInt(int64(500+i)))
				}
			}
			h := newSeedHarness(v)

			for step := 0; step < 60; step++ {
				done := h.step(t, 3)

				// Mutate between blocks: new voters, top-ups, partial
				// withdrawals and full exits. Subtractions are always bounded by
				// an existing weight — an unbacked negative delta is a bug in
				// the caller, not a scenario, and the anomaly detector treats it
				// as one.
				switch rng.Intn(4) {
				case 0:
					v.Apply(candID(20+rng.Intn(3)), identityset.Address(rng.Intn(12)), big.NewInt(int64(1+rng.Intn(50))))
				case 1, 2:
					pairs := v.SeedPairsAfter(nil, -1)
					if len(pairs) == 0 {
						break
					}
					ref := pairs[rng.Intn(len(pairs))]
					voter := addrOf(t, ref.voter)
					w := findWeight(v.VoterWeightsByCandidate(ref.cand), voter)
					if w == nil {
						break
					}
					amount := w // full exit, driving the pair to exactly zero
					if rng.Intn(2) == 0 && w.Int64() > 1 {
						amount = big.NewInt(1 + rng.Int63n(w.Int64()-1)) // partial
					}
					v.Apply(ref.cand, voter, new(big.Int).Neg(amount))
				}

				if done && step > 40 {
					break
				}
			}
			// One final block so the last mutations are committed.
			h.step(t, 3)

			h.store.markSeedComplete()
			loaded, err := loadVoterWeightView(h.store.reader(t), nil, nil, testVoteWeightConsts())
			r.NoError(err)
			r.Equal(v.Hash(), loaded.Hash(), "committed table diverged from the live view")
		})
	}
}

// TestSeedCursorRoundTrip covers the cursor's own serialization, since it is
// consensus state and a mis-parse would resume the flush at the wrong place.
func TestSeedCursorRoundTrip(t *testing.T) {
	r := require.New(t)
	for _, c := range []voterWeightSeedCursor{
		{},
		{Started: true, LastCand: candID(3), LastVoter: candID(4)},
		{Started: true, Done: true, LastCand: candID(1), LastVoter: candID(2), DoneHeight: 987654},
	} {
		data, err := c.Serialize()
		r.NoError(err)
		r.Len(data, _voterWeightSeedCursorLen)

		var out voterWeightSeedCursor
		r.NoError(out.Deserialize(data))
		r.Equal(c, out)
	}

	r.Error((&voterWeightSeedCursor{}).Deserialize(make([]byte, _voterWeightSeedCursorLen-1)))
}

// TestSeedPairsAfterOrdering pins the walk order the cursor's position depends
// on: a resumed batch must continue where the previous one stopped, with no gap
// and no repeat.
func TestSeedPairsAfterOrdering(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	for c := 0; c < 3; c++ {
		for i := 0; i < 5; i++ {
			v.Apply(candID(20+c), identityset.Address(i), big.NewInt(10))
		}
	}

	all := v.SeedPairsAfter(nil, -1)
	r.Len(all, 15)
	for i := 1; i < len(all); i++ {
		r.True(refLess(all[i-1], all[i]), "walk must be strictly ascending at %d", i)
	}

	// Walking in steps of 4 must reproduce the same sequence exactly.
	var walked []voterWeightRef
	var pos *voterWeightRef
	for {
		batch := v.SeedPairsAfter(pos, 4)
		if len(batch) == 0 {
			break
		}
		walked = append(walked, batch...)
		last := batch[len(batch)-1]
		pos = &last
	}
	r.Equal(all, walked)
}

func addrOf(t *testing.T, id hash.Hash160) address.Address {
	t.Helper()
	a, err := address.FromBytes(id[:])
	require.NoError(t, err)
	return a
}

func testVoteWeightConsts() genesis.VoteWeightCalConsts {
	return genesis.TestDefault().Staking.VoteWeightCalConsts
}
