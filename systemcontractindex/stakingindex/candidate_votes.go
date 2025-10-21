package stakingindex

import (
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex/stakingpb"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

var (
	// ErrCandidateVotesIsDirty is returned when candidate votes are dirty
	ErrCandidateVotesIsDirty = errors.New("candidate votes is dirty")
)

// CandidateVotes is the interface to manage candidate votes
type CandidateVotes interface {
	Clone() CandidateVotes
	Votes(fCtx protocol.FeatureCtx, cand string) *big.Int
	Add(cand string, amount *big.Int, votes *big.Int)
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

type candidateVotesWithBuffer struct {
	base   *candidateVotes
	change *candidateVotes
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

func (cv *candidateVotes) Clone() *candidateVotes {
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

func (cv *candidateVotes) Serialize() ([]byte, error) {
	cl := stakingpb.CandidateList{}

	// Create a slice of candidate addresses and sort them for consistent ordering
	addresses := make([]string, 0, len(cv.cands))
	for cand := range cv.cands {
		addresses = append(addresses, cand)
	}
	sort.Strings(addresses)

	// Add candidates in sorted order
	for _, cand := range addresses {
		c := cv.cands[cand]
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

func (cv *candidateVotes) Encode() (systemcontracts.GenericValue, error) {
	data, err := cv.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (cv *candidateVotes) Decode(data systemcontracts.GenericValue) error {
	return cv.Deserialize(data.PrimaryData)
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
		change: cv.change.Clone(),
	}
}

func (cv *candidateVotesWraper) IsDirty() bool {
	return len(cv.change.cands) > 0 || cv.base.IsDirty()
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
	if cv.IsDirty() {
		return nil, errors.Wrap(ErrCandidateVotesIsDirty, "cannot serialize dirty candidate votes")
	}
	return cv.base.Serialize()
}

func (cv *candidateVotesWraper) Deserialize(data []byte) error {
	cv.change = newCandidateVotes()
	return cv.base.Deserialize(data)
}

func (cv *candidateVotesWraper) Base() CandidateVotes {
	return cv.base.Base()
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

func (cv *candidateVotesWraperCommitInClone) Base() CandidateVotes {
	return cv.base
}

func newCandidateVotesWithBuffer(base *candidateVotes) *candidateVotesWithBuffer {
	return &candidateVotesWithBuffer{
		base:   base,
		change: newCandidateVotes(),
	}
}

func (cv *candidateVotesWithBuffer) Clone() CandidateVotes {
	return &candidateVotesWithBuffer{
		base:   cv.base.Clone(),
		change: cv.change.Clone(),
	}
}

func (cv *candidateVotesWithBuffer) IsDirty() bool {
	return len(cv.change.cands) > 0
}

func (cv *candidateVotesWithBuffer) Votes(fCtx protocol.FeatureCtx, cand string) *big.Int {
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

func (cv *candidateVotesWithBuffer) Add(cand string, amount *big.Int, votes *big.Int) {
	cv.change.Add(cand, amount, votes)
}

func (cv *candidateVotesWithBuffer) Commit() CandidateVotes {
	// Commit the changes to the base
	for cand, change := range cv.change.cands {
		cv.base.Add(cand, change.amount, change.votes)
	}
	cv.change = newCandidateVotes()
	return cv
}

func (cv *candidateVotesWithBuffer) Serialize() ([]byte, error) {
	if cv.IsDirty() {
		return nil, errors.Wrap(ErrCandidateVotesIsDirty, "cannot serialize dirty candidate votes")
	}
	return cv.base.Serialize()
}

func (cv *candidateVotesWithBuffer) Deserialize(data []byte) error {
	cv.change = newCandidateVotes()
	return cv.base.Deserialize(data)
}

func (cv *candidateVotesWithBuffer) Base() CandidateVotes {
	return newCandidateVotesWithBuffer(cv.base)
}

func (cv *candidateVotesWithBuffer) Encode() (systemcontracts.GenericValue, error) {
	if cv.IsDirty() {
		return systemcontracts.GenericValue{}, errors.Wrap(ErrCandidateVotesIsDirty, "cannot encode dirty candidate votes")
	}
	return cv.base.Encode()
}

func (cv *candidateVotesWithBuffer) Decode(data systemcontracts.GenericValue) error {
	cv.change = newCandidateVotes()
	return cv.base.Decode(data)
}
