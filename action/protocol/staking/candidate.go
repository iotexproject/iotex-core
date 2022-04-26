// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"sort"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// Candidate represents the candidate
	Candidate struct {
		Owner              address.Address
		Operator           address.Address
		Reward             address.Address
		Name               string
		Votes              *big.Int
		SelfStakeBucketIdx uint64
		SelfStake          *big.Int
	}

	// CandidateList is a list of candidates which is sortable
	CandidateList []*Candidate

	// RegistrationConsts are the registration fee and min self stake
	RegistrationConsts struct {
		Fee          *big.Int
		MinSelfStake *big.Int
	}
)

// Clone returns a copy
func (d *Candidate) Clone() *Candidate {
	return &Candidate{
		Owner:              d.Owner,
		Operator:           d.Operator,
		Reward:             d.Reward,
		Name:               d.Name,
		Votes:              new(big.Int).Set(d.Votes),
		SelfStakeBucketIdx: d.SelfStakeBucketIdx,
		SelfStake:          new(big.Int).Set(d.SelfStake),
	}
}

// Equal tests equality of 2 candidates
func (d *Candidate) Equal(c *Candidate) bool {
	return d.Name == c.Name &&
		d.SelfStakeBucketIdx == c.SelfStakeBucketIdx &&
		address.Equal(d.Owner, c.Owner) &&
		address.Equal(d.Operator, c.Operator) &&
		address.Equal(d.Reward, c.Reward) &&
		d.Votes.Cmp(c.Votes) == 0 &&
		d.SelfStake.Cmp(c.SelfStake) == 0
}

// Validate does the sanity check
func (d *Candidate) Validate() error {
	if d.Votes == nil {
		return ErrInvalidAmount
	}

	if d.Name == "" {
		return ErrInvalidCanName
	}

	if d.Owner == nil {
		return ErrInvalidOwner
	}

	if d.Operator == nil {
		return ErrInvalidOperator
	}

	if d.Reward == nil {
		return ErrInvalidReward
	}

	if d.SelfStake == nil {
		return ErrInvalidAmount
	}
	return nil
}

// Collision checks collsion of 2 candidates
func (d *Candidate) Collision(c *Candidate) error {
	if address.Equal(d.Owner, c.Owner) {
		return nil
	}
	if c.Name == d.Name {
		return ErrInvalidCanName
	}
	if address.Equal(c.Operator, d.Operator) {
		return ErrInvalidOperator
	}
	if c.SelfStakeBucketIdx == d.SelfStakeBucketIdx {
		return ErrInvalidSelfStkIndex
	}
	return nil
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

// AddSelfStake adds self stake
func (d *Candidate) AddSelfStake(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}
	d.SelfStake.Add(d.SelfStake, amount)
	return nil
}

// SubSelfStake subtracts self stake
func (d *Candidate) SubSelfStake(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}

	if d.Votes.Cmp(amount) == -1 {
		return ErrInvalidAmount
	}
	d.SelfStake.Sub(d.SelfStake, amount)
	return nil
}

// Serialize serializes candidate to bytes
func (d *Candidate) Serialize() ([]byte, error) {
	pb, err := d.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes to candidate
func (d *Candidate) Deserialize(buf []byte) error {
	pb := &stakingpb.Candidate{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate")
	}
	return d.fromProto(pb)
}

func (d *Candidate) toProto() (*stakingpb.Candidate, error) {
	if d.Owner == nil || d.Operator == nil || d.Reward == nil ||
		len(d.Name) == 0 || d.Votes == nil || d.SelfStake == nil {
		return nil, ErrMissingField
	}

	return &stakingpb.Candidate{
		OwnerAddress:       d.Owner.String(),
		OperatorAddress:    d.Operator.String(),
		RewardAddress:      d.Reward.String(),
		Name:               d.Name,
		Votes:              d.Votes.String(),
		SelfStakeBucketIdx: d.SelfStakeBucketIdx,
		SelfStake:          d.SelfStake.String(),
	}, nil
}

func (d *Candidate) fromProto(pb *stakingpb.Candidate) error {
	var err error
	d.Owner, err = address.FromString(pb.GetOwnerAddress())
	if err != nil {
		return err
	}

	d.Operator, err = address.FromString(pb.GetOperatorAddress())
	if err != nil {
		return err
	}

	d.Reward, err = address.FromString(pb.GetRewardAddress())
	if err != nil {
		return err
	}

	if len(pb.GetName()) == 0 {
		return ErrMissingField
	}
	d.Name = pb.GetName()

	var ok bool
	d.Votes, ok = new(big.Int).SetString(pb.GetVotes(), 10)
	if !ok {
		return ErrInvalidAmount
	}

	d.SelfStakeBucketIdx = pb.GetSelfStakeBucketIdx()
	d.SelfStake, ok = new(big.Int).SetString(pb.GetSelfStake(), 10)
	if !ok {
		return ErrInvalidAmount
	}
	return nil
}

func (d *Candidate) toIoTeXTypes() *iotextypes.CandidateV2 {
	return &iotextypes.CandidateV2{
		OwnerAddress:       d.Owner.String(),
		OperatorAddress:    d.Operator.String(),
		RewardAddress:      d.Reward.String(),
		Name:               d.Name,
		TotalWeightedVotes: d.Votes.String(),
		SelfStakeBucketIdx: d.SelfStakeBucketIdx,
		SelfStakingTokens:  d.SelfStake.String(),
	}
}

func (d *Candidate) toStateCandidate() *state.Candidate {
	return &state.Candidate{
		Address:       d.Operator.String(), // state need candidate operator not owner address
		Votes:         new(big.Int).Set(d.Votes),
		RewardAddress: d.Reward.String(),
		CanName:       []byte(d.Name),
	}
}

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l CandidateList) Less(i, j int) bool {
	if res := l[i].Votes.Cmp(l[j].Votes); res != 0 {
		return res == 1
	}
	return strings.Compare(l[i].Owner.String(), l[j].Owner.String()) == 1
}

// Serialize serializes candidate to bytes
func (l CandidateList) Serialize() ([]byte, error) {
	pb, err := l.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

func (l CandidateList) toProto() (*stakingpb.Candidates, error) {
	candidatePb := make([]*stakingpb.Candidate, len(l))
	for i, del := range l {
		dpb, err := del.toProto()
		if err != nil {
			return nil, err
		}
		candidatePb[i] = dpb
	}
	return &stakingpb.Candidates{Candidates: candidatePb}, nil
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

func (l CandidateList) toStateCandidateList() (state.CandidateList, error) {
	list := make(state.CandidateList, 0, len(l))
	for _, c := range l {
		list = append(list, c.toStateCandidate())
	}
	sort.Sort(list)
	return list, nil
}
