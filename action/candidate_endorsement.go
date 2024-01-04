package action

import (
	"math/big"

	"github.com/pkg/errors"
)

const (
	// CandidateEndorsementBaseIntrinsicGas represents the base intrinsic gas for CandidateEndorsement
	CandidateEndorsementBaseIntrinsicGas = uint64(10000)
)

// CandidateEndorsement is the action to endorse or unendorse a candidate
type CandidateEndorsement struct {
	AbstractAction

	// bucketIndex is the bucket index want to be endorsed or unendorsed
	bucketIndex uint64
	// endorse is true if the action is to endorse a candidate, false if unendorse
	endorse bool
}

// BucketIndex returns the bucket index of the action
func (act *CandidateEndorsement) BucketIndex() uint64 {
	return act.bucketIndex
}

// Endorse returns true if the action is to endorse a candidate
func (act *CandidateEndorsement) Endorse() bool {
	return act.endorse
}

// IntrinsicGas returns the intrinsic gas of a CandidateEndorsement
func (act *CandidateEndorsement) IntrinsicGas() (uint64, error) {
	return CandidateEndorsementBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateEndorsement
func (act *CandidateEndorsement) Cost() (*big.Int, error) {
	intrinsicGas, err := act.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CandidateEndorsement")
	}
	fee := big.NewInt(0).Mul(act.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
