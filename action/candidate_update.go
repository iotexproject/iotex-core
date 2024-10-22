// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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
	_                      EthCompatibleAction = (*CandidateUpdate)(nil)
)

// CandidateUpdate is the action to update a candidate
type CandidateUpdate struct {
	stake_common
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
func NewCandidateUpdate(name, operatorAddrStr, rewardAddrStr string) (*CandidateUpdate, error) {
	cu := &CandidateUpdate{
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

func (act *CandidateUpdate) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_CandidateUpdate{CandidateUpdate: act.Proto()}
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

// SanityCheck validates the variables in the action
func (cu *CandidateUpdate) SanityCheck() error {
	if !IsValidCandidateName(cu.Name()) {
		return ErrInvalidCanName
	}
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (cu *CandidateUpdate) EthData() ([]byte, error) {
	if cu.operatorAddress == nil {
		return nil, ErrAddress
	}
	if cu.rewardAddress == nil {
		return nil, ErrAddress
	}
	data, err := _candidateUpdateMethod.Inputs.Pack(cu.name,
		common.BytesToAddress(cu.operatorAddress.Bytes()),
		common.BytesToAddress(cu.rewardAddress.Bytes()))
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
