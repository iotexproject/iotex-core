package stakingindex

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
)

var (
	voteViewKeyPrefix = []byte("voteview")
	voteViewNS        = state.StakingViewNamespace
)

// CandidateVotesManager defines the interface to manage candidate votes
type CandidateVotesManager interface {
	Load(ctx context.Context, sr protocol.StateReader) (CandidateVotes, error)
	Store(ctx context.Context, sm protocol.StateManager, candVotes CandidateVotes) error
}

type candidateVotesManager struct {
	contractAddr address.Address
}

// NewCandidateVotesManager creates a new instance of CandidateVotesManager
func NewCandidateVotesManager(contractAddr address.Address) CandidateVotesManager {
	return &candidateVotesManager{
		contractAddr: contractAddr,
	}
}

func (s *candidateVotesManager) Store(ctx context.Context, sm protocol.StateManager, candVotes CandidateVotes) error {
	if _, err := sm.PutState(candVotes,
		protocol.KeyOption(s.key()),
		protocol.NamespaceOption(voteViewNS),
	); err != nil {
		return errors.Wrap(err, "failed to put candidate votes state")
	}
	return nil
}

func (s *candidateVotesManager) Load(ctx context.Context, sr protocol.StateReader) (CandidateVotes, error) {
	cur := newCandidateVotes()
	_, err := sr.State(cur,
		protocol.KeyOption(s.key()),
		protocol.NamespaceOption(voteViewNS),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get candidate votes state")
	}
	return newCandidateVotesWithBuffer(cur), nil
}

func (s *candidateVotesManager) key() []byte {
	return append(voteViewKeyPrefix, s.contractAddr.Bytes()...)
}
