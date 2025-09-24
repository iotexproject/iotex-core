package stakingindex

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex/stakingpb"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type CandidateVotes interface {
	Clone() CandidateVotes
	Votes(fCtx protocol.FeatureCtx, cand string) *big.Int
	Add(cand string, amount *big.Int, votes *big.Int)
	Clear()
	Commit() CandidateVotes
	Base() CandidateVotes
	IsDirty() bool
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

type candidate struct {
	// total stake amount of candidate
	amount *big.Int
	// total weighted votes of candidate
	votes *big.Int
}

type candidateVotes struct {
	cands map[string]*candidate
}

type candidateVotesWraper struct {
	base   CandidateVotes
	change *candidateVotes
}

type candidateVotesWraperCommitInClone struct {
	*candidateVotesWraper
}

func newCandidate() *candidate {
	return &candidate{
		amount: big.NewInt(0),
		votes:  big.NewInt(0),
	}
}

func (cv *candidateVotes) Clone() CandidateVotes {
	newCands := make(map[string]*candidate)
	for cand, c := range cv.cands {
		newCands[cand] = &candidate{
			amount: new(big.Int).Set(c.amount),
			votes:  new(big.Int).Set(c.votes),
		}
	}
	return &candidateVotes{
		cands: newCands,
	}
}

func (cv *candidateVotes) IsDirty() bool {
	return false
}

func (cv *candidateVotes) Votes(fCtx protocol.FeatureCtx, cand string) *big.Int {
	c := cv.cands[cand]
	if c == nil {
		return nil
	}
	if !fCtx.FixContractStakingWeightedVotes {
		return c.amount
	}
	return c.votes
}

func (cv *candidateVotes) Add(cand string, amount *big.Int, votes *big.Int) {
	if cv.cands[cand] == nil {
		cv.cands[cand] = newCandidate()
	}
	if amount != nil {
		cv.cands[cand].amount = new(big.Int).Add(cv.cands[cand].amount, amount)
	}
	if votes != nil {
		cv.cands[cand].votes = new(big.Int).Add(cv.cands[cand].votes, votes)
	}
}

func (cv *candidateVotes) Clear() {
	cv.cands = make(map[string]*candidate)
}

func (cv *candidateVotes) Serialize() ([]byte, error) {
	cl := stakingpb.CandidateList{}
	for cand, c := range cv.cands {
		cl.Candidates = append(cl.Candidates, &stakingpb.Candidate{
			Address: cand,
			Votes:   c.votes.String(),
			Amount:  c.amount.String(),
		})
	}
	return proto.Marshal(&cl)
}

func (cv *candidateVotes) Deserialize(data []byte) error {
	cl := stakingpb.CandidateList{}
	if err := proto.Unmarshal(data, &cl); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate list")
	}
	for _, c := range cl.Candidates {
		votes, ok := new(big.Int).SetString(c.Votes, 10)
		if !ok {
			return errors.Errorf("failed to parse votes: %s", c.Votes)
		}
		amount, ok := new(big.Int).SetString(c.Amount, 10)
		if !ok {
			return errors.Errorf("failed to parse amount: %s", c.Amount)
		}
		cv.Add(c.Address, amount, votes)
	}
	return nil
}

func (cv *candidateVotes) Commit() CandidateVotes {
	return cv
}

func (cv *candidateVotes) Base() CandidateVotes {
	return cv
}

func (cv *candidateVotes) ContractStorageAddress(ns string) (address.Address, error) {
	return systemcontracts.SystemContracts[systemcontracts.StakingViewContractIndex].Address, nil
}

func (cv *candidateVotes) New(data []byte) (any, error) {
	c := &candidateVotes{
		cands: make(map[string]*candidate),
	}
	if err := c.Deserialize(data); err != nil {
		return nil, err
	}
	return c, nil
}

func (cv *candidateVotes) ContractStorageProxy() erigonstore.ContractStorage {
	return erigonstore.NewContractStorageNamespacedWrapper(cv)
}

func newCandidateVotes() *candidateVotes {
	return &candidateVotes{
		cands: make(map[string]*candidate),
	}
}

func newCandidateVotesWrapper(base CandidateVotes) *candidateVotesWraper {
	return &candidateVotesWraper{
		base:   base,
		change: newCandidateVotes(),
	}
}

func (cv *candidateVotesWraper) Clone() CandidateVotes {
	return &candidateVotesWraper{
		base:   cv.base.Clone(),
		change: cv.change.Clone().(*candidateVotes),
	}
}

func (cv *candidateVotesWraper) IsDirty() bool {
	return cv.change.IsDirty() || cv.base.IsDirty()
}

func (cv *candidateVotesWraper) Votes(fCtx protocol.FeatureCtx, cand string) *big.Int {
	base := cv.base.Votes(fCtx, cand)
	change := cv.change.Votes(fCtx, cand)
	if change == nil {
		return base
	}
	if base == nil {
		return change
	}
	return new(big.Int).Add(base, change)
}

func (cv *candidateVotesWraper) Add(cand string, amount *big.Int, votes *big.Int) {
	cv.change.Add(cand, amount, votes)
}

func (cv *candidateVotesWraper) Clear() {
	cv.change.Clear()
	cv.base.Clear()
}

func (cv *candidateVotesWraper) Commit() CandidateVotes {
	// Commit the changes to the base
	for cand, change := range cv.change.cands {
		cv.base.Add(cand, change.amount, change.votes)
	}
	cv.change = newCandidateVotes()
	// base commit
	return cv.base.Commit()
}

func (cv *candidateVotesWraper) Serialize() ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (cv *candidateVotesWraper) Deserialize(data []byte) error {
	return errors.New("not implemented")
}

func (cv *candidateVotesWraper) Base() CandidateVotes {
	return cv.base
}

func newCandidateVotesWrapperCommitInClone(base CandidateVotes) *candidateVotesWraperCommitInClone {
	return &candidateVotesWraperCommitInClone{
		candidateVotesWraper: newCandidateVotesWrapper(base),
	}
}

func (cv *candidateVotesWraperCommitInClone) Clone() CandidateVotes {
	return &candidateVotesWraperCommitInClone{
		candidateVotesWraper: cv.candidateVotesWraper.Clone().(*candidateVotesWraper),
	}
}

func (cv *candidateVotesWraperCommitInClone) Commit() CandidateVotes {
	cv.base = cv.base.Clone()
	return cv.candidateVotesWraper.Commit()
}
