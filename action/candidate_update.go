// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// CandidateUpdateBaseIntrinsicGas represents the base intrinsic gas for CandidateUpdate
	CandidateUpdateBaseIntrinsicGas = uint64(10000)

	_candidateUpdateInterfaceABI = `[
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
				}
			],
			"name": "candidateUpdate",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _candidateUpdateMethod is the interface of the abi encoding of stake action
	_candidateUpdateMethod abi.Method
)

// CandidateUpdate is the action to update a candidate
type CandidateUpdate struct {
	AbstractAction

	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
}

func init() {
	_candidateUpdateInterface, err := abi.JSON(strings.NewReader(_candidateUpdateInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_candidateUpdateMethod, ok = _candidateUpdateInterface.Methods["candidateUpdate"]
	if !ok {
		panic("fail to load the method")
	}
}

// NewCandidateUpdate creates a CandidateUpdate instance
func NewCandidateUpdate(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr string,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CandidateUpdate, error) {
	cu := &CandidateUpdate{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		name: name,
	}

	var err error
	if len(operatorAddrStr) > 0 {
		cu.operatorAddress, err = address.FromString(operatorAddrStr)
		if err != nil {
			return nil, err
		}
	}

	if len(rewardAddrStr) > 0 {
		cu.rewardAddress, err = address.FromString(rewardAddrStr)
		if err != nil {
			return nil, err
		}
	}
	return cu, nil
}

// Name returns candidate name to update
func (cu *CandidateUpdate) Name() string { return cu.name }

// OperatorAddress returns candidate operatorAddress to update
func (cu *CandidateUpdate) OperatorAddress() address.Address { return cu.operatorAddress }

// RewardAddress returns candidate rewardAddress to update
func (cu *CandidateUpdate) RewardAddress() address.Address { return cu.rewardAddress }

// Serialize returns a raw byte stream of the CandidateUpdate struct
func (cu *CandidateUpdate) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cu.Proto()))
}

// Proto converts to protobuf CandidateUpdate Action
func (cu *CandidateUpdate) Proto() *iotextypes.CandidateBasicInfo {
	act := &iotextypes.CandidateBasicInfo{
		Name: cu.name,
	}

	if cu.operatorAddress != nil {
		act.OperatorAddress = cu.operatorAddress.String()
	}

	if cu.rewardAddress != nil {
		act.RewardAddress = cu.rewardAddress.String()
	}

	return act
}

// LoadProto converts a protobuf's Action to CandidateUpdate
func (cu *CandidateUpdate) LoadProto(pbAct *iotextypes.CandidateBasicInfo) error {
	if pbAct == nil {
		return ErrNilProto
	}

	cu.name = pbAct.GetName()

	if len(pbAct.GetOperatorAddress()) > 0 {
		operatorAddr, err := address.FromString(pbAct.GetOperatorAddress())
		if err != nil {
			return err
		}
		cu.operatorAddress = operatorAddr
	}

	if len(pbAct.GetRewardAddress()) > 0 {
		rewardAddr, err := address.FromString(pbAct.GetRewardAddress())
		if err != nil {
			return err
		}
		cu.rewardAddress = rewardAddr
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateUpdate
func (cu *CandidateUpdate) IntrinsicGas() (uint64, error) {
	return CandidateUpdateBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateUpdate
func (cu *CandidateUpdate) Cost() (*big.Int, error) {
	intrinsicGas, err := cu.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the CandidateUpdate")
	}
	fee := big.NewInt(0).Mul(cu.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}

// SanityCheck validates the variables in the action
func (cu *CandidateUpdate) SanityCheck() error {
	if !IsValidCandidateName(cu.Name()) {
		return ErrInvalidCanName
	}

	return cu.AbstractAction.SanityCheck()
}

// EncodeABIBinary encodes data in abi encoding
func (cu *CandidateUpdate) EncodeABIBinary() ([]byte, error) {
	return cu.encodeABIBinary()
}

func (cu *CandidateUpdate) encodeABIBinary() ([]byte, error) {
	operatorEthAddr := common.BytesToAddress(cu.operatorAddress.Bytes())
	rewardEthAddr := common.BytesToAddress(cu.rewardAddress.Bytes())
	data, err := _candidateUpdateMethod.Inputs.Pack(cu.name, operatorEthAddr, rewardEthAddr)
	if err != nil {
		return nil, err
	}
	return append(_candidateUpdateMethod.ID, data...), nil
}

// NewCandidateUpdateFromABIBinary decodes data into CandidateUpdate action
func NewCandidateUpdateFromABIBinary(data []byte) (*CandidateUpdate, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		err       error
		cu        CandidateUpdate
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_candidateUpdateMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _candidateUpdateMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cu.name, ok = paramsMap["name"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cu.operatorAddress, err = ethAddrToNativeAddr(paramsMap["operatorAddress"]); err != nil {
		return nil, err
	}
	if cu.rewardAddress, err = ethAddrToNativeAddr(paramsMap["rewardAddress"]); err != nil {
		return nil, err
	}
	return &cu, nil
}

// ToEthTx converts action to eth-compatible tx
func (cu *CandidateUpdate) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.StakingProtocolAddr)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := cu.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(cu.Nonce(), ethAddr, big.NewInt(0), cu.GasLimit(), cu.GasPrice(), data), nil
}
