// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"

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
		BLSPubKey     []byte // BLS public key, used for verification
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
		c.Votes.Cmp(d.Votes) == 0 &&
		bytes.Equal(c.BLSPubKey, d.BLSPubKey)
}

// Clone makes a copy of the candidate
func (c *Candidate) Clone() *Candidate {
	if c == nil {
		return nil
	}
	name := make([]byte, len(c.CanName))
	copy(name, c.CanName)
	var pubkey []byte
	if len(c.BLSPubKey) > 0 {
		pubkey = make([]byte, len(c.BLSPubKey))
		copy(pubkey, c.BLSPubKey)
	}
	return &Candidate{
		Address:       c.Address,
		Votes:         new(big.Int).Set(c.Votes),
		RewardAddress: c.RewardAddress,
		CanName:       name,
		BLSPubKey:     pubkey,
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

// Encode encodes a CandidateList into a GenericValue
func (l *CandidateList) Encode() ([][]byte, []systemcontracts.GenericValue, error) {
	var (
		suffix [][]byte
		values []systemcontracts.GenericValue
	)
	for idx, cand := range *l {
		pbCand := candidateToPb(cand)
		dataVotes, err := proto.Marshal(&iotextypes.Candidate{
			Votes: pbCand.Votes,
		})
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to serialize candidate votes")
		}
		pbCand.Address = "" // address is stored in the suffix
		pbCand.Votes = nil  // votes is stored in the secondary data
		data, err := proto.Marshal(pbCand)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to serialize candidate")
		}
		addr, err := address.FromString(cand.Address)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to get the hash of the address %s", cand.Address)
		}
		// Use linked list format: AuxiliaryData stores the next node's address
		// For the last element, AuxiliaryData is nil
		var nextAddr []byte
		if idx < len(*l)-1 {
			nextAddrObj, err := address.FromString((*l)[idx+1].Address)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to get the address of the next candidate %s", (*l)[idx+1].Address)
			}
			nextAddr = nextAddrObj.Bytes()
		}
		suffix = append(suffix, addr.Bytes())
		values = append(values, systemcontracts.GenericValue{
			PrimaryData:   data,
			SecondaryData: dataVotes,
			AuxiliaryData: nextAddr})
	}
	return suffix, values, nil
}

// Decode decodes a GenericValue into CandidateList
func (l *CandidateList) Decode(suffixs [][]byte, values []systemcontracts.GenericValue) error {
	if len(suffixs) != len(values) {
		return errors.New("suffix and values length mismatch")
	}
	if len(suffixs) == 0 {
		*l = CandidateList{}
		return nil
	}
	// Build address to candidate node mapping
	nodeMap, err := buildCandidateMap(suffixs, values)
	if err != nil {
		return err
	}

	// Find the head of the linked list
	headAddr, err := findLinkedListHead(nodeMap)
	if err != nil {
		return err
	}

	// Traverse the linked list to reconstruct the candidate list
	candidates, err := traverseLinkedList(headAddr, nodeMap)
	if err != nil {
		return err
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
	if len(cand.BLSPubKey) > 0 {
		candidatePb.BlsPubKey = cand.BLSPubKey
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
		BLSPubKey:     candPb.BlsPubKey,
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

// candidateNode represents a node in the candidate linked list
type candidateNode struct {
	candidate *Candidate
	nextAddr  string
}

// buildCandidateMap builds a map from address bytes to candidate node
func buildCandidateMap(suffixs [][]byte, values []systemcontracts.GenericValue) (map[string]*candidateNode, error) {
	nodeMap := make(map[string]*candidateNode, len(suffixs))

	for kid, gv := range values {
		pb := &iotextypes.Candidate{}
		if err := proto.Unmarshal(gv.PrimaryData, pb); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal candidate")
		}

		addr, err := address.FromBytes(suffixs[kid])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the string of the address from bytes %x", suffixs[kid])
		}
		pb.Address = addr.String()

		// Load votes from SecondaryData
		pbVotes := &iotextypes.Candidate{}
		if err := proto.Unmarshal(gv.SecondaryData, pbVotes); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal candidate votes")
		}
		pb.Votes = pbVotes.Votes

		// Convert pb to candidate
		cand, err := pbToCandidate(pb)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert protobuf's candidate message to candidate")
		}
		var next string
		if len(gv.AuxiliaryData) > 0 {
			nextAddr, err := address.FromBytes(gv.AuxiliaryData)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get the string of the next address from bytes %x", gv.AuxiliaryData)
			}
			next = nextAddr.String()
		}
		nodeMap[addr.String()] = &candidateNode{
			candidate: cand,
			nextAddr:  next,
		}
	}

	return nodeMap, nil
}

// findLinkedListHead finds the head of the linked list (not pointed to by any node)
func findLinkedListHead(nodeMap map[string]*candidateNode) (string, error) {
	// Mark all addresses that are pointed to
	pointedTo := make(map[string]bool, len(nodeMap))
	for _, node := range nodeMap {
		if len(node.nextAddr) > 0 {
			pointedTo[node.nextAddr] = true
		}
	}

	// Find the address that is not pointed to by any node
	for addrKey := range nodeMap {
		if !pointedTo[addrKey] {
			return addrKey, nil
		}
	}

	return "", errors.New("failed to find head of candidate list")
}

// traverseLinkedList traverses the linked list and returns an ordered candidate list
func traverseLinkedList(headAddr string, nodeMap map[string]*candidateNode) (CandidateList, error) {
	candidates := make(CandidateList, 0, len(nodeMap))
	visited := make(map[string]bool, len(nodeMap))
	currentAddr := headAddr

	for currentAddr != "" {
		addrKey := currentAddr

		// Check for circular reference
		if visited[addrKey] {
			return nil, errors.New("circular reference detected in candidate list")
		}
		visited[addrKey] = true

		// Get current node
		node, exists := nodeMap[addrKey]
		if !exists {
			return nil, errors.Errorf("missing candidate for address %x in linked list", currentAddr)
		}

		candidates = append(candidates, node.candidate)
		currentAddr = node.nextAddr
	}

	// Verify all nodes were traversed
	if len(candidates) != len(nodeMap) {
		return nil, errors.Errorf("incomplete traversal: %d/%d candidates", len(candidates), len(nodeMap))
	}

	return candidates, nil
}
