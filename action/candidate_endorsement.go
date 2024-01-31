package action

import (
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/version"
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

// IsEndorse returns true if the action is to endorse a candidate
func (act *CandidateEndorsement) IsEndorse() bool {
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

// Proto converts CandidateEndorsement to protobuf's Action
func (act *CandidateEndorsement) Proto() *iotextypes.CandidateEndorsement {
	return &iotextypes.CandidateEndorsement{
		BucketIndex: act.bucketIndex,
		Endorse:     act.endorse,
	}
}

// LoadProto converts a protobuf's Action to CandidateEndorsement
func (act *CandidateEndorsement) LoadProto(pbAct *iotextypes.CandidateEndorsement) error {
	if pbAct == nil {
		return ErrNilProto
	}
	act.bucketIndex = pbAct.GetBucketIndex()
	act.endorse = pbAct.GetEndorse()
	return nil
}

// NewCandidateEndorsement returns a CandidateEndorsement action
func NewCandidateEndorsement(nonce, gasLimit uint64, gasPrice *big.Int, bucketIndex uint64, endorse bool) *CandidateEndorsement {
	return &CandidateEndorsement{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: bucketIndex,
		endorse:     endorse,
	}
}
