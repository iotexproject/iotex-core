package action

import (
	"math"
	"math/big"

	"github.com/pkg/errors"
)

const (
	// CandidateSelfStakeBaseIntrinsicGas represents the base intrinsic gas for CandidateSelfStake
	CandidateSelfStakeBaseIntrinsicGas = uint64(10000)
)

// CandidateSelfStake is the action to update a candidate's bucket
type CandidateSelfStake struct {
	AbstractAction

	// bucketID is the bucket index want to be changed to
	// if bucketID is max uint64, it means should be changed to newly self-stake bucket
	bucketID uint64

	// newly self-stake bucket info
	amount    *big.Int
	duration  uint32
	autoStake bool
}

// BucketID returns the bucket index want to be changed to
func (cr *CandidateSelfStake) BucketID() uint64 { return cr.bucketID }

// IsUsingExistingBucket returns if the bucket is existing
func (cr *CandidateSelfStake) IsUsingExistingBucket() bool { return cr.bucketID < math.MaxUint64 }

// Amount returns the amount of the newly self-stake bucket
func (cr *CandidateSelfStake) Amount() *big.Int { return cr.amount }

// Duration returns the duration of the newly self-stake bucket
func (cr *CandidateSelfStake) Duration() uint32 { return cr.duration }

// AutoStake returns the auto-stake flag of the newly self-stake bucket
func (cr *CandidateSelfStake) AutoStake() bool { return cr.autoStake }

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
