// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/state"
)

func TestStakingCommittee_FilterCandidates(t *testing.T) {
	r := require.New(t)
	sc := &stakingCommittee{scoreThreshold: big.NewInt(100)}

	candidates := state.CandidateList{
		{Address: "a", Votes: big.NewInt(50)},   // below threshold, dropped
		{Address: "b", Votes: big.NewInt(100)},  // equal, kept
		{Address: "c", Votes: big.NewInt(1000)}, // above, kept
	}
	got := sc.filterCandidates(candidates)
	r.Len(got, 2)
	addrs := map[string]bool{}
	for _, c := range got {
		addrs[c.Address] = true
	}
	r.True(addrs["b"])
	r.True(addrs["c"])
	r.False(addrs["a"])
}

func TestStakingCommittee_FilterCandidatesEmpty(t *testing.T) {
	r := require.New(t)
	sc := &stakingCommittee{scoreThreshold: big.NewInt(100)}
	// everything below threshold => empty result
	got := sc.filterCandidates(state.CandidateList{
		{Address: "a", Votes: big.NewInt(1)},
	})
	r.Empty(got)
}

func TestStakingCommittee_MergeCandidates(t *testing.T) {
	r := require.New(t)
	sc := &stakingCommittee{scoreThreshold: big.NewInt(100)}

	name1 := []byte("cand00000001")
	name2 := []byte("cand00000002")
	list := state.CandidateList{
		{Address: "addr1", Votes: big.NewInt(60), CanName: name1},
		{Address: "addr2", Votes: big.NewInt(200), CanName: name2},
	}
	// native votes push addr1 over the threshold (60+50=110), addr2 unaffected
	votes := &VoteTally{
		Candidates: map[[12]byte]*state.Candidate{
			to12Bytes(name1): {Votes: big.NewInt(50)},
		},
	}
	merged := sc.mergeCandidates(list, votes, time.Unix(1000, 0))
	r.Len(merged, 2)

	byAddr := map[string]*big.Int{}
	for _, c := range merged {
		byAddr[c.Address] = c.Votes
	}
	r.Equal(int64(110), byAddr["addr1"].Int64())
	r.Equal(int64(200), byAddr["addr2"].Int64())
}

func TestStakingCommittee_MergeCandidatesDropsBelowThreshold(t *testing.T) {
	r := require.New(t)
	sc := &stakingCommittee{scoreThreshold: big.NewInt(100)}

	name1 := []byte("cand00000001")
	list := state.CandidateList{
		{Address: "addr1", Votes: big.NewInt(60), CanName: name1},
	}
	// no native votes, stays below threshold => dropped
	merged := sc.mergeCandidates(list, &VoteTally{Candidates: map[[12]byte]*state.Candidate{}}, time.Unix(1000, 0))
	r.Empty(merged)
}

// mergeCandidates must not mutate the input candidate's vote count (it clones).
func TestStakingCommittee_MergeCandidatesDoesNotMutateInput(t *testing.T) {
	r := require.New(t)
	sc := &stakingCommittee{scoreThreshold: big.NewInt(0)}
	name1 := []byte("cand00000001")
	original := big.NewInt(60)
	list := state.CandidateList{
		{Address: "addr1", Votes: original, CanName: name1},
	}
	votes := &VoteTally{
		Candidates: map[[12]byte]*state.Candidate{
			to12Bytes(name1): {Votes: big.NewInt(50)},
		},
	}
	sc.mergeCandidates(list, votes, time.Unix(1000, 0))
	r.Equal(int64(60), original.Int64(), "input candidate votes should be unchanged")
}
