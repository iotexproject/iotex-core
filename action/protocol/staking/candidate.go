// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"sort"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	// Candidate represents the candidate
	Candidate struct {
		Owner              address.Address
		Operator           address.Address
		Reward             address.Address
		Identifier         address.Address
		BLSPubKey          []byte // BLS public key
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
		Identifier:         d.Identifier,
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
		address.Equal(d.Identifier, c.Identifier) &&
		d.Votes.Cmp(c.Votes) == 0 &&
		d.SelfStake.Cmp(c.SelfStake) == 0
}

// Validate does the sanity check
func (d *Candidate) Validate() error {
	if d.Votes == nil {
		return action.ErrInvalidAmount
	}

	if d.Name == "" {
		return action.ErrInvalidCanName
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
		return action.ErrInvalidAmount
	}
	return nil
}

// isSelfStakeBucketSettled checks if self stake bucket is settled
func (d *Candidate) isSelfStakeBucketSettled() bool {
	return d.SelfStakeBucketIdx != candidateNoSelfStakeBucketIndex
}

// Collision checks collsion of 2 candidates
func (d *Candidate) Collision(c *Candidate) error {
	if address.Equal(d.Owner, c.Owner) || address.Equal(d.GetIdentifier(), c.GetIdentifier()) {
		return nil
	}
	if address.Equal(d.Owner, c.Owner) {
		return action.ErrInvalidOwner
	}
	if c.Name == d.Name {
		return action.ErrInvalidCanName
	}
	if address.Equal(c.Operator, d.Operator) {
		return ErrInvalidOperator
	}
	if c.SelfStakeBucketIdx == d.SelfStakeBucketIdx && c.isSelfStakeBucketSettled() {
		return ErrInvalidSelfStkIndex
	}
	return nil
}

// AddVote adds vote
func (d *Candidate) AddVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return action.ErrInvalidAmount
	}
	d.Votes.Add(d.Votes, amount)
	return nil
}

// SubVote subtracts vote
func (d *Candidate) SubVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return action.ErrInvalidAmount
	}

	if d.Votes.Cmp(amount) == -1 {
		return action.ErrInvalidAmount
	}
	d.Votes.Sub(d.Votes, amount)
	return nil
}

// AddSelfStake adds self stake
func (d *Candidate) AddSelfStake(amount *big.Int) error {
	if amount.Sign() < 0 {
		return action.ErrInvalidAmount
	}
	d.SelfStake.Add(d.SelfStake, amount)
	return nil
}

// SubSelfStake subtracts self stake
func (d *Candidate) SubSelfStake(amount *big.Int) error {
	if amount.Sign() < 0 {
		return action.ErrInvalidAmount
	}

	if d.Votes.Cmp(amount) == -1 {
		return action.ErrInvalidAmount
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

// TODO: rename to ID
// GetIdentifier returns the identifier
func (d *Candidate) GetIdentifier() address.Address {
	if d.Identifier == nil {
		return d.Owner
	}
	return d.Identifier
}

// Encode encodes candidate into generic value
func (d *Candidate) Encode() (systemcontracts.GenericValue, error) {
	var (
		primaryData   []byte
		secondaryData []byte
		err           error
		value         systemcontracts.GenericValue
	)
	if d.Votes.Sign() > 0 {
		secondaryData, err = proto.Marshal(&stakingpb.Candidate{Votes: d.Votes.String()})
		if err != nil {
			return value, errors.Wrap(err, "failed to marshal candidate votes")
		}
	}
	clone := d.Clone()
	clone.Votes = big.NewInt(0)
	primaryData, err = clone.Serialize()
	if err != nil {
		return value, errors.Wrap(err, "failed to serialize candidate")
	}
	value.PrimaryData = primaryData
	value.SecondaryData = secondaryData
	return value, nil
}

// Decode decodes candidate from generic value
func (d *Candidate) Decode(gv systemcontracts.GenericValue) error {
	if err := d.Deserialize(gv.PrimaryData); err != nil {
		return errors.Wrap(err, "failed to deserialize candidate")
	}
	if len(gv.SecondaryData) > 0 {
		votes := &stakingpb.Candidate{}
		if err := proto.Unmarshal(gv.SecondaryData, votes); err != nil {
			return errors.Wrap(err, "failed to unmarshal candidate votes")
		}
		vote, ok := new(big.Int).SetString(votes.Votes, 10)
		if !ok {
			return errors.Wrapf(action.ErrInvalidAmount, "failed to parse candidate votes: %s", votes.Votes)
		}
		d.Votes = vote
	}
	return nil
}

func (d *Candidate) toProto() (*stakingpb.Candidate, error) {
	if d.Owner == nil || d.Operator == nil || d.Reward == nil ||
		len(d.Name) == 0 || d.Votes == nil || d.SelfStake == nil {
		return nil, ErrMissingField
	}
	voter := ""
	if d.Identifier != nil {
		voter = d.Identifier.String()
	}
	var pubkey []byte
	if len(d.BLSPubKey) > 0 {
		pubkey = make([]byte, len(d.BLSPubKey))
		copy(pubkey, d.BLSPubKey)
	}

	return &stakingpb.Candidate{
		OwnerAddress:       d.Owner.String(),
		OperatorAddress:    d.Operator.String(),
		RewardAddress:      d.Reward.String(),
		IdentifierAddress:  voter,
		Name:               d.Name,
		Votes:              d.Votes.String(),
		SelfStakeBucketIdx: d.SelfStakeBucketIdx,
		SelfStake:          d.SelfStake.String(),
		Pubkey:             pubkey,
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

	if id := pb.GetIdentifierAddress(); len(id) > 0 {
		d.Identifier, err = address.FromString(id)
		if err != nil {
			return err
		}
	}

	if len(pb.GetName()) == 0 {
		return ErrMissingField
	}
	d.Name = pb.GetName()

	var ok bool
	d.Votes, ok = new(big.Int).SetString(pb.GetVotes(), 10)
	if !ok {
		return action.ErrInvalidAmount
	}

	d.SelfStakeBucketIdx = pb.GetSelfStakeBucketIdx()
	d.SelfStake, ok = new(big.Int).SetString(pb.GetSelfStake(), 10)
	if !ok {
		return action.ErrInvalidAmount
	}
	if len(pb.GetPubkey()) > 0 {
		d.BLSPubKey = make([]byte, len(pb.GetPubkey()))
		copy(d.BLSPubKey, pb.GetPubkey())
	} else {
		d.BLSPubKey = nil
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
		Id:                 d.GetIdentifier().String(),
	}
}

func (d *Candidate) toStateCandidate() *state.Candidate {
	return &state.Candidate{
		Address:       d.Operator.String(), // state need candidate operator not owner address
		Votes:         new(big.Int).Set(d.Votes),
		RewardAddress: d.Reward.String(),
		CanName:       []byte(d.Name),
		BLSPubKey:     d.BLSPubKey,
	}
}

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l CandidateList) Less(i, j int) bool {
	if res := l[i].Votes.Cmp(l[j].Votes); res != 0 {
		return res == 1
	}
	if res := strings.Compare(l[i].Owner.String(), l[j].Owner.String()); res != 0 {
		return res == 1
	}
	if res := strings.Compare(l[i].Reward.String(), l[j].Reward.String()); res != 0 {
		return res == 1
	}
	if res := strings.Compare(l[i].Operator.String(), l[j].Operator.String()); res != 0 {
		return res == 1
	}
	switch {
	case l[i].SelfStakeBucketIdx > l[j].SelfStakeBucketIdx:
		return true
	case l[i].SelfStakeBucketIdx < l[j].SelfStakeBucketIdx:
		return false
	}
	if res := l[i].SelfStake.Cmp(l[j].SelfStake); res != 0 {
		return res == 1
	}
	return strings.Compare(l[i].Name, l[j].Name) == 1
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

// Encode encodes candidate list into generic value
func (l *CandidateList) Encode() (systemcontracts.GenericValue, error) {
	data, err := l.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, errors.Wrap(err, "failed to serialize candidate list")
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode decodes candidate list from generic value
func (l *CandidateList) Decode(gv systemcontracts.GenericValue) error {
	return l.Deserialize(gv.PrimaryData)
}

func (l CandidateList) toStateCandidateList() (state.CandidateList, error) {
	list := make(state.CandidateList, 0, len(l))
	for _, c := range l {
		list = append(list, c.toStateCandidate())
	}
	sort.Sort(list)
	return list, nil
}
