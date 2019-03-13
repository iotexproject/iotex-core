// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package types

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"

	pb "github.com/iotexproject/iotex-election/pb/election"
)

// ErrInvalidProto indicates a format error of an election proto
var ErrInvalidProto = errors.New("Invalid election proto")

// ElectionResult defines the collection of voting result on a height
type ElectionResult struct {
	mintTime         time.Time
	delegates        []*Candidate
	votes            map[string][]*Vote
	totalVotes       *big.Int
	totalVotedStakes *big.Int
}

// MintTime returns the mint time of the corresponding beacon chain block
func (r *ElectionResult) MintTime() time.Time {
	return r.mintTime
}

// Delegates returns a list of sorted delegates
func (r *ElectionResult) Delegates() []*Candidate {
	return r.delegates
}

// VotesByDelegate returns a list of votes for a given delegate
func (r *ElectionResult) VotesByDelegate(name []byte) []*Vote {
	return r.votes[hex.EncodeToString(name)]
}

// TotalVotes returns the total votes in the result
func (r *ElectionResult) TotalVotes() *big.Int {
	return new(big.Int).Set(r.totalVotes)
}

// TotalVotedStakes returns the total amount of stakings which has been voted
func (r *ElectionResult) TotalVotedStakes() *big.Int {
	return new(big.Int).Set(r.totalVotedStakes)
}

// ToProtoMsg converts the vote to protobuf
func (r *ElectionResult) ToProtoMsg() (*pb.ElectionResult, error) {
	delegates := make([]*pb.Candidate, len(r.delegates))
	delegateVotes := make([]*pb.VoteList, len(r.votes))
	var err error
	for i := 0; i < len(r.delegates); i++ {
		delegate := r.delegates[i]
		if delegates[i], err = delegate.ToProtoMsg(); err != nil {
			return nil, err
		}
		name := hex.EncodeToString(delegate.Name())
		votes, ok := r.votes[name]
		if !ok {
			return nil, errors.Errorf("Cannot find votes for delegate %s", name)
		}
		voteList := make([]*pb.Vote, len(votes))
		for j := 0; j < len(votes); j++ {
			if voteList[j], err = votes[j].ToProtoMsg(); err != nil {
				return nil, err
			}
		}
		delegateVotes[i] = &pb.VoteList{Votes: voteList}
	}
	t, err := ptypes.TimestampProto(r.mintTime)
	if err != nil {
		return nil, err
	}

	return &pb.ElectionResult{
		Timestamp:        t,
		Delegates:        delegates,
		DelegateVotes:    delegateVotes,
		TotalVotedStakes: r.totalVotedStakes.Bytes(),
		TotalVotes:       r.totalVotes.Bytes(),
	}, nil
}

// Serialize converts result to byte array
func (r *ElectionResult) Serialize() ([]byte, error) {
	rPb, err := r.ToProtoMsg()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(rPb)
}

// FromProtoMsg extracts result details from protobuf message
func (r *ElectionResult) FromProtoMsg(rPb *pb.ElectionResult) (err error) {
	if len(rPb.Delegates) != len(rPb.DelegateVotes) {
		return errors.Wrapf(
			ErrInvalidProto,
			"size of delegate list %d is different from score list %d",
			len(rPb.Delegates),
			len(rPb.DelegateVotes),
		)
	}
	r.votes = map[string][]*Vote{}
	r.delegates = make([]*Candidate, len(rPb.Delegates))
	for i, cPb := range rPb.Delegates {
		r.delegates[i] = &Candidate{}
		if err := r.delegates[i].FromProtoMsg(cPb); err != nil {
			return err
		}
		name := hex.EncodeToString(r.delegates[i].Name())
		if _, ok := r.votes[name]; ok {
			return errors.Wrapf(
				ErrInvalidProto,
				"duplicate delegate %s",
				name,
			)
		}
		voteList := rPb.DelegateVotes[i]
		r.votes[name] = make([]*Vote, len(voteList.Votes))
		for j, vPb := range voteList.Votes {
			r.votes[name][j] = &Vote{}
			if err := r.votes[name][j].FromProtoMsg(vPb); err != nil {
				return err
			}
		}
	}
	if r.mintTime, err = ptypes.Timestamp(rPb.Timestamp); err != nil {
		return err
	}
	r.totalVotedStakes = new(big.Int).SetBytes(rPb.TotalVotedStakes)
	r.totalVotes = new(big.Int).SetBytes(rPb.TotalVotes)

	return nil
}

// Deserialize converts a byte array to election result
func (r *ElectionResult) Deserialize(data []byte) error {
	pb := &pb.ElectionResult{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return err
	}

	return r.FromProtoMsg(pb)
}

// NewElectionResultForTest creates an election result for test purpose only
func NewElectionResultForTest(
	mintTime time.Time,
) *ElectionResult {
	return &ElectionResult{
		mintTime: mintTime,
		delegates: []*Candidate{
			&Candidate{
				name:            []byte("name1"),
				address:         []byte("address1"),
				operatorAddress: []byte("io1kfpsvefk74cqxd245j2h5t2pld2wtxzyg6tqrt"),
				rewardAddress:   []byte("io1kfpsvefk74cqxd245j2h5t2pld2wtxzyg6tqrt"),
				score:           big.NewInt(15),
			},
			&Candidate{
				name:            []byte("name2"),
				address:         []byte("address2"),
				operatorAddress: []byte("io1llr6zs37gxrwmvnczexpg35dptta2mdvjv6w2q"),
				rewardAddress:   []byte("io1llr6zs37gxrwmvnczexpg35dptta2mdvjv6w2q"),
				score:           big.NewInt(14),
			},
		},
		votes: map[string][]*Vote{
			"name1": []*Vote{},
			"name2": []*Vote{},
		},
	}
}
