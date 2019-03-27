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

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// CandidatesPrefix is the prefix of the key of candidateList
const CandidatesPrefix = "Candidates."

// LoadAndAddCandidates loads candidates from trie and adds a new candidate
func LoadAndAddCandidates(sm protocol.StateManager, blkHeight uint64, addr string) error {
	candidateMap, err := GetMostRecentCandidateMap(sm, blkHeight)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	if err := addCandidate(candidateMap, addr); err != nil {
		return errors.Wrap(err, "failed to add candidate to candidate map")
	}
	return storeCandidates(candidateMap, sm, blkHeight)
}

// LoadAndDeleteCandidates loads candidates from trie and deletes a candidate if exists
func LoadAndDeleteCandidates(sm protocol.StateManager, blkHeight uint64, encodedAddr string) error {
	candidateMap, err := GetMostRecentCandidateMap(sm, blkHeight)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	if _, ok := candidateMap[addrHash]; ok {
		delete(candidateMap, addrHash)
	}
	return storeCandidates(candidateMap, sm, blkHeight)
}

// LoadAndUpdateCandidates loads candidates from trie and updates an existing candidate
func LoadAndUpdateCandidates(sm protocol.StateManager, blkHeight uint64, addr string, votingWeight *big.Int) error {
	candidateMap, err := GetMostRecentCandidateMap(sm, blkHeight)
	if err != nil {
		return errors.Wrap(err, "failed to get most recent candidates from trie")
	}
	if err := updateCandidate(candidateMap, addr, votingWeight, blkHeight); err != nil {
		return errors.Wrapf(err, "failed to update candidate %s", addr)
	}
	return storeCandidates(candidateMap, sm, blkHeight)
}

// GetMostRecentCandidateMap gets the most recent candidateMap from trie
func GetMostRecentCandidateMap(sm protocol.StateManager, blkHeight uint64) (map[hash.Hash160]*state.Candidate, error) {
	var sc state.CandidateList
	for h := int(blkHeight); h >= 0; h-- {
		candidatesKey := ConstructKey(uint64(h))
		var err error
		if err = sm.State(candidatesKey, &sc); err == nil {
			return state.CandidatesToMap(sc)
		}
		if errors.Cause(err) != state.ErrStateNotExist {
			return nil, errors.Wrap(err, "failed to get most recent state of candidateList")
		}
	}
	if blkHeight == uint64(0) || blkHeight == uint64(1) {
		return make(map[hash.Hash160]*state.Candidate), nil
	}
	return nil, errors.Wrap(state.ErrStateNotExist, "failed to get most recent state of candidateList")
}

// ConstructKey constructs a key for candidates storage
func ConstructKey(height uint64) hash.Hash160 {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return hash.Hash160b(k)
}

// addCandidate adds a new candidate to candidateMap
func addCandidate(candidateMap map[hash.Hash160]*state.Candidate, encodedAddr string) error {
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	if _, ok := candidateMap[addrHash]; !ok {
		candidateMap[addrHash] = &state.Candidate{
			Address: encodedAddr,
			Votes:   big.NewInt(0),
		}
	}
	return nil
}

// updateCandidate updates a candidate state
func updateCandidate(
	candidateMap map[hash.Hash160]*state.Candidate,
	encodedAddr string,
	totalWeight *big.Int,
	blockHeight uint64,
) error {
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := candidateMap[addrHash]
	candidate.Votes = totalWeight

	return nil
}

// storeCandidates puts updated candidates to trie
func storeCandidates(candidateMap map[hash.Hash160]*state.Candidate, sm protocol.StateManager, blkHeight uint64) error {
	candidateList, err := state.MapToCandidates(candidateMap)
	if err != nil {
		return errors.Wrap(err, "failed to convert candidate map to candidate list")
	}
	sort.Sort(candidateList)
	candidatesKey := ConstructKey(blkHeight)
	return sm.PutState(candidatesKey, &candidateList)
}
