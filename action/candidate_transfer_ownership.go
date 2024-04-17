package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

const (
	// CandidateTransferOwnershipBaseIntrinsicGas represents the base intrinsic gas for CandidateTransferOwnership
	CandidateTransferOwnershipBaseIntrinsicGas = uint64(10000)

	_candidateTransferOwnershipInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "string",
					"name": "name",
					"type": "string"
				},
				{
					"internalType": "address",
					"name": "newOwner",
					"type": "address"
				},
				{
					"internalType": "uint8[]",
					"name": "payload",
					"type": "uint8[]"
				}
			],
			"name": "candidateTransferOwnership",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _candidateTransferOwnershipMethod is the interface of the abi encoding of candidate transfer ownership action
	_candidateTransferOwnershipMethod abi.Method
)

func init() {
	candidateTransferOwnershipInterface, err := abi.JSON(strings.NewReader(_candidateTransferOwnershipInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateTransferOwnershipMethod, ok = candidateTransferOwnershipInterface.Methods["candidateTransferOwnership"]
	if !ok {
		panic("fail to load the method")
	}
}

// CandidateTransferOwnership is the action to transfer ownership of a candidate
type CandidateTransferOwnership struct {
	AbstractAction

	name     string
	newOwner address.Address
	payload  []byte
}

// NewCandidateTransferOwnership returns a CandidateTransferOwnership action
func NewCandidateTransferOwnership(nonce, gasLimit uint64, gasPrice *big.Int, name string,
	newOwnerStr string, payload []byte) (*CandidateTransferOwnership, error) {
	newOwner, err := address.FromString(newOwnerStr)
	if err != nil {
		return nil, err
	}
	return &CandidateTransferOwnership{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		name:     name,
		newOwner: newOwner,
		payload:  payload,
	}, nil
}

// NewCandidateTransferOwnershipFromAction decode data to CandidateTransferOwnership
func NewCandidateTransferOwnershipFromABIBinary(data []byte) (*CandidateTransferOwnership, error) {
	var (
		paramsMap = map[string]any{}
		ok        bool
		err       error
		cr        CandidateTransferOwnership
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_candidateTransferOwnershipMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err = _candidateTransferOwnershipMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cr.name, ok = paramsMap["name"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cr.newOwner, err = ethAddrToNativeAddr(paramsMap["newOwner"]); err != nil {
		return nil, err
	}
	if cr.payload, ok = paramsMap["payload"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &cr, nil
}

// Payload returns the payload bytes
func (act *CandidateTransferOwnership) Payload() []byte { return act.payload }

// Name returns candidate name to transfer ownership
func (act *CandidateTransferOwnership) Name() string { return act.name }

// NewOwner returns the new owner address
func (act *CandidateTransferOwnership) NewOwner() address.Address { return act.newOwner }

// IntrinsicGas returns the intrinsic gas of a CandidateTransferOwnership
func (act *CandidateTransferOwnership) IntrinsicGas() (uint64, error) {
	return CandidateTransferOwnershipBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateTransferOwnership
func (act *CandidateTransferOwnership) Cost() (*big.Int, error) {
	intrinsicGas, _ := act.IntrinsicGas()

	fee := big.NewInt(0).Mul(act.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}

// Serialize returns a raw byte stream of the CandidateTransferOwnership struct
func (act *CandidateTransferOwnership) Serialize() []byte {
	return byteutil.Must(proto.Marshal(act.Proto()))
}

// Proto converts to protobuf CandidateTransferOwnership Action
func (act *CandidateTransferOwnership) Proto() *iotextypes.CandidateTransferOwnership {
	ac := iotextypes.CandidateTransferOwnership{
		CandidateName:   act.Name(),
		NewOwnerAddress: act.newOwner.String(),
	}

	if len(act.payload) > 0 {
		ac.Payload = make([]byte, len(act.payload))
		copy(ac.Payload, act.payload)
	}
	return &ac
}

// LoadProto loads the CandidateTransferOwnership Action from protobuf
func (act *CandidateTransferOwnership) LoadProto(pbAct *iotextypes.CandidateTransferOwnership) error {
	if pbAct == nil {
		return ErrNilProto
	}
	act.name = pbAct.GetCandidateName()
	newOwner, err := address.FromString(pbAct.GetNewOwnerAddress())
	if err != nil {
		return err
	}
	act.newOwner = newOwner
	act.payload = nil
	if len(pbAct.GetPayload()) > 0 {
		act.payload = make([]byte, len(pbAct.GetPayload()))
		copy(act.payload, pbAct.GetPayload())
	}
	return nil
}

// EncodeABIBinary encodes data in abi encoding
func (act *CandidateTransferOwnership) EncodeABIBinary() ([]byte, error) {
	return act.encodeABIBinary()
}

func (act *CandidateTransferOwnership) encodeABIBinary() ([]byte, error) {
	if act.newOwner == nil {
		return nil, ErrAddress
	}
	data, err := _candidateTransferOwnershipMethod.Inputs.Pack(act.name, common.BytesToAddress(act.newOwner.Bytes()), act.payload)
	if err != nil {
		return nil, err
	}
	return append(_candidateTransferOwnershipMethod.ID, data...), nil
}

// SanityCheck validates the variables in the action
func (act *CandidateTransferOwnership) SanityCheck() error {
	if !IsValidCandidateName(act.Name()) {
		return ErrInvalidCanName
	}

	return act.AbstractAction.SanityCheck()
}

// ToEthTx returns an Ethereum transaction which corresponds to this action
func (act *CandidateTransferOwnership) ToEthTx(_ uint32) (*types.Transaction, error) {
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
