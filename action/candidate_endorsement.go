package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// CandidateEndorsementBaseIntrinsicGas represents the base intrinsic gas for CandidateEndorsement
	CandidateEndorsementBaseIntrinsicGas = uint64(10000)

	candidateEndorsementInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "bool",
					"name": "endorse",
					"type": "bool"
				}
			],
			"name": "candidateEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	candidateEndorsementMethod abi.Method
)

// CandidateEndorsement is the action to endorse or unendorse a candidate
type CandidateEndorsement struct {
	AbstractAction

	// bucketIndex is the bucket index want to be endorsed or unendorsed
	bucketIndex uint64
	// endorse is true if the action is to endorse a candidate, false if unendorse
	endorse bool
}

func init() {
	candidateEndorsementInterface, err := abi.JSON(strings.NewReader(candidateEndorsementInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	candidateEndorsementMethod, ok = candidateEndorsementInterface.Methods["candidateEndorsement"]
	if !ok {
		panic("fail to load the candidateEndorsement method")
	}
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

func (act *CandidateEndorsement) encodeABIBinary() ([]byte, error) {
	data, err := candidateEndorsementMethod.Inputs.Pack(act.bucketIndex, act.endorse)
	if err != nil {
		return nil, err
	}
	return append(candidateEndorsementMethod.ID, data...), nil
}

// ToEthTx returns an Ethereum transaction which corresponds to this action
func (act *CandidateEndorsement) ToEthTx(_ uint32) (*types.Transaction, error) {
	data, err := act.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTx(&types.LegacyTx{
		Nonce:    act.Nonce(),
		GasPrice: act.GasPrice(),
		Gas:      act.GasLimit(),
		To:       &_stakingProtocolEthAddr,
		Value:    big.NewInt(0),
		Data:     data,
	}), nil
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

// NewCandidateEndorsementFromABIBinary parses the smart contract input and creates an action
func NewCandidateEndorsementFromABIBinary(data []byte) (*CandidateEndorsement, error) {
	var (
		paramsMap = map[string]any{}
		cr        CandidateEndorsement
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(candidateEndorsementMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := candidateEndorsementMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	bucketID, ok := paramsMap["bucketIndex"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	endorse, ok := paramsMap["endorse"].(bool)
	if !ok {
		return nil, errDecodeFailure
	}
	cr.bucketIndex = bucketID
	cr.endorse = endorse
	return &cr, nil
}
