// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package types

import (
	"bytes"
	"errors"
	"math/big"

	pb "github.com/iotexproject/iotex-election/pb/election"
	"github.com/iotexproject/iotex-election/util"
)

// Candidate defines a delegate candidate
type Candidate struct {
	name              []byte
	address           []byte
	operatorAddress   []byte
	rewardAddress     []byte
	score             *big.Int
	selfStakingTokens *big.Int
	selfStakingWeight uint64
}

// NewCandidate creates a new candidate with scores as 0s
func NewCandidate(
	name []byte,
	address []byte,
	operatorAddress []byte,
	rewardPubKey []byte,
	selfStakingWeight uint64,
) *Candidate {
	return &Candidate{
		name:              util.CopyBytes(name),
		address:           util.CopyBytes(address),
		operatorAddress:   util.CopyBytes(operatorAddress),
		rewardAddress:     util.CopyBytes(rewardPubKey),
		score:             big.NewInt(0),
		selfStakingTokens: big.NewInt(0),
		selfStakingWeight: selfStakingWeight,
	}
}

// Clone clones the candidate
func (c *Candidate) Clone() *Candidate {
	return &Candidate{
		name:              c.Name(),
		address:           c.Address(),
		operatorAddress:   c.OperatorAddress(),
		rewardAddress:     c.RewardAddress(),
		score:             c.Score(),
		selfStakingTokens: c.SelfStakingTokens(),
		selfStakingWeight: c.SelfStakingWeight(),
	}
}

func (c *Candidate) equal(candidate *Candidate) bool {
	if c == candidate {
		return true
	}
	if c == nil || candidate == nil {
		return false
	}
	if !bytes.Equal(c.name, candidate.name) {
		return false
	}
	if !bytes.Equal(c.address, candidate.address) {
		return false
	}
	if !bytes.Equal(c.operatorAddress, candidate.operatorAddress) {
		return false
	}
	if !bytes.Equal(c.rewardAddress, candidate.rewardAddress) {
		return false
	}
	if c.score.Cmp(candidate.score) != 0 {
		return false
	}
	if c.selfStakingTokens.Cmp(candidate.selfStakingTokens) != 0 {
		return false
	}
	return c.selfStakingWeight == candidate.selfStakingWeight
}

func (c *Candidate) reset() *Candidate {
	c.selfStakingTokens.SetInt64(0)
	c.score.SetInt64(0)
	return c
}

func (c *Candidate) addScore(s *big.Int) error {
	if s.Cmp(big.NewInt(0)) < 0 {
		return errors.New("score cannot be negative")
	}
	c.score.Add(c.score, s)
	return nil
}

func (c *Candidate) addSelfStakingTokens(s *big.Int) error {
	if s.Cmp(big.NewInt(0)) < 0 {
		return errors.New("score cannot be negative")
	}
	c.selfStakingTokens.Add(c.selfStakingTokens, s)
	return nil
}

// Name returns the name of this candidate
func (c *Candidate) Name() []byte {
	return util.CopyBytes(c.name)
}

// Address returns the address of this candidate on beacon chain
func (c *Candidate) Address() []byte {
	return util.CopyBytes(c.address)
}

// OperatorAddress returns the address of the assigned operator on chain
func (c *Candidate) OperatorAddress() []byte {
	return util.CopyBytes(c.operatorAddress)
}

// RewardAddress returns the address of the assigned benefiter on chain
func (c *Candidate) RewardAddress() []byte {
	return util.CopyBytes(c.rewardAddress)
}

// Score returns the total votes (weighted) of this candidate
func (c *Candidate) Score() *big.Int {
	return new(big.Int).Set(c.score)
}

// SelfStakingTokens returns the total self votes (weighted)
func (c *Candidate) SelfStakingTokens() *big.Int {
	return new(big.Int).Set(c.selfStakingTokens)
}

// SelfStakingWeight returns the extra weight for self staking
func (c *Candidate) SelfStakingWeight() uint64 {
	return c.selfStakingWeight
}

// ToProtoMsg converts the instance to a protobuf message
func (c *Candidate) ToProtoMsg() (*pb.Candidate, error) {
	return &pb.Candidate{
		Name:              c.Name(),
		Address:           c.Address(),
		OperatorAddress:   c.OperatorAddress(),
		RewardAddress:     c.RewardAddress(),
		Score:             c.score.Bytes(),
		SelfStakingTokens: c.selfStakingTokens.Bytes(),
		SelfStakingWeight: c.selfStakingWeight,
	}, nil
}

// FromProtoMsg fills the instance with a protobuf message
func (c *Candidate) FromProtoMsg(msg *pb.Candidate) error {
	c.name = util.CopyBytes(msg.GetName())
	c.address = util.CopyBytes(msg.GetAddress())
	c.operatorAddress = util.CopyBytes(msg.GetOperatorAddress())
	c.rewardAddress = util.CopyBytes(msg.GetRewardAddress())
	c.score = new(big.Int).SetBytes(msg.GetScore())
	c.selfStakingTokens = new(big.Int).SetBytes(msg.GetSelfStakingTokens())
	c.selfStakingWeight = msg.GetSelfStakingWeight()

	return nil
}
