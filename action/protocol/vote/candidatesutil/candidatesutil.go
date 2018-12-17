// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package candidatesutil

import (
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// CandidatesPrefix is the prefix of the key of candidateList
const CandidatesPrefix = "Candidates."

// LoadAndAddCandidates loads candidates from trie and adds a new candidate
func LoadAndAddCandidates(sm protocol.StateManager, vote *action.Vote) error {
	candidateMap, err := GetMostRecentCandidateMap(sm)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	if err := addCandidate(candidateMap, vote, sm.Height()); err != nil {
		return errors.Wrap(err, "failed to add candidate to candidate map")
	}
	return storeCandidates(candidateMap, sm)
}

// LoadAndDeleteCandidates loads candidates from trie and deletes a candidate if exists
func LoadAndDeleteCandidates(sm protocol.StateManager, addr string) error {
	candidateMap, err := GetMostRecentCandidateMap(sm)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	if _, ok := candidateMap[addrHash]; ok {
		delete(candidateMap, addrHash)
	}
	return storeCandidates(candidateMap, sm)
}

// LoadAndUpdateCandidates loads candidates from trie and updates an existing candidate
func LoadAndUpdateCandidates(sm protocol.StateManager, addr string, votingWeight *big.Int) error {
	candidateMap, err := GetMostRecentCandidateMap(sm)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	if err := updateCandidate(candidateMap, addr, votingWeight, sm.Height()); err != nil {
		return errors.Wrapf(err, "failed to update candidate %s", addr)
	}
	return storeCandidates(candidateMap, sm)
}

// GetMostRecentCandidateMap gets the most recent candidateMap from trie
func GetMostRecentCandidateMap(sm protocol.StateManager) (map[hash.PKHash]*state.Candidate, error) {
	var sc state.CandidateList
	for h := int(sm.Height()); h >= 0; h-- {
		candidatesKey := ConstructKey(uint64(h))
		var err error
		if err = sm.State(candidatesKey, &sc); err == nil {
			return state.CandidatesToMap(sc)
		}
		if errors.Cause(err) != state.ErrStateNotExist {
			return nil, errors.Wrap(err, "failed to get most recent state of candidateList")
		}
	}
	if sm.Height() == uint64(0) {
		return make(map[hash.PKHash]*state.Candidate), nil
	}
	return nil, errors.Wrap(state.ErrStateNotExist, "failed to get most recent state of candidateList")
}

// ConstructKey constructs a key for candidates storage
func ConstructKey(height uint64) hash.PKHash {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return byteutil.BytesTo20B(hash.Hash160b(k))
}

// addCandidate adds a new candidate to candidateMap
func addCandidate(candidateMap map[hash.PKHash]*state.Candidate, vote *action.Vote, height uint64) error {
	votePubkey := vote.VoterPublicKey()
	voterPKHash, err := iotxaddress.AddressToPKHash(vote.Voter())
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	if _, ok := candidateMap[voterPKHash]; !ok {
		candidateMap[voterPKHash] = &state.Candidate{
			Address:        vote.Voter(),
			PublicKey:      votePubkey,
			Votes:          big.NewInt(0),
			CreationHeight: height,
		}
	}
	return nil
}

// updateCandidate updates a candidate state
func updateCandidate(
	candiateMap map[hash.PKHash]*state.Candidate,
	addr string,
	totalWeight *big.Int,
	blockHeight uint64,
) error {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := candiateMap[addrHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight

	return nil
}

// storeCandidates puts updated candidates to trie
func storeCandidates(candidateMap map[hash.PKHash]*state.Candidate, sm protocol.StateManager) error {
	candidateList, err := state.MapToCandidates(candidateMap)
	if err != nil {
		return errors.Wrap(err, "failed to convert candidate map to candidate list")
	}
	sort.Sort(candidateList)
	candidatesKey := ConstructKey(sm.Height())
	return sm.PutState(candidatesKey, &candidateList)
}
