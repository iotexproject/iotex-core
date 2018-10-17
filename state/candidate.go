// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
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

// Candidate indicates the structure of a candidate
type Candidate struct {
	Address          string
	Votes            *big.Int
	PublicKey        keypair.PublicKey
	CreationHeight   uint64
	LastUpdateHeight uint64
}

// CandidateList indicates the list of Candidates which is sortable
type CandidateList []*Candidate

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l CandidateList) Less(i, j int) bool {
	res := l[i].Votes.Cmp(l[j].Votes)
	return res == 1
}

// Serialize serializes a list of Candidates to bytes
func (l CandidateList) Serialize() ([]byte, error) {
	candidatesPb := make([]*iproto.Candidate, 0, len(l))
	for _, cand := range l {
		candPb, err := candidateToPb(cand)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert candidate to protobuf's candidate message")
		}
		candidatesPb = append(candidatesPb, candPb)
	}
	return proto.Marshal(&iproto.CandidateList{Candidates: candidatesPb})
}

// Deserialize deserializes bytes to list of Candidates
func (l CandidateList) Deserialize(buf []byte) (CandidateList, error) {
	candList := &iproto.CandidateList{}
	if err := proto.Unmarshal(buf, candList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal candidate list")
	}
	candidatesPb := candList.Candidates
	for _, candPb := range candidatesPb {
		cand, err := pbToCandidate(candPb)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert protobuf's candidate message to candidate")
		}
		l = append(l, cand)
	}
	return l, nil
}

// candidateToPb converts a candidate to protobuf's candidate message
func candidateToPb(cand *Candidate) (*iproto.Candidate, error) {
	if cand == nil {
		return nil, errors.Wrap(ErrCandidate, "candidate cannot be nil")
	}
	candidatePb := &iproto.Candidate{
		Address:          cand.Address,
		PubKey:           cand.PublicKey[:],
		CreationHeight:   cand.CreationHeight,
		LastUpdateHeight: cand.LastUpdateHeight,
	}
	if cand.Votes != nil && len(cand.Votes.Bytes()) > 0 {
		candidatePb.Votes = cand.Votes.Bytes()
	}
	return candidatePb, nil
}

// pbToCandidate converts a protobuf's candidate message to candidate
func pbToCandidate(candPb *iproto.Candidate) (*Candidate, error) {
	if candPb == nil {
		return nil, errors.Wrap(ErrCandidatePb, "protobuf's candidate message cannot be nil")
	}

	var pk keypair.PublicKey
	copy(pk[:], candPb.PubKey)
	candidate := &Candidate{
		Address:          candPb.Address,
		Votes:            big.NewInt(0).SetBytes(candPb.Votes),
		PublicKey:        pk,
		CreationHeight:   candPb.CreationHeight,
		LastUpdateHeight: candPb.LastUpdateHeight,
	}
	return candidate, nil
}

// MapToCandidates converts a map of cachedCandidates to candidate list
func MapToCandidates(candidateMap map[hash.PKHash]*Candidate) (CandidateList, error) {
	if candidateMap == nil {
		return nil, errors.Wrap(ErrCandidateMap, "candidate map cannot be nil")
	}
	candidates := make(CandidateList, 0, len(candidateMap))
	for _, cand := range candidateMap {
		candidates = append(candidates, cand)
	}
	return candidates, nil
}

// CandidatesToMap converts a candidate list to map of cachedCandidates
func CandidatesToMap(candidates CandidateList) (map[hash.PKHash]*Candidate, error) {
	if candidates == nil {
		return nil, errors.Wrap(ErrCandidateList, "candidate list cannot be nil")
	}
	candidateMap := make(map[hash.PKHash]*Candidate)
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, errors.Wrap(ErrCandidate, "candidate cannot be nil")
		}
		pkHash, err := iotxaddress.GetPubkeyHash(candidate.Address)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get the hash of the address")
		}
		candidateMap[byteutil.BytesTo20B(pkHash)] = candidate
	}
	return candidateMap, nil
}
