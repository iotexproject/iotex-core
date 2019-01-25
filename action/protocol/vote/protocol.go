// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-ethereum/core/vm"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// VoteSizeLimit is the maximum size of vote allowed
	VoteSizeLimit = 278
)

// Protocol defines the protocol of handling votes
type Protocol struct {
	cm protocol.ChainManager
}

// NewProtocol instantiates the protocol of vote
func NewProtocol(cm protocol.ChainManager) *Protocol { return &Protocol{cm: cm} }

// Handle handles a vote
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil, nil
	}

	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return nil, errors.New("failed to get action context")
	}

	voteFrom, err := account.LoadOrCreateAccount(sm, vote.Voter(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of voter %s", vote.Voter())
	}
	if raCtx.EnableGasCharge {
		// Load or create account for producer
		producer, err := account.LoadOrCreateAccount(sm, raCtx.ProducerAddr, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of block producer %s", raCtx.ProducerAddr)
		}
		gas, err := vote.IntrinsicGas()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get intrinsic gas for vote hash %s", vote.Hash())
		}
		if *raCtx.GasLimit < gas {
			return nil, vm.ErrOutOfGas
		}
		gasFee := big.NewInt(0).Mul(vote.GasPrice(), big.NewInt(0).SetUint64(gas))

		if gasFee.Cmp(voteFrom.Balance) == 1 {
			return nil, errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance for gas of voter %s, %d, %d", vote.Voter(), gas, voteFrom.Balance)
		}

		// charge voter Gas
		if err := voteFrom.SubBalance(gasFee); err != nil {
			return nil, errors.Wrapf(err, "failed to charge the gas for voter %s", vote.Voter())
		}
		// compensate block producer gas
		if err := producer.AddBalance(gasFee); err != nil {
			return nil, errors.Wrapf(err, "failed to compensate gas to producer")
		}
		// Put updated producer's state to trie
		if err := account.StoreAccount(sm, raCtx.ProducerAddr, producer); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		*raCtx.GasLimit -= gas
	}
	// Update voteFrom Nonce
	account.SetNonce(vote, voteFrom)
	prevVotee := voteFrom.Votee
	voteFrom.Votee = vote.Votee()
	if vote.Votee() == "" {
		// unvote operation
		voteFrom.IsCandidate = false
		// Remove the candidate from candidateMap if the person is not a candidate anymore
		if err := candidatesutil.LoadAndDeleteCandidates(sm, vote.Voter()); err != nil {
			return nil, errors.Wrap(err, "failed to load and delete candidates")
		}
	} else if vote.Voter() == vote.Votee() {
		// Vote to self: self-nomination
		voteFrom.IsCandidate = true
		if err := candidatesutil.LoadAndAddCandidates(sm, vote); err != nil {
			return nil, errors.Wrap(err, "failed to load and add candidates")
		}
	}
	// Put updated voter's state to trie
	if err := account.StoreAccount(sm, vote.Voter(), voteFrom); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	// Update old votee's weight
	if len(prevVotee) > 0 {
		// voter already voted
		oldVotee, err := account.LoadOrCreateAccount(sm, prevVotee, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of voter's old votee %s", prevVotee)
		}
		oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
		// Put updated state of voter's old votee to trie
		if err := account.StoreAccount(sm, prevVotee, oldVotee); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if oldVotee.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, prevVotee, oldVotee.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}

	if vote.Votee() != "" {
		voteTo, err := account.LoadOrCreateAccount(sm, vote.Votee(), big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of votee %s", vote.Votee())
		}
		// Update new votee's weight
		voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)

		// Put updated votee's state to trie
		if err := account.StoreAccount(sm, vote.Votee(), voteTo); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if voteTo.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, vote.Votee(), voteTo.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}

	return nil, nil
}

// Validate validates a vote
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil
	}
	// Reject oversized vote
	if vote.TotalSize() > VoteSizeLimit {
		return errors.Wrapf(action.ErrActPool, "oversized data")
	}
	// check if votee's address is valid
	if vote.Votee() != action.EmptyAddress {
		if _, err := address.Bech32ToAddress(vote.Votee()); err != nil {
			return errors.Wrapf(err, "error when validating votee's address %s", vote.Votee())
		}
	}
	if vote.Votee() != "" {
		// Reject vote if votee is not a candidate
		voteeState, err := p.cm.StateByAddr(vote.Votee())
		if err != nil {
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.Votee())
		}
		if vote.Voter() != vote.Votee() && !voteeState.IsCandidate {
			return errors.Wrapf(action.ErrVotee, "votee has not self-nominated: %s", vote.Votee())
		}
	}
	return nil
}
