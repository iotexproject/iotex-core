// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"context"
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// VoteSizeLimit is the maximum size of vote allowed
	VoteSizeLimit = 278
	// CandidatesPrefix is the prefix of the key of candidateList
	CandidatesPrefix = "Candidates."
)

// Protocol defines the protocol of handling votes
type Protocol struct {
	cm               protocol.ChainManager
	cachedCandidates map[hash.PKHash]*state.Candidate
}

// NewProtocol instantiates the protocol of vote
func NewProtocol(cm protocol.ChainManager) *Protocol { return &Protocol{cm: cm} }

// Handle handles a vote
func (p *Protocol) Handle(_ context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil, nil
	}
	// Get candidateList from trie and convert it to candidates map
	candidateMap, err := p.getCandidateMap(sm.Height(), sm)
	switch {
	case errors.Cause(err) == state.ErrStateNotExist:
		if sm.Height() == uint64(0) {
			candidateMap = make(map[hash.PKHash]*state.Candidate)
		} else if candidateMap, err = p.getCandidateMap(sm.Height()-1, sm); err != nil {
			return nil, errors.Wrapf(err, "failed to get candidates on height %d from trie", sm.Height()-1)
		}
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get candidates on height %d from trie", sm.Height())
	}

	p.cachedCandidates = candidateMap

	voteFrom, err := account.LoadOrCreateAccountState(sm, vote.Voter(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of voter %s", vote.Voter())
	}
	// Update voteFrom Nonce
	account.SetNonce(vote, voteFrom)
	prevVotee := voteFrom.Votee
	voteFrom.Votee = vote.Votee()
	if vote.Votee() == "" {
		// unvote operation
		voteFrom.IsCandidate = false
		// Remove the candidate from candidateMap if the person is not a candidate anymore
		addrHash, err := iotxaddress.AddressToPKHash(vote.Voter())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert address to public key hash")
		}
		if _, ok := candidateMap[addrHash]; ok {
			delete(candidateMap, addrHash)
		}
	} else if vote.Voter() == vote.Votee() {
		// Vote to self: self-nomination
		voteFrom.IsCandidate = true
		if err := p.addCandidate(vote, sm.Height()); err != nil {
			return nil, errors.Wrap(err, "failed to add candidate to candidate map")
		}
	}
	// Put updated voter's state to trie
	if err := account.StoreState(sm, vote.Voter(), voteFrom); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	// Update old votee's weight
	if len(prevVotee) > 0 {
		// voter already voted
		oldVotee, err := account.LoadOrCreateAccountState(sm, prevVotee, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of voter's old votee %s", prevVotee)
		}
		oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
		// Put updated state of voter's old votee to trie
		if err := account.StoreState(sm, prevVotee, oldVotee); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if oldVotee.IsCandidate {
			p.updateCandidate(prevVotee, oldVotee.VotingWeight, sm.Height())
		}
	}

	if vote.Votee() != "" {
		voteTo, err := account.LoadOrCreateAccountState(sm, vote.Votee(), big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of votee %s", vote.Votee())
		}
		// Update new votee's weight
		voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)

		// Put updated votee's state to trie
		if err := account.StoreState(sm, vote.Votee(), voteTo); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if voteTo.IsCandidate {
			p.updateCandidate(vote.Votee(), voteTo.VotingWeight, sm.Height())
		}
	}

	// Put updated candidate map to trie
	candidateList, err := state.MapToCandidates(candidateMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert candidate map to candidate list")
	}
	sort.Sort(candidateList)
	candidatesKey := p.constructKey(sm.Height())
	if err := sm.PutState(candidatesKey, &candidateList); err != nil {
		return nil, errors.Wrap(err, "failed to put updated candidates to trie")
	}

	// clear cached candidates
	p.cachedCandidates = nil
	return nil, nil
}

// Validate validates a vote
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil
	}
	// Reject oversized vote
	if vote.TotalSize() > VoteSizeLimit {
		return errors.Wrapf(action.ErrActPool, "oversized data")
	}
	// check if votee's address is valid
	if vote.Votee() != action.EmptyAddress {
		if _, err := iotxaddress.GetPubkeyHash(vote.Votee()); err != nil {
			return errors.Wrapf(err, "error when validating votee's address %s", vote.Votee())
		}
	}
	if vote.Votee() != "" {
		// Reject vote if votee is not a candidate
		voteeState, err := p.cm.StateByAddr(vote.Votee())
		if err != nil {
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.Votee())
		}
		if vote.Voter() != vote.Votee() && !voteeState.IsCandidate {
			return errors.Wrapf(action.ErrVotee, "votee has not self-nominated: %s", vote.Votee())
		}
	}
	return nil
}

func (p *Protocol) constructKey(height uint64) hash.PKHash {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return byteutil.BytesTo20B(hash.Hash160b(k))
}

func (p *Protocol) getCandidateMap(height uint64, sm protocol.StateManager) (map[hash.PKHash]*state.Candidate, error) {
	candidatesKey := p.constructKey(height)
	var sc state.CandidateList
	if err := sm.State(candidatesKey, &sc); err != nil {
		return nil, err
	}
	return state.CandidatesToMap(sc)
}

func (p *Protocol) addCandidate(vote *action.Vote, height uint64) error {
	votePubkey := vote.VoterPublicKey()
	voterPKHash, err := iotxaddress.AddressToPKHash(vote.Voter())
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	if _, ok := p.cachedCandidates[voterPKHash]; !ok {
		p.cachedCandidates[voterPKHash] = &state.Candidate{
			Address:        vote.Voter(),
			PublicKey:      votePubkey,
			CreationHeight: height,
		}
	}
	return nil
}

func (p *Protocol) updateCandidate(
	addr string,
	totalWeight *big.Int,
	blockHeight uint64,
) error {
	pkHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := p.cachedCandidates[pkHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight

	return nil
}
