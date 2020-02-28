// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// Candidate represents the candidate
	Candidate struct {
		Owner     address.Address
		Operator  address.Address
		Reward    address.Address
		Name      string
		Votes     *big.Int
		SelfStake *big.Int
	}

	// CandidateList is a list of candidates which is sortable
	CandidateList []*Candidate
)

// NewCandidate creates a Candidate instance and set votes to 0.
func NewCandidate(owner, operator, reward address.Address, name string, selfStake *big.Int) *Candidate {
	return &Candidate{
		Owner:     owner,
		Operator:  operator,
		Reward:    reward,
		Name:      name,
		Votes:     big.NewInt(0),
		SelfStake: selfStake,
	}
}

// AddVote adds vote
func (d *Candidate) AddVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}
	d.Votes.Add(d.Votes, amount)
	return nil
}

// SubVote subtracts vote
func (d *Candidate) SubVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}

	if d.Votes.Cmp(amount) == -1 {
		return ErrInvalidAmount
	}
	d.Votes.Sub(d.Votes, amount)
	return nil
}

// Serialize serializes candidate to bytes
func (d *Candidate) Serialize() ([]byte, error) {
	return proto.Marshal(d.toProto())
}

// Deserialize deserializes bytes to candidate
func (d *Candidate) Deserialize(buf []byte) error {
	pb := &stakingpb.Candidate{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate")
	}
	return d.fromProto(pb)
}

func (d *Candidate) toProto() *stakingpb.Candidate {
	pb := &stakingpb.Candidate{
		OwnerAddress:    d.Owner.String(),
		OperatorAddress: d.Operator.String(),
		RewardAddress:   d.Reward.String(),
		Name:            d.Name,
	}
	if d.Votes != nil {
		pb.Votes = d.Votes.String()
	}
	if d.SelfStake != nil {
		pb.SelfStake = d.SelfStake.String()
	}
	return pb
}

func (d *Candidate) fromProto(pb *stakingpb.Candidate) error {
	var err error
	d.Owner, err = address.FromString(pb.GetOwnerAddress())
	if err != nil {
		return err
	}

	d.Operator, err = address.FromString(pb.GetOwnerAddress())
	if err != nil {
		return err
	}

	d.Reward, err = address.FromString(pb.GetRewardAddress())
	if err != nil {
		return err
	}
	d.Name = pb.GetName()

	d.Votes, _ = new(big.Int).SetString(pb.GetVotes(), 10)
	d.SelfStake, _ = new(big.Int).SetString(pb.GetSelfStake(), 10)
	return nil
}

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l CandidateList) Less(i, j int) bool {
	if res := l[i].Votes.Cmp(l[j].Votes); res != 0 {
		return res == 1
	}
	return strings.Compare(l[i].Owner.String(), l[j].Owner.String()) == 1
}

func (l CandidateList) toProto() *stakingpb.Candidates {
	candidatePb := make([]*stakingpb.Candidate, len(l))
	for i, del := range l {
		candidatePb[i] = del.toProto()
	}
	return &stakingpb.Candidates{Candidates: candidatePb}
}

// Deserialize deserializes bytes to list of candidates
func (l *CandidateList) Deserialize(buf []byte) error {
	pb := &stakingpb.Candidates{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate list")
	}

	*l = (*l)[:0]
	for _, v := range pb.Candidates {
		c := &Candidate{}
		if err := c.fromProto(v); err != nil {
			return err
		}
		*l = append(*l, c)
	}
	return nil
}

func stakingGetCandidate(sr protocol.StateReader, name address.Address) (*Candidate, error) {
	key := make([]byte, len(name.Bytes()))
	copy(key, name.Bytes())

	var d Candidate
	_, err := sr.State(&d, protocol.NamespaceOption(factory.CandidateNameSpace), protocol.KeyOption(key))
	if err == nil {
		return &d, nil
	}

	if errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}

	if _, err = sr.State(&d, protocol.NamespaceOption(factory.CandidateNameSpace), protocol.KeyOption(key)); err != nil {
		return nil, err
	}
	return &d, nil
}

func stakingPutCandidate(sm protocol.StateManager, name address.Address, d *Candidate) error {
	key := make([]byte, len(name.Bytes()))
	copy(key, name.Bytes())

	_, err := sm.PutState(d, protocol.NamespaceOption(factory.CandidateNameSpace), protocol.KeyOption(key))
	return err
}

func stakingDelCandidate(sm protocol.StateManager, name address.Address) error {
	key := make([]byte, len(name.Bytes()))
	copy(key, name.Bytes())

	_, err := sm.DelState(protocol.NamespaceOption(factory.CandidateNameSpace), protocol.KeyOption(key))
	return err
}
