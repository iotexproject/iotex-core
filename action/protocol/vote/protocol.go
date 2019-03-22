// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// VoteSizeLimit is the maximum size of vote allowed
	VoteSizeLimit = 278

	// ProtocolID is the protocol ID
	// TODO: it works only for one instance per protocol definition now
	ProtocolID = "vote"
)

// Protocol defines the protocol of handling votes
type Protocol struct {
	cm protocol.ChainManager
}

// NewProtocol instantiates the protocol of vote
func NewProtocol(cm protocol.ChainManager) *Protocol { return &Protocol{cm: cm} }

// Initialize initializes the rewarding protocol by setting the original admin, block and epoch reward
func (p *Protocol) Initialize(ctx context.Context, sm protocol.StateManager, addrs []address.Address) error {
	for _, addr := range addrs {
		selfNominator, err := accountutil.LoadOrCreateAccount(sm, addr.String(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of self nominator %s", addr)
		}
		selfNominator.IsCandidate = true
		if err := candidatesutil.LoadAndAddCandidates(sm, 0, addr.String()); err != nil {
			return err
		}
		if err := accountutil.StoreAccount(sm, addr.String(), selfNominator); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		if err := candidatesutil.LoadAndUpdateCandidates(sm, 0, addr.String(), selfNominator.Balance); err != nil {
			return errors.Wrap(err, "failed to load and update candidates")
		}

	}
	return nil
}

// Handle handles a vote
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil, nil
	}

	voteFrom, err := accountutil.LoadOrCreateAccount(sm, raCtx.Caller.String(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of voter %s", raCtx.Caller.String())
	}

	if raCtx.GasLimit < raCtx.IntrinsicGas {
		return nil, action.ErrHitGasLimit
	}
	gasFee := big.NewInt(0).Mul(raCtx.GasPrice, big.NewInt(0).SetUint64(raCtx.IntrinsicGas))
	if gasFee.Cmp(voteFrom.Balance) == 1 {
		return nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"failed to verify the Balance for gas of voter %s, %d, %d",
			raCtx.Caller.String(),
			raCtx.IntrinsicGas,
			voteFrom.Balance,
		)
	}
	// charge voter Gas
	if err := voteFrom.SubBalance(gasFee); err != nil {
		return nil, errors.Wrapf(err, "failed to charge the gas for voter %s", raCtx.Caller.String())
	}
	if err := rewarding.DepositGas(ctx, sm, gasFee, raCtx.Registry); err != nil {
		return nil, err
	}
	// Update voteFrom Nonce
	accountutil.SetNonce(vote, voteFrom)
	prevVotee := voteFrom.Votee
	voteFrom.Votee = vote.Votee()
	if vote.Votee() == "" {
		// unvote operation
		voteFrom.IsCandidate = false
		// Remove the candidate from candidateMap if the person is not a candidate anymore
		if err := candidatesutil.LoadAndDeleteCandidates(sm, raCtx.BlockHeight, raCtx.Caller.String()); err != nil {
			return nil, errors.Wrap(err, "failed to load and delete candidates")
		}
	} else if raCtx.Caller.String() == vote.Votee() {
		// Vote to self: self-nomination
		voteFrom.IsCandidate = true
		addr, err := address.FromBytes(vote.VoterPublicKey().Hash())
		if err != nil {
			return nil, err
		}
		if err := candidatesutil.LoadAndAddCandidates(sm, raCtx.BlockHeight, addr.String()); err != nil {
			return nil, errors.Wrap(err, "failed to load and add candidates")
		}
	}
	// Put updated voter's state to trie
	if err := accountutil.StoreAccount(sm, raCtx.Caller.String(), voteFrom); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	// Update old votee's weight
	if len(prevVotee) > 0 {
		// voter already voted
		oldVotee, err := accountutil.LoadOrCreateAccount(sm, prevVotee, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of voter's old votee %s", prevVotee)
		}
		oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
		// Put updated state of voter's old votee to trie
		if err := accountutil.StoreAccount(sm, prevVotee, oldVotee); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if oldVotee.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, raCtx.BlockHeight, prevVotee, oldVotee.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}

	if vote.Votee() != "" {
		voteTo, err := accountutil.LoadOrCreateAccount(sm, vote.Votee(), big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of votee %s", vote.Votee())
		}
		// Update new votee's weight
		voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)

		// Put updated votee's state to trie
		if err := accountutil.StoreAccount(sm, vote.Votee(), voteTo); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update candidate map
		if voteTo.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, raCtx.BlockHeight, vote.Votee(), voteTo.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}

	return &action.Receipt{
		Status:      action.SuccessReceiptStatus,
		ActHash:     raCtx.ActionHash,
		GasConsumed: raCtx.IntrinsicGas,
	}, nil
}

// Validate validates a vote
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	vaCtx := protocol.MustGetValidateActionsCtx(ctx)

	vote, ok := act.(*action.Vote)
	if !ok {
		if _, ok := act.(*action.PutPollResult); ok {
			return errors.New("with vote protocol, put poll result action cannot be processed")
		}
		return nil
	}
	// Reject oversized vote
	if vote.TotalSize() > VoteSizeLimit {
		return errors.Wrapf(action.ErrActPool, "oversized data")
	}
	// check if votee's address is valid
	if vote.Votee() != action.EmptyAddress {
		if _, err := address.FromString(vote.Votee()); err != nil {
			return errors.Wrapf(err, "error when validating votee's address %s", vote.Votee())
		}
	}
	if vote.Votee() != "" {
		// Reject vote if votee is not a candidate
		voteeState, err := p.cm.StateByAddr(vote.Votee())
		if err != nil {
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.Votee())
		}
		if vaCtx.Caller.String() != vote.Votee() && !voteeState.IsCandidate {
			return errors.Wrapf(action.ErrVotee, "votee has not self-nominated: %s", vote.Votee())
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}
