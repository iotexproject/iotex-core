// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package candidatesutil

import (
	"go.uber.org/zap"
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// CandidatesPrefix is the prefix of the key of candidateList
const CandidatesPrefix = "Candidates."

// KickoutPrefix is the prefix of the key of blackList for kick-out
const KickoutPrefix = "KickoutList."

// CandidatesByHeight returns array of Candidates in candidate pool of a given height
func CandidatesByHeight(sr protocol.StateReader, height uint64) ([]*state.Candidate, error) {
	var candidates state.CandidateList
	// Load Candidates on the given height from underlying db
	candidatesKey := ConstructKey(height)
	err := sr.State(candidatesKey, &candidates)
	log.L().Debug(
		"CandidatesByHeight",
		zap.Uint64("height", height),
		zap.Any("candidates", candidates),
		zap.Error(err),
	)
	if errors.Cause(err) == nil {
		if len(candidates) > 0 {
			return candidates, nil
		}
		err = state.ErrStateNotExist
	}
	return nil, errors.Wrapf(
		err,
		"failed to get state of candidateList for height %d",
		height,
	)
}

// KickoutListByEpoch returns array of unqualified delegate address in delegate pool for the given epochNum
func KickoutListByEpoch(sr protocol.StateReader, epochNum uint64) (*vote.Blacklist, error) {
	blackList := &vote.Blacklist{}
	// Load kick out list on the given epochNum from underlying db
	blackListKey := ConstructBlackListKey(epochNum)
	err := sr.State(blackListKey, blackList)
	log.L().Debug(
		"KickoutListByEpoch",
		zap.Uint64("epoch number", epochNum),
		zap.Any("kick out list ", blackList),
		zap.Error(err),
	)
	if err == nil {
		return blackList, nil
	}
	return nil, errors.Wrapf(
		err,
		"failed to get state of kick-out list for epoch number %d",
		epochNum,
	)
}

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

// ConstructBlackListKey constructs a key for kick-out blacklist storage
func ConstructBlackListKey(epochNum uint64) hash.Hash160 {
	epochInBytes := byteutil.Uint64ToBytes(epochNum)
	k := []byte(KickoutPrefix)
	k = append(k, epochInBytes...)
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
