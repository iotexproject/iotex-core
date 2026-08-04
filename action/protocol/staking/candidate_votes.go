// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
)

// addCandidateVotes credits weight to a candidate's aggregate vote total.
//
// Every site that changes a candidate's votes on behalf of a bucket owner must
// go through this or subCandidateVotes rather than calling Candidate.AddVote /
// Candidate.SubVote directly. TestCandidateVoteMutationsUseChokePoint fails the
// build if a new call site bypasses them.
//
// The choke point outlived the retired per-(candidate, voter) VoterWeightView it
// was originally introduced for. Candidate.Votes is now the quantity an era
// boundary freezes as TotalWeight -- the denominator every voter's reward share
// is divided by -- so a mutation that skips this funnel no longer drifts a
// second derivation, it moves the payout denominator itself. Keeping one funnel
// is what makes TestVoterWeightInvariant a statement about all of them.
//
// The error is returned unwrapped so callers keep their own receipt failure
// status, which differs between the add and sub directions at several sites.
func addCandidateVotes(cand *Candidate, weight *big.Int) error {
	return cand.AddVote(weight)
}

// subCandidateVotes debits weight from a candidate's aggregate vote total.
// See addCandidateVotes.
func subCandidateVotes(cand *Candidate, weight *big.Int) error {
	return cand.SubVote(weight)
}
