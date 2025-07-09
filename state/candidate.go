// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"strings"

	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/log"

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

	Storage interface {
		StorageAddr(ns string, key []byte) hash.Hash160
		Storage(ns string, key []byte) map[hash.Hash256]uint256.Int
		Load(ns string, key []byte, store func(hash.Hash256) (uint256.Int, bool)) error
	}
)

var _ Storage = &CandidateList{}

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

// StorageAddr returns the storage address for the candidate list
func (l CandidateList) StorageAddr(_ string, _ []byte) hash.Hash160 {
	return hash.Hash160(address.StakingProtocolAddrHash)
}

// Storage converts the candidate list to a storage map
func (l CandidateList) Storage(ns string, key []byte) map[hash.Hash256]uint256.Int {
	prefix := append([]byte(ns), key...)
	store := make(map[hash.Hash256]uint256.Int)

	// Store the number of candidates
	countKey := hash.Hash256b(append(prefix, []byte("count")...))
	store[countKey] = *uint256.NewInt(uint64(len(l)))

	// Store each candidate with an index prefix
	validIndex := 0
	for _, cand := range l {
		if cand == nil {
			continue
		}

		// Use validIndex instead of i to ensure consecutive indices
		indexBytes := make([]byte, 4)
		indexBytes[0] = byte(validIndex >> 24)
		indexBytes[1] = byte(validIndex >> 16)
		indexBytes[2] = byte(validIndex >> 8)
		indexBytes[3] = byte(validIndex)

		candPrefix := append(prefix, indexBytes...)

		// Store candidate address (field 0)
		addr, err := address.FromString(cand.Address)
		if err != nil {
			log.S().Panicf("failed to get address from candidate %s: %v", cand.Address, err)
		}
		store[hash.Hash256b(append(candPrefix, byte(0)))] = *uint256.NewInt(0).SetBytes(addr.Bytes())

		// Store candidate votes (field 1)
		store[hash.Hash256b(append(candPrefix, byte(1)))] = *uint256.NewInt(0).SetBytes(cand.Votes.Bytes())

		// Store reward address (field 2) - always store, even if empty
		if cand.RewardAddress != "" {
			rewardAddr, err := address.FromString(cand.RewardAddress)
			if err != nil {
				log.S().Panicf("failed to get reward address from candidate %s: %v", cand.RewardAddress, err)
			}
			store[hash.Hash256b(append(candPrefix, byte(2)))] = *uint256.NewInt(0).SetBytes(rewardAddr.Bytes())
		} else {
			// Store empty address as zero bytes
			store[hash.Hash256b(append(candPrefix, byte(2)))] = *uint256.NewInt(0)
		}

		// Store candidate name (field 3) - always store, even if empty
		if len(cand.CanName) > 0 {
			if len(cand.CanName) > 32 {
				log.S().Panicf("candidate name %x is too long, should not exceed 32 bytes", cand.CanName)
			}
			store[hash.Hash256b(append(candPrefix, byte(3)))] = *uint256.NewInt(0).SetBytes(cand.CanName)
		} else {
			// Store empty name as zero bytes
			store[hash.Hash256b(append(candPrefix, byte(3)))] = *uint256.NewInt(0)
		}

		validIndex++
	}

	return store
}

// Load restores the candidate list from storage
func (l *CandidateList) Load(ns string, key []byte, store func(hash.Hash256) (uint256.Int, bool)) error {
	prefix := append([]byte(ns), key...)

	// Get the number of candidates
	countKey := hash.Hash256b(append(prefix, []byte("count")...))
	countVal, exists := store(countKey)
	if !exists {
		*l = CandidateList{}
		return nil
	}

	count := countVal.Uint64()
	candidates := make(CandidateList, 0, count)

	// Load each candidate by index
	for i := uint64(0); i < count; i++ {
		// Create index bytes
		indexBytes := make([]byte, 4)
		indexBytes[0] = byte(i >> 24)
		indexBytes[1] = byte(i >> 16)
		indexBytes[2] = byte(i >> 8)
		indexBytes[3] = byte(i)

		candPrefix := append(prefix, indexBytes...)

		// Load candidate address (field 0)
		addrKey := hash.Hash256b(append(candPrefix, byte(0)))
		addrVal, exists := store(addrKey)
		if !exists {
			return errors.Wrapf(ErrCandidateMap, "missing address for candidate at index %d", i)
		}

		addr, err := address.FromBytes(addrVal.Bytes())
		if err != nil {
			return errors.Wrapf(err, "failed to parse address for candidate at index %d", i)
		}

		// Load candidate votes (field 1)
		votesKey := hash.Hash256b(append(candPrefix, byte(1)))
		votesVal, exists := store(votesKey)
		if !exists {
			return errors.Wrapf(ErrCandidateMap, "missing votes for candidate at index %d", i)
		}

		votes := big.NewInt(0).SetBytes(votesVal.Bytes())

		// Create candidate
		candidate := &Candidate{
			Address: addr.String(),
			Votes:   votes,
		}

		// Load reward address (field 2) - always expected to be present
		rewardKey := hash.Hash256b(append(candPrefix, byte(2)))
		rewardVal, exists := store(rewardKey)
		if !exists {
			return errors.Wrapf(ErrCandidateMap, "missing reward address for candidate at index %d", i)
		}

		// Check if reward address is empty (zero value)
		if !rewardVal.IsZero() {
			rewardAddr, err := address.FromBytes(rewardVal.Bytes())
			if err != nil {
				return errors.Wrapf(err, "failed to parse reward address for candidate at index %d", i)
			}
			candidate.RewardAddress = rewardAddr.String()
		} else {
			candidate.RewardAddress = ""
		}

		// Load candidate name (field 3) - always expected to be present
		nameKey := hash.Hash256b(append(candPrefix, byte(3)))
		nameVal, exists := store(nameKey)
		if !exists {
			return errors.Wrapf(ErrCandidateMap, "missing candidate name for candidate at index %d", i)
		}

		// Check if candidate name is empty (zero value)
		if !nameVal.IsZero() {
			candidate.CanName = nameVal.Bytes()
		} else {
			candidate.CanName = nil
		}

		candidates = append(candidates, candidate)
	}

	*l = candidates
	return nil
}

func (l *CandidateList) String() string {
	if l == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, cand := range *l {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(cand.Address)
		if cand.Votes != nil {
			sb.WriteString(" (Votes: " + cand.Votes.String() + ")")
		}
		if cand.RewardAddress != "" {
			sb.WriteString(" (Reward: " + cand.RewardAddress + ")")
		}
		if len(cand.CanName) > 0 {
			sb.WriteString(" (Name: " + string(cand.CanName) + ")")
		}
	}
	sb.WriteString("]")
	return sb.String()
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
