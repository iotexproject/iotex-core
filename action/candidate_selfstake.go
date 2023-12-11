package action

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// CandidateSelfStakeBaseIntrinsicGas represents the base intrinsic gas for CandidateSelfStake
	CandidateSelfStakeBaseIntrinsicGas = uint64(10000)
)

// CandidateSelfStake is the action to update a candidate's bucket
type CandidateSelfStake struct {
	AbstractAction

	// bucketID is the bucket index want to be changed to
	bucketID uint64
}

// BucketID returns the bucket index want to be changed to
func (cr *CandidateSelfStake) BucketID() uint64 { return cr.bucketID }

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateSelfStake) IntrinsicGas() (uint64, error) {
	return CandidateSelfStakeBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateRegister
func (cr *CandidateSelfStake) Cost() (*big.Int, error) {
	intrinsicGas, err := cr.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CandidateRegister creates")
	}
	fee := big.NewInt(0).Mul(cr.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}

// NewCandidateSelfStake returns a CandidateSelfStake action
func NewCandidateSelfStake(nonce, gasLimit uint64, gasPrice *big.Int, bucketID uint64) *CandidateSelfStake {
	return &CandidateSelfStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketID: bucketID,
	}
}
