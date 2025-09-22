package stakingindex

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

var (
	voteViewKey      = []byte("voteview")
	voteViewNSPrefix = "voterview"
)

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
		protocol.KeyOption(voteViewKey),
		protocol.NamespaceOption(s.namespace()),
		protocol.SecondaryOnlyOption(),
	); err != nil {
		return errors.Wrap(err, "failed to put candidate votes state")
	}
	return nil
}

func (s *candidateVotesManager) Load(ctx context.Context, sr protocol.StateReader) (CandidateVotes, error) {
	cur := newCandidateVotes()
	_, err := sr.State(cur,
		protocol.KeyOption(voteViewKey),
		protocol.NamespaceOption(s.namespace()),
		protocol.SecondaryOnlyOption(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get candidate votes state")
	}
	return cur, nil
}

func (s *candidateVotesManager) namespace() string {
	return voteViewNSPrefix + s.contractAddr.String()
}
