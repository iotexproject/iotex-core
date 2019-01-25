package election

import (
	"math/big"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/election/electionpb"
)

// ErrInvalidProto indicates a format error of an election proto
var ErrInvalidProto = errors.New("Invalid election proto")

// Result is the collection of election result for an epoch
type Result struct {
	scores map[string]*big.Int
	mutex  sync.RWMutex
}

// NewResult creates a result instance with specific eth height
func NewResult() *Result {
	return &Result{scores: make(map[string]*big.Int)}
}

// AddPoints adds points for a candidate
func (r *Result) AddPoints(candidate string, points *big.Int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if score, ok := r.scores[candidate]; ok {
		r.scores[candidate] = score.Add(score, points)
		return
	}
	r.scores[candidate] = points
}

func (r *Result) toProto() *electionpb.ElectionResultPb {
	candidates := []string{}
	scores := [][]byte{}
	for candidate, score := range r.scores {
		candidates = append(candidates, candidate)
		scores = append(scores, score.Bytes())
	}

	return &electionpb.ElectionResultPb{
		Candidates: candidates,
		Scores:     scores,
	}
}

// Serialize converts result to byte array
func (r *Result) Serialize() ([]byte, error) {
	return proto.Marshal(r.toProto())
}

func (r *Result) fromProto(pb *electionpb.ElectionResultPb) error {
	if len(pb.Candidates) != len(pb.Scores) {
		return errors.Wrapf(
			ErrInvalidProto,
			"size of candidate list %d is different from score list %d",
			len(pb.Candidates),
			len(pb.Scores),
		)
	}
	newScores := make(map[string]*big.Int)
	for i, candidate := range pb.Candidates {
		score := big.NewInt(0)
		score = score.SetBytes(pb.Scores[i])
		if _, ok := newScores[candidate]; ok {
			return errors.Wrapf(
				ErrInvalidProto,
				"duplicate candidate %s",
				candidate,
			)
		}
		newScores[candidate] = score
	}
	r.scores = newScores

	return nil
}

// Deserialize converts a byte array to election result
func (r *Result) Deserialize(data []byte) error {
	pb := &electionpb.ElectionResultPb{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return err
	}

	return r.fromProto(pb)
}
