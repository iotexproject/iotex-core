package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// CandidateActivateBaseIntrinsicGas represents the base intrinsic gas for CandidateActivate
	CandidateActivateBaseIntrinsicGas = uint64(10000)

	// TODO: move all parts of staking abi to a unified file
	candidateActivateInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "candidateActivate",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	candidateActivateMethod abi.Method
	_                       EthCompatibleAction = (*CandidateActivate)(nil)
)

// CandidateActivate is the action to update a candidate's bucket
type CandidateActivate struct {
	stake_common
	// bucketID is the bucket index want to be changed to
	bucketID uint64
}

func init() {
	candidateActivateInterface, err := abi.JSON(strings.NewReader(candidateActivateInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	candidateActivateMethod, ok = candidateActivateInterface.Methods["candidateActivate"]
	if !ok {
		panic("fail to load the candidateActivate method")
	}
}

// NewCandidateActivate returns a CandidateActivate action
func NewCandidateActivate(bucketID uint64) *CandidateActivate {
	return &CandidateActivate{
		bucketID: bucketID,
	}
}

// BucketID returns the bucket index want to be changed to
func (cr *CandidateActivate) BucketID() uint64 { return cr.bucketID }

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateActivate) IntrinsicGas() (uint64, error) {
	return CandidateActivateBaseIntrinsicGas, nil
}

func (cr *CandidateActivate) SanityCheck() error {
	return nil
}

func (cr *CandidateActivate) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_CandidateActivate{CandidateActivate: cr.Proto()}
}

// Proto converts CandidateActivate to protobuf's Action
func (cr *CandidateActivate) Proto() *iotextypes.CandidateActivate {
	return &iotextypes.CandidateActivate{
		BucketIndex: cr.bucketID,
	}
}

// LoadProto converts a protobuf's Action to CandidateActivate
func (cr *CandidateActivate) LoadProto(pbAct *iotextypes.CandidateActivate) error {
	if pbAct == nil {
		return ErrNilProto
	}
	cr.bucketID = pbAct.GetBucketIndex()
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (cr *CandidateActivate) EthData() ([]byte, error) {
	data, err := candidateActivateMethod.Inputs.Pack(cr.bucketID)
	if err != nil {
		return nil, err
	}
	return append(candidateActivateMethod.ID, data...), nil
}

// NewCandidateActivateFromABIBinary parses the smart contract input and creates an action
func NewCandidateActivateFromABIBinary(data []byte) (*CandidateActivate, error) {
	var (
		paramsMap = map[string]any{}
		cr        CandidateActivate
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(candidateActivateMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := candidateActivateMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	bucketID, ok := paramsMap["bucketIndex"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	cr.bucketID = bucketID
	return &cr, nil
}
