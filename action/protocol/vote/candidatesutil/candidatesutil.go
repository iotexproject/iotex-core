// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package candidatesutil

import (
	"math/big"
	"sort"

	"go.uber.org/zap"

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

// CurCandidateKey is the key of current candidate list
const CurCandidateKey = "CurrentCandidateList."

// NxtCandidateKey is the key of next candidate list
const NxtCandidateKey = "NextCandidateList."

// CurKickoutKey is the key of current kickout list
const CurKickoutKey = "CurrentKickoutKey."

// NxtKickoutKey is the key of next kickout list
const NxtKickoutKey = "NextKickoutKey."

// UnproductiveDelegateKey is the key of unproductive Delegate struct
const UnproductiveDelegateKey = "UnproductiveDelegateKey."

// CandidatesByHeight returns array of Candidates in candidate pool of a given height (deprecated version)
func CandidatesByHeight(sr protocol.StateReader, height uint64) ([]*state.Candidate, error) {
	var candidates state.CandidateList
	// Load Candidates on the given height from underlying db
	candidatesKey := ConstructLegacyKey(height)
	_, err := sr.State(&candidates, protocol.LegacyKeyOption(candidatesKey))
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

// CandidatesFromDB returns array of Candidates at current epoch
func CandidatesFromDB(sr protocol.StateReader, epochStartPoint bool) ([]*state.Candidate, uint64, error) {
	var candidates state.CandidateList
	candidatesKey := ConstructKey(CurCandidateKey)
	if epochStartPoint {
		// if not shifted yet
		candidatesKey = ConstructKey(NxtCandidateKey)
	}
	stateHeight, err := sr.State(
		&candidates,
		protocol.KeyOption(candidatesKey[:]),
		protocol.NamespaceOption(protocol.SystemNamespace),
	)
	log.L().Debug(
		"GetCandidates",
		zap.Any("candidates", candidates),
		zap.Uint64("state height", stateHeight),
		zap.Error(err),
	)
	if errors.Cause(err) == nil {
		if len(candidates) > 0 {
			return candidates, stateHeight, nil
		}
		err = state.ErrStateNotExist
	}
	return nil, stateHeight, errors.Wrapf(
		err,
		"failed to get candidates with epochStartEpoch: %t",
		epochStartPoint,
	)
}

// KickoutListFromDB returns array of kickout list at current epoch
func KickoutListFromDB(sr protocol.StateReader, epochStartPoint bool) (*vote.Blacklist, uint64, error) {
	blackList := &vote.Blacklist{}
	blackListKey := ConstructKey(CurKickoutKey)
	if epochStartPoint {
		// if not shifted yet
		blackListKey = ConstructKey(NxtKickoutKey)
	}
	stateHeight, err := sr.State(
		blackList,
		protocol.KeyOption(blackListKey[:]),
		protocol.NamespaceOption(protocol.SystemNamespace),
	)
	log.L().Debug(
		"GetKickoutList",
		zap.Any("kick out list", blackList.BlacklistInfos),
		zap.Uint64("state height", stateHeight),
		zap.Error(err),
	)
	if err == nil {
		return blackList, stateHeight, nil
	}
	return nil, stateHeight, errors.Wrapf(
		err,
		"failed to get kick-out list with epochStartPoint: %t",
		epochStartPoint,
	)
}

// UnproductiveDelegateFromDB returns latest UnproductiveDelegate struct
func UnproductiveDelegateFromDB(sr protocol.StateReader) (*vote.UnproductiveDelegate, error) {
	upd := &vote.UnproductiveDelegate{}
	updKey := ConstructKey(UnproductiveDelegateKey)
	stateHeight, err := sr.State(
		upd,
		protocol.KeyOption(updKey[:]),
		protocol.NamespaceOption(protocol.SystemNamespace),
	)
	log.L().Debug(
		"GetUnproductiveDelegate",
		zap.Uint64("state height", stateHeight),
		zap.Error(err),
	)
	if err == nil {
		return upd, nil
	}
	return nil, err
}

// ConstructLegacyKey constructs a key for candidates storage (deprecated version)
func ConstructLegacyKey(height uint64) hash.Hash160 {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return hash.Hash160b(k)
}

// ConstructKey constructs a const key
func ConstructKey(key string) hash.Hash256 {
	bytesKey := []byte(key)
	return hash.Hash256b(bytesKey)
}

// LoadAndAddCandidates loads candidates from trie and adds a new candidate	// KickoutListFromDB returns array of kickout list at current epoch
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
		candidatesKey := ConstructLegacyKey(uint64(h))
		var err error
		if _, err = sm.State(&sc, protocol.LegacyKeyOption(candidatesKey)); err == nil {
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
	candidatesKey := ConstructLegacyKey(blkHeight)
	_, err = sm.PutState(&candidateList, protocol.LegacyKeyOption(candidatesKey))
	return err
}
