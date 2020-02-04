// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package candidatesutil

import (
	"go.uber.org/zap"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
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
	candidatesKey := ConstructKey(height)
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
// GetCandidates returns array of Candidates at current epoch
func GetCandidates(sr protocol.StateReader, epochStartPoint bool) ([]*state.Candidate, error) {
	var candidates state.CandidateList
	candidatesKey := ConstructConstKey(CurCandidateKey)
	if epochStartPoint {
		// if not shifted yet
		candidatesKey = ConstructConstKey(NxtCandidateKey)
	}
	err := sr.State(candidatesKey, &candidates)
	log.L().Debug(
		"GetCandidates",
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
		"failed to get candidates with epochStartEpoch: %t",
		epochStartPoint,
	)
}

// GetKickoutList returns array of kickout list at current epoch
func GetKickoutList(sr protocol.StateReader, epochStartPoint bool) (*vote.Blacklist, error) {
	blackList := &vote.Blacklist{}
	blackListKey := ConstructConstKey(CurKickoutKey)
	if epochStartPoint {
		// if not shifted yet
		blackListKey = ConstructConstKey(NxtKickoutKey)
	}
	_, err := sr.State(blackList, protocol.LegacyKeyOption(blackListKey))
	log.L().Debug(
		"GetKickoutList",
		zap.Any("kick out list", blackList.BlacklistInfos),
		zap.Error(err),
	)
	if err == nil {
		return blackList, nil
	}
	return nil, errors.Wrapf(
		err,
		"failed to get kick-out list with epochStartPoint: %t",
		epochStartPoint,
	)
}

// GetUnproductiveDelegate returns latest UnproductiveDelegate struct
func GetUnproductiveDelegate(sr protocol.StateReader) (*vote.UnproductiveDelegate, error) {
	upd := &vote.UnproductiveDelegate{}
	// Load kick out list on the given epochNum from underlying db
	updKey := ConstructConstKey(UnproductiveDelegateKey)
	err := sr.State(upd, protocol.LegacyKeyOption(updKey))
	log.L().Debug(
		"GetUnproductiveDelegate",
		zap.Error(err),
	)
	if err == nil {
		return upd, nil
	}
	return nil, err
}

// ConstructKey constructs a key for candidates storage (deprecated version)
func ConstructKey(height uint64) hash.Hash160 {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return hash.Hash160b(k)
}

// ConstructConstKey constructs a const key
func ConstructConstKey(key string) hash.Hash160 {
	bytesKey := []byte(key)
	return hash.Hash160b(bytesKey)
}
