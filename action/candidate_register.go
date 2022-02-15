// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// CandidateRegisterPayloadGas represents the CandidateRegister payload gas per uint
	CandidateRegisterPayloadGas = uint64(100)
	// CandidateRegisterBaseIntrinsicGas represents the base intrinsic gas for CandidateRegister
	CandidateRegisterBaseIntrinsicGas = uint64(10000)

	candidateRegisterInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "string",
					"name": "name",
					"type": "string"
				},
				{
					"internalType": "address",
					"name": "operatorAddress",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "rewardAddress",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "ownerAddress",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"internalType": "uint32",
					"name": "duration",
					"type": "uint32"
				},
				{
					"internalType": "bool",
					"name": "autoStake",
					"type": "bool"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "candidateRegister",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _candidateRegisterInterface is the interface of the abi encoding of stake action
	_candidateRegisterMethod abi.Method

	// ErrInvalidAmount represents that amount is 0 or negative
	ErrInvalidAmount = errors.New("invalid amount")
)

// CandidateRegister is the action to register a candidate
type CandidateRegister struct {
	AbstractAction

	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
	ownerAddress    address.Address
	amount          *big.Int
	duration        uint32
	autoStake       bool
	payload         []byte
}

func init() {
	candidateRegisterInterface, err := abi.JSON(strings.NewReader(candidateRegisterInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateRegisterMethod, ok = candidateRegisterInterface.Methods["candidateRegister"]
	if !ok {
		panic("fail to load the method")
	}
}

// NewCandidateRegister creates a CandidateRegister instance
func NewCandidateRegister(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CandidateRegister, error) {
	operatorAddr, err := address.FromString(operatorAddrStr)
	if err != nil {
		return nil, err
	}

	rewardAddress, err := address.FromString(rewardAddrStr)
	if err != nil {
		return nil, err
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return nil, errors.Wrapf(ErrInvalidAmount, "amount %s", amount)
	}

	cr := &CandidateRegister{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		name:            name,
		operatorAddress: operatorAddr,
		rewardAddress:   rewardAddress,
		amount:          amount,
		duration:        duration,
		autoStake:       autoStake,
		payload:         payload,
	}

	if len(ownerAddrStr) > 0 {
		ownerAddress, err := address.FromString(ownerAddrStr)
		if err != nil {
			return nil, err
		}
		cr.ownerAddress = ownerAddress
	}
	return cr, nil
}

// Amount returns the amount
func (cr *CandidateRegister) Amount() *big.Int { return cr.amount }

// Payload returns the payload bytes
func (cr *CandidateRegister) Payload() []byte { return cr.payload }

// Duration returns the self-stake duration
func (cr *CandidateRegister) Duration() uint32 { return cr.duration }

// AutoStake returns the if staking is auth stake
func (cr *CandidateRegister) AutoStake() bool { return cr.autoStake }

// Name returns candidate name to register
func (cr *CandidateRegister) Name() string { return cr.name }

// OperatorAddress returns candidate operatorAddress to register
func (cr *CandidateRegister) OperatorAddress() address.Address { return cr.operatorAddress }

// RewardAddress returns candidate rewardAddress to register
func (cr *CandidateRegister) RewardAddress() address.Address { return cr.rewardAddress }

// OwnerAddress returns candidate ownerAddress to register
func (cr *CandidateRegister) OwnerAddress() address.Address { return cr.ownerAddress }

// Serialize returns a raw byte stream of the CandidateRegister struct
func (cr *CandidateRegister) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cr.Proto()))
}

// Proto converts to protobuf CandidateRegister Action
func (cr *CandidateRegister) Proto() *iotextypes.CandidateRegister {
	act := iotextypes.CandidateRegister{
		Candidate: &iotextypes.CandidateBasicInfo{
			Name:            cr.name,
			OperatorAddress: cr.operatorAddress.String(),
			RewardAddress:   cr.rewardAddress.String(),
		},
		StakedDuration: cr.duration,
		AutoStake:      cr.autoStake,
	}

	if cr.amount != nil {
		act.StakedAmount = cr.amount.String()
	}

	if cr.ownerAddress != nil {
		act.OwnerAddress = cr.ownerAddress.String()
	}

	if len(cr.payload) > 0 {
		act.Payload = make([]byte, len(cr.payload))
		copy(act.Payload, cr.payload)
	}
	return &act
}

// LoadProto converts a protobuf's Action to CandidateRegister
func (cr *CandidateRegister) LoadProto(pbAct *iotextypes.CandidateRegister) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}

	cInfo := pbAct.GetCandidate()
	cr.name = cInfo.GetName()

	operatorAddr, err := address.FromString(cInfo.GetOperatorAddress())
	if err != nil {
		return err
	}
	rewardAddr, err := address.FromString(cInfo.GetRewardAddress())
	if err != nil {
		return err
	}

	cr.operatorAddress = operatorAddr
	cr.rewardAddress = rewardAddr
	cr.duration = pbAct.GetStakedDuration()
	cr.autoStake = pbAct.GetAutoStake()

	if len(pbAct.GetStakedAmount()) > 0 {
		var ok bool
		if cr.amount, ok = new(big.Int).SetString(pbAct.GetStakedAmount(), 10); !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetStakedAmount())
		}
	}

	cr.payload = nil
	if len(pbAct.GetPayload()) > 0 {
		cr.payload = make([]byte, len(pbAct.GetPayload()))
		copy(cr.payload, pbAct.GetPayload())
	}

	if len(pbAct.GetOwnerAddress()) > 0 {
		ownerAddr, err := address.FromString(pbAct.GetOwnerAddress())
		if err != nil {
			return err
		}
		cr.ownerAddress = ownerAddr
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateRegister) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cr.Payload()))
	return CalculateIntrinsicGas(CandidateRegisterBaseIntrinsicGas, CandidateRegisterPayloadGas, payloadSize)
}

// Cost returns the total cost of a CandidateRegister
func (cr *CandidateRegister) Cost() (*big.Int, error) {
	intrinsicGas, err := cr.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CandidateRegister creates")
	}
	fee := big.NewInt(0).Mul(cr.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(cr.Amount(), fee), nil
}

// SanityCheck validates the variables in the action
func (cr *CandidateRegister) SanityCheck() error {
	if cr.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}

	return cr.AbstractAction.SanityCheck()
}

// EncodeABIBinary encodes data in abi encoding
func (cr *CandidateRegister) EncodeABIBinary() ([]byte, error) {
	operatorEthAddr := common.BytesToAddress(cr.operatorAddress.Bytes())
	rewardEthAddr := common.BytesToAddress(cr.rewardAddress.Bytes())
	ownerEthAddr := common.BytesToAddress(cr.ownerAddress.Bytes())
	data, err := _candidateRegisterMethod.Inputs.Pack(
		cr.name,
		operatorEthAddr,
		rewardEthAddr,
		ownerEthAddr,
		cr.amount,
		cr.duration,
		cr.autoStake,
		cr.payload)
	if err != nil {
		return nil, err
	}
	return append(_candidateRegisterMethod.ID, data...), nil
}

// NewCandidateRegisterFromABIBinary decodes data into CandidateRegister action
func NewCandidateRegisterFromABIBinary(data []byte) (*CandidateRegister, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		err       error
		cr        CandidateRegister
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_candidateRegisterMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _candidateRegisterMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cr.name, ok = paramsMap["name"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cr.operatorAddress, err = ethAddrToNativeAddr(paramsMap["operatorAddress"]); err != nil {
		return nil, err
	}
	if cr.rewardAddress, err = ethAddrToNativeAddr(paramsMap["rewardAddress"]); err != nil {
		return nil, err
	}
	if cr.ownerAddress, err = ethAddrToNativeAddr(paramsMap["ownerAddress"]); err != nil {
		return nil, err
	}
	if cr.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if cr.duration, ok = paramsMap["duration"].(uint32); !ok {
		return nil, errDecodeFailure
	}
	if cr.autoStake, ok = paramsMap["autoStake"].(bool); !ok {
		return nil, errDecodeFailure
	}
	if cr.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &cr, nil
}

func ethAddrToNativeAddr(in interface{}) (address.Address, error) {
	ethAddr, ok := in.(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	return address.FromBytes(ethAddr.Bytes())
}
