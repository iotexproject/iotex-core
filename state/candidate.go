// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
)

var (
	// ErrCandidate indicates the error of candidate
	ErrCandidate = errors.New("invalid candidate")
	// ErrCandidatePb indicates the error of protobuf's candidate message
	ErrCandidatePb = errors.New("invalid protobuf's candidate message")
	// ErrCandidateMap indicates the error of candidate map
	ErrCandidateMap = errors.New("invalid candidate map")
	// ErrCandidateList indicates the error of candidate list
	ErrCandidateList = errors.New("invalid candidate list")
)

type (
	// Candidate indicates the structure of a candidate
	Candidate struct {
		Address       string
		Votes         *big.Int
		RewardAddress string
		CanName       []byte // used as identifier to merge with native staking result, not part of protobuf
	}

	// CandidateList indicates the list of Candidates which is sortable
	CandidateList []*Candidate

	// CandidateMap is a map of Candidates using Hash160 as key
	CandidateMap map[hash.Hash160]*Candidate
)

// Equal compares two candidate instances
func (c *Candidate) Equal(d *Candidate) bool {
	if c == d {
		return true
	}
	if c == nil || d == nil {
		return false
	}
	return strings.Compare(c.Address, d.Address) == 0 &&
		c.RewardAddress == d.RewardAddress &&
		c.Votes.Cmp(d.Votes) == 0
}

// Clone makes a copy of the candidate
func (c *Candidate) Clone() *Candidate {
	if c == nil {
		return nil
	}
	name := make([]byte, len(c.CanName))
	copy(name, c.CanName)
	return &Candidate{
		Address:       c.Address,
		Votes:         new(big.Int).Set(c.Votes),
		RewardAddress: c.RewardAddress,
		CanName:       name,
	}
}

// Serialize serializes candidate to bytes
func (c *Candidate) Serialize() ([]byte, error) {
	return proto.Marshal(candidateToPb(c))
}

// Deserialize deserializes bytes to candidate
func (c *Candidate) Deserialize(buf []byte) error {
	pb := &iotextypes.Candidate{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate")
	}

	cand, err := pbToCandidate(pb)
	if err != nil {
		return errors.Wrap(err, "failed to convert protobuf's candidate message to candidate")
	}
	*c = *cand

	return nil
}

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l CandidateList) Less(i, j int) bool {
	if res := l[i].Votes.Cmp(l[j].Votes); res != 0 {
		return res == 1
	}
	return strings.Compare(l[i].Address, l[j].Address) == 1
}

// Serialize serializes a list of Candidates to bytes
func (l *CandidateList) Serialize() ([]byte, error) {
	return proto.Marshal(l.Proto())
}

// Proto converts the candidate list to a protobuf message
func (l *CandidateList) Proto() *iotextypes.CandidateList {
	candidatesPb := make([]*iotextypes.Candidate, 0, len(*l))
	for _, cand := range *l {
		candidatesPb = append(candidatesPb, candidateToPb(cand))
	}
	return &iotextypes.CandidateList{Candidates: candidatesPb}
}

// Deserialize deserializes bytes to list of Candidates
func (l *CandidateList) Deserialize(buf []byte) error {
	candList := &iotextypes.CandidateList{}
	if err := proto.Unmarshal(buf, candList); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate list")
	}
	return l.LoadProto(candList)
}

// LoadProto loads candidate list from proto
func (l *CandidateList) LoadProto(candList *iotextypes.CandidateList) error {
	candidates := make(CandidateList, 0)
	candidatesPb := candList.Candidates
	for _, candPb := range candidatesPb {
		cand, err := pbToCandidate(candPb)
		if err != nil {
			return errors.Wrap(err, "failed to convert protobuf's candidate message to candidate")
		}
		candidates = append(candidates, cand)
	}
	*l = candidates

	return nil
}

// candidateToPb converts a candidate to protobuf's candidate message
func candidateToPb(cand *Candidate) *iotextypes.Candidate {
	candidatePb := &iotextypes.Candidate{
		Address:       cand.Address,
		Votes:         cand.Votes.Bytes(),
		RewardAddress: cand.RewardAddress,
	}
	if cand.Votes != nil && len(cand.Votes.Bytes()) > 0 {
		candidatePb.Votes = cand.Votes.Bytes()
	}
	return candidatePb
}

// pbToCandidate converts a protobuf's candidate message to candidate
func pbToCandidate(candPb *iotextypes.Candidate) (*Candidate, error) {
	if candPb == nil {
		return nil, errors.Wrap(ErrCandidatePb, "protobuf's candidate message cannot be nil")
	}
	candidate := &Candidate{
		Address:       candPb.Address,
		Votes:         big.NewInt(0).SetBytes(candPb.Votes),
		RewardAddress: candPb.RewardAddress,
	}
	return candidate, nil
}

// MapToCandidates converts a map of cachedCandidates to candidate list
func MapToCandidates(candidateMap CandidateMap) (CandidateList, error) {
	candidates := make(CandidateList, 0, len(candidateMap))
	for _, cand := range candidateMap {
		candidates = append(candidates, cand)
	}
	return candidates, nil
}

// CandidatesToMap converts a candidate list to map of cachedCandidates
func CandidatesToMap(candidates CandidateList) (CandidateMap, error) {
	candidateMap := make(CandidateMap)
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, errors.Wrap(ErrCandidate, "candidate cannot be nil")
		}
		addr, err := address.FromString(candidate.Address)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get the hash of the address")
		}
		pkHash := hash.BytesToHash160(addr.Bytes())
		candidateMap[pkHash] = candidate
	}
	return candidateMap, nil
}
