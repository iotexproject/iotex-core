// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package types

import (
	"bytes"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"

	pb "github.com/iotexproject/iotex-election/pb/election"
)

// Vote defines the structure of a vote
type Vote struct {
	startTime time.Time
	duration  time.Duration
	amount    *big.Int
	weighted  *big.Int
	voter     []byte
	candidate []byte
	decay     bool
}

// NewVote creates a new vote
func NewVote(
	startTime time.Time,
	duration time.Duration,
	amount *big.Int,
	weighted *big.Int,
	voter []byte,
	candidate []byte,
	decay bool,
) (*Vote, error) {
	if duration < 0 {
		return nil, errors.Errorf("duration %s cannot be negative", duration)
	}
	if amount == nil || big.NewInt(0).Cmp(amount) > 0 {
		return nil, errors.Errorf("amount %s cannot be nil or negative", amount)
	}
	if weighted == nil || big.NewInt(0).Cmp(weighted) > 0 {
		return nil, errors.Errorf("weighted amount %s cannot be nil or negative", weighted)
	}
	cVoter := make([]byte, len(voter))
	copy(cVoter, voter)
	cCandidate := make([]byte, len(candidate))
	copy(cCandidate, candidate)

	return &Vote{
		startTime: startTime,
		duration:  duration,
		amount:    new(big.Int).Set(amount),
		weighted:  new(big.Int).Set(weighted),
		voter:     cVoter,
		candidate: cCandidate,
		decay:     decay,
	}, nil
}

// Clone clones the vote
func (v *Vote) Clone() *Vote {
	return &Vote{
		v.StartTime(),
		v.Duration(),
		v.Amount(),
		v.WeightedAmount(),
		v.Voter(),
		v.Candidate(),
		v.Decay(),
	}
}

// SetWeightedAmount sets the weighted amount for the vote
func (v *Vote) SetWeightedAmount(w *big.Int) error {
	if w == nil || big.NewInt(0).Cmp(w) > 0 {
		return errors.New("weighted amount cannot be negative")
	}
	v.weighted = new(big.Int).Set(w)

	return nil
}

// StartTime returns the start time
func (v *Vote) StartTime() time.Time {
	return v.startTime
}

// Duration returns the duration this vote for
func (v *Vote) Duration() time.Duration {
	return v.duration
}

// Voter returns the voter address in bytes
func (v *Vote) Voter() []byte {
	voter := make([]byte, len(v.voter))
	copy(voter, v.voter)

	return voter
}

// Amount returns the amount of vote
func (v *Vote) Amount() *big.Int {
	return new(big.Int).Set(v.amount)
}

// WeightedAmount returns the weighted amount of vote
func (v *Vote) WeightedAmount() *big.Int {
	return new(big.Int).Set(v.weighted)
}

// Candidate returns the candidate
func (v *Vote) Candidate() []byte {
	candidate := make([]byte, len(v.candidate))
	copy(candidate, v.candidate)

	return candidate
}

// Decay returns whether this is a decay vote
func (v *Vote) Decay() bool {
	return v.decay
}

// RemainingTime returns the remaining time to given time
func (v *Vote) RemainingTime(now time.Time) time.Duration {
	if now.Before(v.startTime) {
		return 0
	}
	if v.decay {
		endTime := v.startTime.Add(v.duration)
		if endTime.After(now) {
			return v.startTime.Add(v.duration).Sub(now)
		}
		return 0
	}
	return v.duration
}

// ToProtoMsg converts the vote to protobuf
func (v *Vote) ToProtoMsg() (*pb.Vote, error) {
	startTime, err := ptypes.TimestampProto(v.startTime)
	if err != nil {
		return nil, err
	}
	return &pb.Vote{
		Voter:          v.Voter(),
		Candidate:      v.Candidate(),
		Amount:         v.amount.Bytes(),
		WeightedAmount: v.weighted.Bytes(),
		StartTime:      startTime,
		Duration:       ptypes.DurationProto(v.duration),
		Decay:          v.decay,
	}, nil
}

// Serialize serializes the vote to bytes
func (v *Vote) Serialize() ([]byte, error) {
	vPb, err := v.ToProtoMsg()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(vPb)
}

// FromProtoMsg extracts vote details from protobuf message
func (v *Vote) FromProtoMsg(vPb *pb.Vote) (err error) {
	voter := make([]byte, len(vPb.Voter))
	copy(voter, vPb.Voter)
	v.voter = voter
	candidate := make([]byte, len(vPb.Candidate))
	copy(candidate, vPb.Candidate)
	v.candidate = candidate
	v.amount = big.NewInt(0).SetBytes(vPb.Amount)
	v.weighted = new(big.Int).SetBytes(vPb.WeightedAmount)
	if v.startTime, err = ptypes.Timestamp(vPb.StartTime); err != nil {
		return err
	}
	if v.duration, err = ptypes.Duration(vPb.Duration); err != nil {
		return err
	}
	if v.duration < 0 {
		return errors.Errorf("duration %s cannot be negative", v.duration)
	}
	v.decay = vPb.Decay

	return nil
}

// Deserialize deserializes a byte array to vote
func (v *Vote) Deserialize(data []byte) error {
	vPb := &pb.Vote{}
	if err := proto.Unmarshal(data, vPb); err != nil {
		return err
	}

	return v.FromProtoMsg(vPb)
}

func (v *Vote) equal(vote *Vote) bool {
	if v == vote {
		return true
	}
	if v == nil || vote == nil {
		return false
	}
	if !v.startTime.Equal(vote.startTime) {
		return false
	}
	if v.duration != vote.duration {
		return false
	}
	if v.amount.Cmp(vote.amount) != 0 {
		return false
	}
	if v.weighted.Cmp(vote.weighted) != 0 {
		return false
	}
	if !bytes.Equal(v.voter, vote.voter) {
		return false
	}
	if !bytes.Equal(v.candidate, vote.candidate) {
		return false
	}
	return v.decay == vote.decay
}
