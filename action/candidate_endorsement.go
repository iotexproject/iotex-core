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
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "candidateEndorse",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "candidateIntentToRevokeEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "candidateRevokeEndorsement",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

// CandidateEndorsementOp defines the operation of CandidateEndorsement
const (
	OpLegacy CandidateEndorsementOp = iota
	OpEndorse
	OpIntentToRevokeEndorsement
	OpRevokeEndorsement
)

var (
	candidateEndorsementMethod               abi.Method
	candidateEndorseMethod                   abi.Method
	candidateIntentToRevokeEndorsementMethod abi.Method
	candidateRevokeEndorsementMethod         abi.Method
)

type (
	// CandidateEndorsement is the action to endorse or unendorse a candidate
	CandidateEndorsement struct {
		AbstractAction

		// bucketIndex is the bucket index want to be endorsed or unendorsed
		bucketIndex uint64
		// endorse is true if the action is to endorse a candidate, false if unendorse
		endorse bool
		// op is the operation of the endorsement
		op CandidateEndorsementOp
	}

	// CandidateEndorsementOp defines the operation of CandidateEndorsement
	CandidateEndorsementOp uint8
)

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
	candidateEndorseMethod, ok = candidateEndorsementInterface.Methods["candidateEndorse"]
	if !ok {
		panic("fail to load the candidateEndorse method")
	}
	candidateIntentToRevokeEndorsementMethod, ok = candidateEndorsementInterface.Methods["candidateIntentToRevokeEndorsement"]
	if !ok {
		panic("fail to load the candidateIntentToRevokeEndorsement method")
	}
	candidateRevokeEndorsementMethod, ok = candidateEndorsementInterface.Methods["candidateRevokeEndorsement"]
	if !ok {
		panic("fail to load the candidateRevokeEndorsement method")
	}
}

// BucketIndex returns the bucket index of the action
func (act *CandidateEndorsement) BucketIndex() uint64 {
	return act.bucketIndex
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

// IsEndorse returns true if the action is to endorse a candidate
func (act *CandidateEndorsement) IsEndorse() bool {
	if act.IsLegacy() {
		return act.endorse
	}
	return act.op == OpEndorse
}

// IsLegacy returns true if the action is in legacy version
func (act *CandidateEndorsement) IsLegacy() bool {
	return act.op == OpLegacy
}

// Op returns the operation of the endorsement
func (act *CandidateEndorsement) Op() CandidateEndorsementOp {
	if act.IsLegacy() {
		if act.endorse {
			return OpEndorse
		}
		return OpIntentToRevokeEndorsement
	}
	return act.op
}

// Proto converts CandidateEndorsement to protobuf's Action
func (act *CandidateEndorsement) Proto() *iotextypes.CandidateEndorsement {
	p := &iotextypes.CandidateEndorsement{
		BucketIndex: act.bucketIndex,
	}
	p.Endorse = act.endorse
	p.Op = uint32(act.op)
	return p
}

// LoadProto converts a protobuf's Action to CandidateEndorsement
func (act *CandidateEndorsement) LoadProto(pbAct *iotextypes.CandidateEndorsement) error {
	if pbAct == nil {
		return ErrNilProto
	}
	act.bucketIndex = pbAct.GetBucketIndex()
	act.op = CandidateEndorsementOp(pbAct.GetOp())
	act.endorse = pbAct.GetEndorse()
	return nil
}

func (act *CandidateEndorsement) encodeABIBinary() ([]byte, error) {
	var method abi.Method
	switch act.op {
	case OpLegacy:
		data, err := candidateEndorsementMethod.Inputs.Pack(act.bucketIndex, act.endorse)
		if err != nil {
			return nil, err
		}
		return append(candidateEndorsementMethod.ID, data...), nil
	case OpEndorse:
		method = candidateEndorseMethod
	case OpIntentToRevokeEndorsement:
		method = candidateIntentToRevokeEndorsementMethod
	case OpRevokeEndorsement:
		method = candidateRevokeEndorsementMethod
	default:
		return nil, errors.New("invalid operation")
	}
	data, err := method.Inputs.Pack(act.bucketIndex)
	if err != nil {
		return nil, err
	}
	return append(method.ID, data...), nil
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

// NewCandidateEndorsementLegacy returns a CandidateEndorsement action
func NewCandidateEndorsementLegacy(nonce, gasLimit uint64, gasPrice *big.Int, bucketIndex uint64, endorse bool) *CandidateEndorsement {
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

// NewCandidateEndorsement returns a CandidateEndorsement action
func NewCandidateEndorsement(nonce, gasLimit uint64, gasPrice *big.Int, bucketIndex uint64, op CandidateEndorsementOp) *CandidateEndorsement {
	return &CandidateEndorsement{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: bucketIndex,
		op:          op,
	}
}

// NewCandidateEndorsementFromABIBinary parses the smart contract input and creates an action
func NewCandidateEndorsementFromABIBinary(data []byte) (*CandidateEndorsement, error) {
	if len(data) <= 4 {
		return nil, errDecodeFailure
	}
	var (
		paramsMap = map[string]any{}
		cr        CandidateEndorsement
		method    abi.Method
	)
	switch {
	case bytes.Equal(candidateEndorsementMethod.ID, data[:4]):
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
		cr.op = OpLegacy
		return &cr, nil
	case bytes.Equal(candidateEndorseMethod.ID, data[:4]):
		method = candidateEndorseMethod
		cr.op = OpEndorse
	case bytes.Equal(candidateIntentToRevokeEndorsementMethod.ID, data[:4]):
		method = candidateIntentToRevokeEndorsementMethod
		cr.op = OpIntentToRevokeEndorsement
	case bytes.Equal(candidateRevokeEndorsementMethod.ID, data[:4]):
		method = candidateRevokeEndorsementMethod
		cr.op = OpRevokeEndorsement
		return &cr, nil
	default:
		return nil, errDecodeFailure
	}
	if err := method.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	bucketID, ok := paramsMap["bucketIndex"].(uint64)
	if !ok {
		return nil, errDecodeFailure
	}
	cr.bucketIndex = bucketID
	cr.endorse = cr.op == OpEndorse
	return &cr, nil
}
