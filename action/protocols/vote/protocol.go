package vote

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
)

// VoteSizeLimit is the maximum size of vote allowed
const VoteSizeLimit = 278

// Protocol defines the protocol of handling votes
type Protocol struct{ bc blockchain.Blockchain }

// NewProtocol instantiates the protocol of vote
func NewProtocol(bc blockchain.Blockchain) *Protocol { return &Protocol{bc} }

// Handle handles a vote
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil
	}
	voteFrom, err := ws.LoadOrCreateAccountState(vote.Voter(), big.NewInt(0))
	if err != nil {
		return errors.Wrapf(err, "failed to load or create the account of voter %s", vote.Voter())
	}
	// update voteFrom Nonce
	if vote.Nonce() > voteFrom.Nonce {
		voteFrom.Nonce = vote.Nonce()
	}
	// Update old votee's weight
	if len(voteFrom.Votee) > 0 && voteFrom.Votee != vote.Voter() {
		// voter already voted
		oldVotee, err := ws.LoadOrCreateAccountState(voteFrom.Votee, big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of voter's old votee %s", voteFrom.Votee)
		}
		oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
		voteFrom.Votee = ""
	}

	if vote.Votee() == "" {
		// unvote operation
		voteFrom.IsCandidate = false
		return nil
	}

	voteTo, err := ws.LoadOrCreateAccountState(vote.Votee(), big.NewInt(0))
	if err != nil {
		return errors.Wrapf(err, "failed to load or create the account of votee %s", vote.Votee())
	}
	if vote.Voter() != vote.Votee() {
		// Voter votes to a different person
		voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)
		voteFrom.Votee = vote.Votee()
	} else {
		// Vote to self: self-nomination or cancel the previous vote case
		voteFrom.Votee = vote.Voter()
		voteFrom.IsCandidate = true
		if err := ws.UpdateCachedCandidates(vote); err != nil {
			return errors.Wrap(err, "failed to update cached candidates in working set")
		}
	}
	return nil
}

// Validate validates a vote
func (p *Protocol) Validate(act action.Action) error {
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
		if _, err := iotxaddress.GetPubkeyHash(vote.Votee()); err != nil {
			return errors.Wrapf(err, "error when validating votee's address %s", vote.Votee())
		}
	}
	if vote.Votee() != "" {
		// Reject vote if votee is not a candidate
		voteeState, err := p.bc.StateByAddr(vote.Votee())
		if err != nil {
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.Votee())
		}
		if vote.Voter() != vote.Votee() && !voteeState.IsCandidate {
			return errors.Wrapf(action.ErrVotee, "votee has not self-nominated: %s", vote.Votee())
		}
	}
	return nil
}
